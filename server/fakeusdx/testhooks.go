package fakeusdx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultClockInterval = time.Second

// routeTestHook dispatches /_test/* paths. Called by ServeHTTP when the path
// matches the /_test/ prefix.
func (f *Fake) routeTestHook(w http.ResponseWriter, r *http.Request) {
	subPath := strings.TrimPrefix(r.URL.Path, "/_test/")
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	switch subPath {
	case "set-screen":
		f.hookSetScreen(w, r)
	case "advance-elapsed":
		f.hookAdvanceElapsed(w, r)
	case "press-sing":
		f.hookPressSing(w, r)
	case "press-enter":
		f.hookPressEnter(w, r)
	case "press-esc":
		f.hookPressEsc(w, r)
	case "load-songs":
		f.hookLoadSongs(w, r)
	case "set-current-playing":
		f.hookSetCurrentPlaying(w, r)
	case "set-idle":
		f.hookSetIdle(w, r)
	case "set-default-duration":
		f.hookSetDefaultDuration(w, r)
	case "start-clock":
		f.hookStartClock(w, r)
	case "stop-clock":
		f.hookStopClock(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

// readTestBody reads a JSON body with the shared 64 KB cap. Returns the raw
// bytes or writes the appropriate error and returns nil.
func readTestBody(w http.ResponseWriter, r *http.Request) []byte {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxQueueBodySize))
	if err != nil {
		var mbErr *http.MaxBytesError
		if errors.As(err, &mbErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "body too large")
			return nil
		}
		writeError(w, http.StatusBadRequest, "malformed json")
		return nil
	}
	return body
}

// parseTestBodyObject parses the body as a JSON object. Empty body is treated
// as an empty object (useful for no-arg endpoints).
func parseTestBodyObject(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	body := readTestBody(w, r)
	if body == nil {
		return nil, false
	}
	if len(body) == 0 {
		return map[string]any{}, true
	}
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "malformed json")
		return nil, false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		writeError(w, http.StatusBadRequest, "body must be an object")
		return nil, false
	}
	return obj, true
}

// mapPressErr maps Press* / slot errors to appropriate HTTP status codes.
func mapPressErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrWrongScreen):
		writeError(w, http.StatusConflict, "wrong screen")
	case errors.Is(err, ErrNoSlot):
		writeError(w, http.StatusConflict, "no slot")
	case errors.Is(err, ErrUnknownSongID):
		writeError(w, http.StatusNotFound, "unknown songId")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

func (f *Fake) hookSetScreen(w http.ResponseWriter, r *http.Request) {
	obj, ok := parseTestBodyObject(w, r)
	if !ok {
		return
	}
	screen, isStr := obj["screen"].(string)
	if !isStr || screen == "" {
		writeError(w, http.StatusBadRequest, "screen required (string)")
		return
	}
	if err := f.SetScreen(Screen(screen)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"screen": screen})
}

func (f *Fake) hookAdvanceElapsed(w http.ResponseWriter, r *http.Request) {
	obj, ok := parseTestBodyObject(w, r)
	if !ok {
		return
	}
	seconds, isNum := obj["seconds"].(float64)
	if !isNum {
		writeError(w, http.StatusBadRequest, "seconds required (number)")
		return
	}
	elapsed := f.AdvanceElapsed(seconds)
	writeJSON(w, http.StatusOK, map[string]float64{"elapsed": elapsed})
}

func (f *Fake) hookPressSing(w http.ResponseWriter, r *http.Request) {
	if _, ok := parseTestBodyObject(w, r); !ok {
		return
	}
	if err := f.PressSing(); err != nil {
		mapPressErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"screen": string(f.Screen())})
}

func (f *Fake) hookPressEnter(w http.ResponseWriter, r *http.Request) {
	if _, ok := parseTestBodyObject(w, r); !ok {
		return
	}
	if err := f.PressEnter(); err != nil {
		mapPressErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"screen": string(f.Screen())})
}

