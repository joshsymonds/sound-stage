package fakeusdx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

// --- POST /pause ---

func post(t *testing.T, fake *fakeusdx.Fake, path string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	return rec, rec.Body.Bytes()
}

func TestPause_NotPlaying(t *testing.T) {
	fake := fakeusdx.New()

	rec, body := post(t, fake, "/pause")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertErrorBody(t, body, "not playing")
}

func TestPause_WhilePlaying(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	rec, body := post(t, fake, "/pause")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["status"] != "paused" {
		t.Errorf(`status = %q, want "paused"`, got["status"])
	}

	// Second pause → 409
	rec2, body2 := post(t, fake, "/pause")
	if rec2.Code != http.StatusConflict {
		t.Errorf("second pause status = %d, want 409", rec2.Code)
	}
	assertErrorBody(t, body2, "not playing")
}

func TestPause_GetReturns405(t *testing.T) {
	fake := fakeusdx.New()
	req := httptest.NewRequest(http.MethodGet, "/pause", nil)
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// --- POST /resume ---

func TestResume_NothingToResume(t *testing.T) {
	fake := fakeusdx.New()

	rec, body := post(t, fake, "/resume")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertErrorBody(t, body, "nothing to resume")
}

func TestResume_AfterPause(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}
	if rec, _ := post(t, fake, "/pause"); rec.Code != http.StatusOK {
		t.Fatalf("pause: %d", rec.Code)
	}

	rec, body := post(t, fake, "/resume")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]string
	_ = json.Unmarshal(body, &got)
	if got["status"] != "resumed" {
		t.Errorf(`status = %q, want "resumed"`, got["status"])
	}

	// Second resume → 409
	rec2, body2 := post(t, fake, "/resume")
	if rec2.Code != http.StatusConflict {
		t.Errorf("second resume status = %d, want 409", rec2.Code)
	}
	assertErrorBody(t, body2, "nothing to resume")
}

func TestResume_GetReturns405(t *testing.T) {
	fake := fakeusdx.New()
	req := httptest.NewRequest(http.MethodGet, "/resume", nil)
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// --- Interactions ---

func TestPauseInteraction_SetIdleClearsPaused(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}
	if rec, _ := post(t, fake, "/pause"); rec.Code != http.StatusOK {
		t.Fatalf("pause: %d", rec.Code)
	}

	fake.SetIdle()

	// After SetIdle, /resume has nothing to resume.
	rec, body := post(t, fake, "/resume")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertErrorBody(t, body, "nothing to resume")
}

func TestPauseInteraction_SetCurrentPlayingDoesNotAutoPause(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	// /resume must 409 — not paused.
	rec, body := post(t, fake, "/resume")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertErrorBody(t, body, "nothing to resume")
}

// --- GET /debug/state ---

type debugState struct {
	Screen                string        `json:"screen"`
	IniPlayers            int           `json:"iniPlayers"`
	PlayersPlay           int           `json:"playersPlay"`
	AudioFinished         bool          `json:"audioFinished"`
	IniName               []string      `json:"iniName"`
	Player                []debugPlayer `json:"player"`
	ScreenSingPlayerNames []string      `json:"screenSingPlayerNames"`
	CurrentSong           *debugCurrent `json:"currentSong"`
	QueuedSong            *debugQueued  `json:"queuedSong"`
}

type debugPlayer struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

type debugCurrent struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

