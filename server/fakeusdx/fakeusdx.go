// Package fakeusdx is a faithful in-memory stand-in for the USDX HTTP API
// described in ~/Personal/sound-stage-usdx/docs/API.md. It lets SoundStage
// do end-to-end development without a running Steam Deck.
package fakeusdx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/joshsymonds/sound-stage/server/stableid"
)

// Screen identifies which USDX screen the fake is currently modeling.
// Real USDX has more screens; these are the ones the API contract cares about.
type Screen string

// The subset of USDX screens the API contract distinguishes between.
const (
	ScreenMain   Screen = "ScreenMain"
	ScreenSing   Screen = "ScreenSing"
	ScreenScore  Screen = "ScreenScore"
	ScreenNextUp Screen = "ScreenNextUp"
)

// Song is a song loaded into the fake's library. ID is computed from the
// metadata via stableid.Compute at LoadSongs time.
type Song struct {
	Title  string
	Artist string
	Duet   bool
}

// QueuedSlot is the staged "next up" song plus who asked for it. Lives for
// one song consumption: pulled by the user pressing Sing (ScreenMain) or
// Enter (ScreenNextUp), discarded otherwise.
type QueuedSlot struct {
	SongID    string
	Requester string
	Players   int // 1 or 2
}

type songEntry struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Duet   bool   `json:"duet"`
	// Path is the absolute filesystem path the song was loaded from via
	// POST /refresh. Empty for songs loaded via LoadSongs. Not serialized.
	Path string `json:"-"`
}

type nowPlayingResponse struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Elapsed  float64 `json:"elapsed"`
	Duration float64 `json:"duration"`
}

type playingState struct {
	entry    songEntry
	elapsed  float64
	duration float64
}

// Fake is an in-memory USDX stand-in. Zero value is not usable; construct via New.
type Fake struct {
	mu              sync.Mutex
	songs           []songEntry
	playing         *playingState
	paused          bool
	screen          Screen
	slot            *QueuedSlot
	sessionPlayers  int
	defaultDuration float64
	clockInterval   time.Duration
	clockStop       chan struct{}
	clockDone       chan struct{}
}

// ErrUnknownSongID is returned when a song ID is not present in the loaded library.
var ErrUnknownSongID = errors.New("unknown song id")

// New creates an empty Fake. Initial state: ScreenMain, empty library, no slot, no session.
func New() *Fake {
	return &Fake{songs: []songEntry{}, screen: ScreenMain}
}

// LoadSongs replaces the library with the given songs. Insertion order is
// preserved in GET /songs responses. IDs are computed via stableid.Compute.
// Clears playing state if the current song is no longer loaded.
func (f *Fake) LoadSongs(songs []Song) {
	entries := make([]songEntry, len(songs))
	for i, s := range songs {
		entries[i] = songEntry{
			ID:     stableid.Compute(s.Artist, s.Title, s.Duet),
			Title:  s.Title,
			Artist: s.Artist,
			Duet:   s.Duet,
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.songs = entries
	if f.playing != nil {
		found := false
		for _, e := range entries {
			if e.ID == f.playing.entry.ID {
				found = true
				break
			}
		}
		if !found {
			f.playing = nil
		}
	}
}

// SetCurrentPlaying marks the given song as currently playing on ScreenSing
// with the given elapsed/duration (seconds). Also transitions screen to
// ScreenSing (real USDX can only play audio from ScreenSing). Returns
// ErrUnknownSongID if the ID is not loaded; state is unchanged on error.
func (f *Fake) SetCurrentPlaying(songID string, elapsed, duration float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, e := range f.songs {
		if e.ID == songID {
			f.playing = &playingState{entry: e, elapsed: elapsed, duration: duration}
			f.screen = ScreenSing
			return nil
		}
	}
	return ErrUnknownSongID
}

// SetIdle clears the playing pointer and the paused flag. Screen is left
// alone — use SetScreen if the caller wants to also transition away from
// ScreenSing.
func (f *Fake) SetIdle() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.playing = nil
	f.paused = false
}

// SetScreen forces the current screen. Transitioning to ScreenMain ends any
// active session (resets sessionPlayers to 0). Returns an error for unknown
// Screen values; state is unchanged on error.
func (f *Fake) SetScreen(screen Screen) error {
	switch screen {
	case ScreenMain, ScreenSing, ScreenScore, ScreenNextUp:
	default:
		return fmt.Errorf("unknown screen: %q", screen)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.screen = screen
	if screen == ScreenMain {
		f.sessionPlayers = 0
	}
	return nil
}

// Screen returns the current screen.
func (f *Fake) Screen() Screen {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.screen
}

// Slot returns a copy of the current queued-song slot, or nil if empty.
func (f *Fake) Slot() *QueuedSlot {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.slot == nil {
		return nil
	}
	copied := *f.slot
	return &copied
}

// SessionPlayers returns the locked-in player count for the current session,
// or 0 when no session is active.
func (f *Fake) SessionPlayers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sessionPlayers
}

// ServeHTTP routes requests to the implemented endpoints.
func (f *Fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/_test/") {
		f.routeTestHook(w, r)
		return
	}

	handler, method, ok := f.routeMain(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != method {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	handler(w, r)
}

// routeMain returns the handler for a public endpoint path, its allowed
// method, and ok=false for unknown paths.
func (f *Fake) routeMain(path string) (handler func(http.ResponseWriter, *http.Request), method string, ok bool) {
	switch path {
	case "/songs":
		return func(w http.ResponseWriter, _ *http.Request) { f.handleSongs(w) }, http.MethodGet, true
	case "/now-playing":
		return func(w http.ResponseWriter, _ *http.Request) { f.handleNowPlaying(w) }, http.MethodGet, true
	case "/queue":
		return f.handleQueue, http.MethodPost, true
	case "/refresh":
		return f.handleRefresh, http.MethodPost, true
	case "/pause":
		return func(w http.ResponseWriter, _ *http.Request) { f.handlePause(w) }, http.MethodPost, true
	case "/resume":
		return func(w http.ResponseWriter, _ *http.Request) { f.handleResume(w) }, http.MethodPost, true
	case "/debug/state":
		return func(w http.ResponseWriter, _ *http.Request) { f.handleDebugState(w) }, http.MethodGet, true
	}
	return nil, "", false
}

func (f *Fake) handleSongs(w http.ResponseWriter) {
	f.mu.Lock()
	entries := make([]songEntry, len(f.songs))
	copy(entries, f.songs)
	f.mu.Unlock()

	writeJSON(w, http.StatusOK, entries)
}

func (f *Fake) handleNowPlaying(w http.ResponseWriter) {
	f.mu.Lock()
	var body any
	if f.screen == ScreenSing && f.playing != nil {
		body = nowPlayingResponse{
			ID:       f.playing.entry.ID,
			Title:    f.playing.entry.Title,
			Artist:   f.playing.entry.Artist,
			Elapsed:  f.playing.elapsed,
			Duration: f.playing.duration,
		}
	}
	f.mu.Unlock()

	writeJSON(w, http.StatusOK, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