func (f *Fake) hookPressEsc(w http.ResponseWriter, r *http.Request) {
	if _, ok := parseTestBodyObject(w, r); !ok {
		return
	}
	if err := f.PressEsc(); err != nil {
		mapPressErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"screen": string(f.Screen())})
}

func (f *Fake) hookLoadSongs(w http.ResponseWriter, r *http.Request) {
	body := readTestBody(w, r)
	if body == nil {
		return
	}
	var arr []struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
		Duet   bool   `json:"duet"`
	}
	if err := json.Unmarshal(body, &arr); err != nil {
		writeError(w, http.StatusBadRequest, "body must be an array")
		return
	}
	songs := make([]Song, len(arr))
	for i, s := range arr {
		songs[i] = Song{Title: s.Title, Artist: s.Artist, Duet: s.Duet}
	}
	f.LoadSongs(songs)
	writeJSON(w, http.StatusOK, map[string]int{"count": len(songs)})
}

func (f *Fake) hookSetCurrentPlaying(w http.ResponseWriter, r *http.Request) {
	obj, ok := parseTestBodyObject(w, r)
	if !ok {
		return
	}
	songID, isStr := obj["songId"].(string)
	if !isStr || songID == "" {
		writeError(w, http.StatusBadRequest, "songId required (string)")
		return
	}
	var elapsed, duration float64
	if v, isFloat := obj["elapsed"].(float64); isFloat {
		elapsed = v
	}
	if v, isFloat := obj["duration"].(float64); isFloat {
		duration = v
	}
	if err := f.SetCurrentPlaying(songID, elapsed, duration); err != nil {
		writeError(w, http.StatusNotFound, "unknown songId")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (f *Fake) hookSetIdle(w http.ResponseWriter, r *http.Request) {
	if _, ok := parseTestBodyObject(w, r); !ok {
		return
	}
	f.SetIdle()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (f *Fake) hookSetDefaultDuration(w http.ResponseWriter, r *http.Request) {
	obj, ok := parseTestBodyObject(w, r)
	if !ok {
		return
	}
	seconds, isNum := obj["seconds"].(float64)
	if !isNum {
		writeError(w, http.StatusBadRequest, "seconds required (number)")
		return
	}
	f.SetDefaultDuration(seconds)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (f *Fake) hookStartClock(w http.ResponseWriter, r *http.Request) {
	if _, ok := parseTestBodyObject(w, r); !ok {
		return
	}
	f.StartClock()
	writeJSON(w, http.StatusOK, map[string]bool{"running": true})
}

func (f *Fake) hookStopClock(w http.ResponseWriter, r *http.Request) {
	if _, ok := parseTestBodyObject(w, r); !ok {
		return
	}
	f.StopClock()
	writeJSON(w, http.StatusOK, map[string]bool{"running": false})
}

// --- Auto-clock ---

// SetClockInterval sets the cadence at which the auto-clock advances elapsed
// when running. Defaults to 1s. Tests may lower this to keep suites fast.
func (f *Fake) SetClockInterval(interval time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clockInterval = interval
}

// StartClock begins a goroutine that advances elapsed by `interval` every
// `interval` of real time. Idempotent: a second call while running is a no-op.
func (f *Fake) StartClock() {
	f.mu.Lock()
	if f.clockStop != nil {
		f.mu.Unlock()
		return
	}
	interval := f.clockInterval
	if interval <= 0 {
		interval = defaultClockInterval
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	f.clockStop = stop
	f.clockDone = done
	f.mu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				f.AdvanceElapsed(interval.Seconds())
			}
		}
	}()
}

// StopClock signals the clock goroutine to exit and waits for it to finish.
// Idempotent: a call while not running is a no-op.
func (f *Fake) StopClock() {
	f.mu.Lock()
	stop := f.clockStop
	done := f.clockDone
	f.clockStop = nil
	f.clockDone = nil
	f.mu.Unlock()

	if stop == nil {
		return
	}
	close(stop)
	<-done
}