type debugQueued struct {
	SongID    string `json:"songId"`
	Requester string `json:"requester"`
	Is2P      bool   `json:"is2P"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
}

func getDebugState(t *testing.T, fake *fakeusdx.Fake) debugState {
	t.Helper()
	rec, body := doRequest(t, fake, http.MethodGet, "/debug/state")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}
	var got debugState
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, body)
	}
	return got
}

func TestDebugState_FreshFake(t *testing.T) {
	fake := fakeusdx.New()
	got := getDebugState(t, fake)

	if got.Screen != "ScreenMain" {
		t.Errorf("screen = %q, want ScreenMain", got.Screen)
	}
	if got.IniPlayers != 0 {
		t.Errorf("iniPlayers = %d, want 0", got.IniPlayers)
	}
	if got.PlayersPlay != 1 {
		t.Errorf("playersPlay = %d, want 1", got.PlayersPlay)
	}
	if got.AudioFinished {
		t.Errorf("audioFinished = true, want false")
	}
	if got.CurrentSong != nil {
		t.Errorf("currentSong = %+v, want nil", got.CurrentSong)
	}
	if got.QueuedSong != nil {
		t.Errorf("queuedSong = %+v, want nil", got.QueuedSong)
	}
	if len(got.IniName) != 6 {
		t.Errorf("iniName length = %d, want 6", len(got.IniName))
	}
	if len(got.ScreenSingPlayerNames) != 6 {
		t.Errorf("screenSingPlayerNames length = %d, want 6", len(got.ScreenSingPlayerNames))
	}
	if len(got.Player) != 1 {
		t.Errorf("player length = %d, want 1 (playersPlay)", len(got.Player))
	}
}

func TestDebugState_After2PQueue(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)

	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Charlie",
		"players":   2,
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}

	got := getDebugState(t, fake)
	if got.IniPlayers != 1 {
		t.Errorf("iniPlayers = %d, want 1 (index for 2 players)", got.IniPlayers)
	}
	if got.PlayersPlay != 2 {
		t.Errorf("playersPlay = %d, want 2", got.PlayersPlay)
	}
	if got.QueuedSong == nil {
		t.Fatal("queuedSong = nil, want populated")
	}
	if got.QueuedSong.SongID != id {
		t.Errorf("queuedSong.songId = %q, want %q", got.QueuedSong.SongID, id)
	}
	if got.QueuedSong.Requester != "Charlie" {
		t.Errorf("queuedSong.requester = %q", got.QueuedSong.Requester)
	}
	if !got.QueuedSong.Is2P {
		t.Errorf("queuedSong.is2P = false, want true")
	}
	if got.QueuedSong.Title != "Dancing Queen" {
		t.Errorf("queuedSong.title = %q, want resolved from library", got.QueuedSong.Title)
	}
	if got.QueuedSong.Artist != "ABBA" {
		t.Errorf("queuedSong.artist = %q, want resolved from library", got.QueuedSong.Artist)
	}
	if len(got.Player) != 2 {
		t.Errorf("player length = %d, want 2", len(got.Player))
	}
}

func TestDebugState_CurrentSongPopulated(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "Take On Me", Artist: "a-ha", Duet: false}})
	id := stableid.Compute("a-ha", "Take On Me", false)
	if err := fake.SetCurrentPlaying(id, 42.0, 243.0); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	got := getDebugState(t, fake)
	if got.Screen != "ScreenSing" {
		t.Errorf("screen = %q, want ScreenSing", got.Screen)
	}
	if got.CurrentSong == nil {
		t.Fatal("currentSong = nil, want populated")
	}
	if got.CurrentSong.ID != id {
		t.Errorf("currentSong.id = %q, want %q", got.CurrentSong.ID, id)
	}
	if got.CurrentSong.Title != "Take On Me" {
		t.Errorf("currentSong.title = %q", got.CurrentSong.Title)
	}
}

func TestDebugState_AudioFinished(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}
	// After the audio finishes but before the user leaves ScreenSing:
	fake.SetIdle()

	got := getDebugState(t, fake)
	if got.Screen != "ScreenSing" {
		t.Errorf("screen = %q, want ScreenSing (SetIdle doesn't change screen)", got.Screen)
	}
	if !got.AudioFinished {
		t.Errorf("audioFinished = false, want true (ScreenSing + playing=nil)")
	}
	if got.CurrentSong != nil {
		t.Errorf("currentSong = %+v, want nil", got.CurrentSong)
	}
}

func TestDebugState_PostReturns405(t *testing.T) {
	fake := fakeusdx.New()
	req := httptest.NewRequest(http.MethodPost, "/debug/state", nil)
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestDebugState_RequesterAsFirstPlayer(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
		"players":   1,
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}

	got := getDebugState(t, fake)
	if len(got.Player) < 1 {
		t.Fatalf("player slice empty")
	}
	if got.Player[0].Name != "Alice" {
		t.Errorf("player[0].name = %q, want Alice (requester becomes P1 name)", got.Player[0].Name)
	}
	if got.ScreenSingPlayerNames[0] != "Alice" {
		t.Errorf("screenSingPlayerNames[0] = %q, want Alice", got.ScreenSingPlayerNames[0])
	}
}
