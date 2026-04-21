package fakeusdx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

func testHookPost(t *testing.T, fake *fakeusdx.Fake, path string, body any) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(http.MethodPost, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	return rec, rec.Body.Bytes()
}

// --- Routing ---

func TestTestHooks_GetReturns405(t *testing.T) {
	fake := fakeusdx.New()
	req := httptest.NewRequest(http.MethodGet, "/_test/press-sing", nil)
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestTestHooks_UnknownPathReturns404(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := testHookPost(t, fake, "/_test/nonsense", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	assertErrorBody(t, body, "not found")
}

// --- /_test/set-screen ---

func TestSetScreenHook_Valid(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := testHookPost(t, fake, "/_test/set-screen", map[string]any{"screen": "ScreenScore"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}
	var got map[string]string
	_ = json.Unmarshal(body, &got)
	if got["screen"] != "ScreenScore" {
		t.Errorf("response screen = %q, want ScreenScore", got["screen"])
	}
	if fake.Screen() != fakeusdx.ScreenScore {
		t.Errorf("fake screen = %q, want ScreenScore", fake.Screen())
	}
}

func TestSetScreenHook_Invalid(t *testing.T) {
	fake := fakeusdx.New()
	rec, _ := testHookPost(t, fake, "/_test/set-screen", map[string]any{"screen": "bogus"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestSetScreenHook_Missing(t *testing.T) {
	fake := fakeusdx.New()
	rec, _ := testHookPost(t, fake, "/_test/set-screen", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestSetScreenHook_Malformed(t *testing.T) {
	fake := fakeusdx.New()
	req := httptest.NewRequest(http.MethodPost, "/_test/set-screen", bytes.NewReader([]byte("garbage")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- /_test/advance-elapsed ---

func TestAdvanceElapsedHook_Playing(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	rec, body := testHookPost(t, fake, "/_test/advance-elapsed", map[string]any{"seconds": 5.0})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}
	var got struct {
		Elapsed float64 `json:"elapsed"`
	}
	_ = json.Unmarshal(body, &got)
	if got.Elapsed != 15 {
		t.Errorf("elapsed = %v, want 15", got.Elapsed)
	}
}

func TestAdvanceElapsedHook_NotPlaying(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := testHookPost(t, fake, "/_test/advance-elapsed", map[string]any{"seconds": 1.0})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Elapsed float64 `json:"elapsed"`
	}
	_ = json.Unmarshal(body, &got)
	if got.Elapsed != 0 {
		t.Errorf("elapsed = %v, want 0", got.Elapsed)
	}
}

func TestAdvanceElapsedHook_MissingSeconds(t *testing.T) {
	fake := fakeusdx.New()
	rec, _ := testHookPost(t, fake, "/_test/advance-elapsed", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- /_test/press-sing ---

func TestPressSingHook_Happy(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}

	rec, body := testHookPost(t, fake, "/_test/press-sing", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}
	var got map[string]string
	_ = json.Unmarshal(body, &got)
	if got["screen"] != "ScreenNextUp" {
		t.Errorf("screen = %q, want ScreenNextUp", got["screen"])
	}
}

func TestPressSingHook_WrongScreen(t *testing.T) {
	fake := fakeusdx.New()
	if err := fake.SetScreen(fakeusdx.ScreenSing); err != nil {
		t.Fatal(err)
	}
	rec, body := testHookPost(t, fake, "/_test/press-sing", nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertErrorBody(t, body, "wrong screen")
}

// --- /_test/press-enter ---

func TestPressEnterHook_Happy(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatal(err)
	}

	rec, body := testHookPost(t, fake, "/_test/press-enter", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}
	var got map[string]string
	_ = json.Unmarshal(body, &got)
	if got["screen"] != "ScreenSing" {
		t.Errorf("screen = %q, want ScreenSing", got["screen"])
	}
}

func TestPressEnterHook_WrongScreen(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := testHookPost(t, fake, "/_test/press-enter", nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertErrorBody(t, body, "wrong screen")
}

func TestPressEnterHook_NoSlot(t *testing.T) {
	fake := fakeusdx.New()
	if err := fake.SetScreen(fakeusdx.ScreenNextUp); err != nil {
		t.Fatal(err)
	}
	rec, body := testHookPost(t, fake, "/_test/press-enter", nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertErrorBody(t, body, "no slot")
}

func TestPressEnterHook_UnknownSong(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatal(err)
	}
	fake.LoadSongs(nil) // wipe library

	rec, body := testHookPost(t, fake, "/_test/press-enter", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	assertErrorBody(t, body, "unknown songId")
}

// --- /_test/press-esc ---

func TestPressEscHook_Happy(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatal(err)
	}

	rec, body := testHookPost(t, fake, "/_test/press-esc", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}
	var got map[string]string
	_ = json.Unmarshal(body, &got)
	if got["screen"] != "ScreenMain" {
		t.Errorf("screen = %q, want ScreenMain", got["screen"])
	}
}

// --- /_test/load-songs ---

func TestLoadSongsHook_Array(t *testing.T) {
	fake := fakeusdx.New()
	body := []map[string]any{
		{"title": "A", "artist": "X", "duet": false},
		{"title": "B", "artist": "Y", "duet": true},
	}
	rec, raw := testHookPost(t, fake, "/_test/load-songs", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, raw)
	}
	var got struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(raw, &got)
	if got.Count != 2 {
		t.Errorf("count = %d, want 2", got.Count)
	}

	_, songsBody := doRequest(t, fake, http.MethodGet, "/songs")
	var songs []map[string]any
	_ = json.Unmarshal(songsBody, &songs)
	if len(songs) != 2 {
		t.Errorf("got %d songs, want 2", len(songs))
	}
}

func TestLoadSongsHook_NonArray(t *testing.T) {
	fake := fakeusdx.New()
	rec, _ := testHookPost(t, fake, "/_test/load-songs", map[string]any{"title": "X"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestLoadSongsHook_Empty(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "X", Artist: "Y", Duet: false}})

	rec, _ := testHookPost(t, fake, "/_test/load-songs", []any{})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	_, songsBody := doRequest(t, fake, http.MethodGet, "/songs")
	if string(songsBody) != "[]" {
		t.Errorf("songs = %q, want []", songsBody)
	}
}

// --- /_test/set-current-playing ---

func TestSetCurrentPlayingHook_Happy(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)

	rec, _ := testHookPost(t, fake, "/_test/set-current-playing", map[string]any{
		"songId":   id,
		"elapsed":  5.0,
		"duration": 200.0,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	var np struct {
		ID       string  `json:"id"`
		Elapsed  float64 `json:"elapsed"`
		Duration float64 `json:"duration"`
	}
	_ = json.Unmarshal(body, &np)
	if np.ID != id || np.Elapsed != 5 || np.Duration != 200 {
		t.Errorf("now-playing = %+v", np)
	}
}

func TestSetCurrentPlayingHook_Unknown(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := testHookPost(t, fake, "/_test/set-current-playing", map[string]any{
		"songId":   "deadbeefdeadbeef",
		"elapsed":  0.0,
		"duration": 100.0,
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	assertErrorBody(t, body, "unknown songId")
}

// --- /_test/set-idle ---

func TestSetIdleHook(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 1, 100); err != nil {
		t.Fatal(err)
	}

	rec, _ := testHookPost(t, fake, "/_test/set-idle", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	if string(body) != "null" {
		t.Errorf("now-playing = %q, want null", body)
	}
}

// --- /_test/set-default-duration ---

func TestSetDefaultDurationHook(t *testing.T) {
	fake := fakeusdx.New()
	rec, _ := testHookPost(t, fake, "/_test/set-default-duration", map[string]any{"seconds": 5.0})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
	// Verify via PressEnter → now-playing duration
	id := seedFake(fake)
	if rec2, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec2.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec2.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatal(err)
	}
	if err := fake.PressEnter(); err != nil {
		t.Fatal(err)
	}
	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	var np struct {
		Duration float64 `json:"duration"`
	}
	_ = json.Unmarshal(body, &np)
	if np.Duration != 5 {
		t.Errorf("duration = %v, want 5", np.Duration)
	}
}

// --- /_test/start-clock + /_test/stop-clock ---

func TestClock_StartAdvancesElapsed(t *testing.T) {
	fake := fakeusdx.New()
	// Short tick so the test completes quickly.
	fake.SetClockInterval(20 * time.Millisecond)
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	// elapsed=0, duration=0.05 — one AdvanceElapsed(1.0) tick terminates it.
	if err := fake.SetCurrentPlaying(id, 0, 0.05); err != nil {
		t.Fatal(err)
	}

	rec, _ := testHookPost(t, fake, "/_test/start-clock", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start: %d", rec.Code)
	}
	defer fake.StopClock()

	// Wait for at least one tick to fire.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fake.Screen() == fakeusdx.ScreenScore {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if fake.Screen() != fakeusdx.ScreenScore {
		t.Errorf("screen = %q, want ScreenScore (clock should have fired)", fake.Screen())
	}
}

func TestClock_StopIsIdempotent(t *testing.T) {
	fake := fakeusdx.New()
	fake.StopClock() // never started — should be no-op
	fake.StartClock()
	fake.StopClock()
	fake.StopClock() // already stopped — no-op
	// If any of those hung or panicked we wouldn't reach here; assert a
	// trivial invariant so the t parameter has a purpose.
	if fake.Screen() != fakeusdx.ScreenMain {
		t.Errorf("screen changed unexpectedly: %q", fake.Screen())
	}
}

func TestClock_StartIsIdempotent(t *testing.T) {
	fake := fakeusdx.New()
	fake.SetClockInterval(10 * time.Millisecond)
	fake.StartClock()
	fake.StartClock() // second call is a no-op

	// Stopping must join the single running goroutine without hanging.
	// If the second Start spawned a zombie, StopClock would fail to join it.
	fake.StopClock()
	// A second StopClock must also be a no-op (no channel to wait on).
	fake.StopClock()
	if fake.Screen() != fakeusdx.ScreenMain {
		t.Errorf("screen changed unexpectedly: %q", fake.Screen())
	}
}
