package fakeusdx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

// --- PressSing ---

func TestPressSing_MainWithSlot_GoesToNextUp(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}

	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing: %v", err)
	}
	if fake.Screen() != fakeusdx.ScreenNextUp {
		t.Errorf("screen = %q, want ScreenNextUp", fake.Screen())
	}
	if slot := fake.Slot(); slot == nil || slot.SongID != id {
		t.Errorf("slot changed or cleared: %+v", slot)
	}
}

func TestPressSing_MainWithoutSlot_Noop(t *testing.T) {
	fake := fakeusdx.New()
	if err := fake.PressSing(); err != nil {
		t.Errorf("PressSing without slot: err = %v, want nil", err)
	}
	if fake.Screen() != fakeusdx.ScreenMain {
		t.Errorf("screen = %q, want ScreenMain (no-op)", fake.Screen())
	}
}

func TestPressSing_WrongScreen(t *testing.T) {
	for _, screen := range []fakeusdx.Screen{fakeusdx.ScreenSing, fakeusdx.ScreenScore, fakeusdx.ScreenNextUp} {
		t.Run(string(screen), func(t *testing.T) {
			fake := fakeusdx.New()
			if err := fake.SetScreen(screen); err != nil {
				t.Fatalf("SetScreen: %v", err)
			}
			if err := fake.PressSing(); !errors.Is(err, fakeusdx.ErrWrongScreen) {
				t.Errorf("PressSing on %s: err = %v, want ErrWrongScreen", screen, err)
			}
			if fake.Screen() != screen {
				t.Errorf("screen changed to %q after error", fake.Screen())
			}
		})
	}
}

// --- PressEnter ---

func TestPressEnter_NextUpWithSlot_GoesToSing(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
		"players":   2,
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing: %v", err)
	}

	if err := fake.PressEnter(); err != nil {
		t.Fatalf("PressEnter: %v", err)
	}
	if fake.Screen() != fakeusdx.ScreenSing {
		t.Errorf("screen = %q, want ScreenSing", fake.Screen())
	}
	if fake.Slot() != nil {
		t.Errorf("slot = %+v, want nil (consumed)", fake.Slot())
	}
	if fake.SessionPlayers() != 2 {
		t.Errorf("sessionPlayers = %d, want 2", fake.SessionPlayers())
	}

	// currentSong populated with elapsed=0, duration=default (180)
	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	var np struct {
		ID       string  `json:"id"`
		Elapsed  float64 `json:"elapsed"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(body, &np); err != nil {
		t.Fatalf("unmarshal now-playing: %v, body=%s", err, body)
	}
	if np.ID != id {
		t.Errorf("now-playing.id = %q, want %q", np.ID, id)
	}
	if np.Elapsed != 0 {
		t.Errorf("now-playing.elapsed = %v, want 0", np.Elapsed)
	}
	if np.Duration != 180 {
		t.Errorf("now-playing.duration = %v, want 180 (default)", np.Duration)
	}
}

func TestPressEnter_NoSlot(t *testing.T) {
	fake := fakeusdx.New()
	if err := fake.SetScreen(fakeusdx.ScreenNextUp); err != nil {
		t.Fatalf("SetScreen: %v", err)
	}
	if err := fake.PressEnter(); !errors.Is(err, fakeusdx.ErrNoSlot) {
		t.Errorf("err = %v, want ErrNoSlot", err)
	}
	if fake.Screen() != fakeusdx.ScreenNextUp {
		t.Errorf("screen = %q, state must not change on error", fake.Screen())
	}
}

func TestPressEnter_SongRemovedFromLibrary(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing: %v", err)
	}

	// Remove the song from the library between PressSing and PressEnter.
	fake.LoadSongs(nil)

	if err := fake.PressEnter(); !errors.Is(err, fakeusdx.ErrUnknownSongID) {
		t.Errorf("err = %v, want ErrUnknownSongID", err)
	}
	if fake.Screen() != fakeusdx.ScreenNextUp {
		t.Errorf("screen = %q, state must not change on error", fake.Screen())
	}
	if fake.Slot() == nil {
		t.Errorf("slot = nil, must not be consumed on error")
	}
}

func TestPressEnter_WrongScreen(t *testing.T) {
	for _, screen := range []fakeusdx.Screen{fakeusdx.ScreenMain, fakeusdx.ScreenSing, fakeusdx.ScreenScore} {
		t.Run(string(screen), func(t *testing.T) {
			fake := fakeusdx.New()
			if err := fake.SetScreen(screen); err != nil {
				t.Fatalf("SetScreen: %v", err)
			}
			if err := fake.PressEnter(); !errors.Is(err, fakeusdx.ErrWrongScreen) {
				t.Errorf("err = %v, want ErrWrongScreen", err)
			}
		})
	}
}

func TestPressEnter_ClearsPaused(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing: %v", err)
	}

	if err := fake.PressEnter(); err != nil {
		t.Fatalf("PressEnter: %v", err)
	}

	// New song must start unpaused: a subsequent /resume should 409.
	rec, body := post(t, fake, "/resume")
	if rec.Code != http.StatusConflict {
		t.Errorf("resume status = %d, want 409 (not paused)", rec.Code)
	}
	assertErrorBody(t, body, "nothing to resume")
}

// --- PressEsc ---

func TestPressEsc_NextUp_ReturnsToMainPreservingSlot(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
		"players":   2,
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing: %v", err)
	}

	if err := fake.PressEsc(); err != nil {
		t.Fatalf("PressEsc: %v", err)
	}
	if fake.Screen() != fakeusdx.ScreenMain {
		t.Errorf("screen = %q, want ScreenMain", fake.Screen())
	}
	slot := fake.Slot()
	if slot == nil || slot.SongID != id {
		t.Errorf("slot not preserved: %+v", slot)
	}
	// Per API.md: session ends on return to ScreenMain.
	if fake.SessionPlayers() != 0 {
		t.Errorf("sessionPlayers = %d, want 0 (Esc returns to ScreenMain → session ends)", fake.SessionPlayers())
	}
}

func TestPressEsc_WrongScreen(t *testing.T) {
	for _, screen := range []fakeusdx.Screen{fakeusdx.ScreenMain, fakeusdx.ScreenSing, fakeusdx.ScreenScore} {
		t.Run(string(screen), func(t *testing.T) {
			fake := fakeusdx.New()
			if err := fake.SetScreen(screen); err != nil {
				t.Fatalf("SetScreen: %v", err)
			}
			if err := fake.PressEsc(); !errors.Is(err, fakeusdx.ErrWrongScreen) {
				t.Errorf("err = %v, want ErrWrongScreen", err)
			}
		})
	}
}

func TestPressEsc_ThenSingReentersWithSlot(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing: %v", err)
	}
	if err := fake.PressEsc(); err != nil {
		t.Fatalf("PressEsc: %v", err)
	}

	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing after Esc: %v", err)
	}
	if fake.Screen() != fakeusdx.ScreenNextUp {
		t.Errorf("screen = %q, want ScreenNextUp", fake.Screen())
	}
	if slot := fake.Slot(); slot == nil || slot.SongID != id {
		t.Errorf("slot changed: %+v", slot)
	}
}

// --- AdvanceElapsed ---

func TestAdvanceElapsed_NotPlaying(t *testing.T) {
	fake := fakeusdx.New()
	got := fake.AdvanceElapsed(10.0)
	if got != 0 {
		t.Errorf("returned = %v, want 0 when not playing", got)
	}
}

func TestAdvanceElapsed_Paused(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}
	if rec, _ := post(t, fake, "/pause"); rec.Code != http.StatusOK {
		t.Fatalf("pause: %d", rec.Code)
	}

	got := fake.AdvanceElapsed(30.0)
	if got != 10 {
		t.Errorf("returned = %v, want 10 (elapsed frozen while paused)", got)
	}
}

func TestAdvanceElapsed_NonTerminal(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	got := fake.AdvanceElapsed(5.0)
	if got != 15 {
		t.Errorf("returned = %v, want 15", got)
	}
	// Still playing, still ScreenSing.
	if fake.Screen() != fakeusdx.ScreenSing {
		t.Errorf("screen = %q, want ScreenSing", fake.Screen())
	}
}

func TestAdvanceElapsed_Terminal(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 100, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	fake.AdvanceElapsed(150.0) // overshoots 200

	if fake.Screen() != fakeusdx.ScreenScore {
		t.Errorf("screen = %q, want ScreenScore", fake.Screen())
	}
	// /now-playing should return null now.
	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	if string(body) != "null" {
		t.Errorf("now-playing = %q, want null", body)
	}
}

func TestAdvanceElapsed_Zero(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	got := fake.AdvanceElapsed(0)
	if got != 10 {
		t.Errorf("returned = %v, want 10 (no-op)", got)
	}
}

// --- SetDefaultDuration ---

func TestSetDefaultDuration_AffectsPressEnter(t *testing.T) {
	fake := fakeusdx.New()
	fake.SetDefaultDuration(42.5)
	id := seedFake(fake)
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue: %d", rec.Code)
	}
	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing: %v", err)
	}
	if err := fake.PressEnter(); err != nil {
		t.Fatalf("PressEnter: %v", err)
	}

	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	var np struct {
		Duration float64 `json:"duration"`
	}
	_ = json.Unmarshal(body, &np)
	if np.Duration != 42.5 {
		t.Errorf("duration = %v, want 42.5", np.Duration)
	}
}

// --- End-to-end: two-song auto-advance with push handoff ---

func TestEndToEnd_TwoSongAutoAdvance(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{
		{Title: "Song A", Artist: "Artist A", Duet: false},
		{Title: "Song B", Artist: "Artist B", Duet: false},
	})
	idA := stableid.Compute("Artist A", "Song A", false)
	idB := stableid.Compute("Artist B", "Song B", false)
	fake.SetDefaultDuration(0.1) // tiny so AdvanceElapsed terminates quickly

	// 1. /queue songA on ScreenMain → slot populated
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    idA,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue A: %d", rec.Code)
	}
	if fake.Screen() != fakeusdx.ScreenMain {
		t.Fatalf("screen = %q, want ScreenMain", fake.Screen())
	}

	// 2. Sing pulls the slot
	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing A: %v", err)
	}
	if fake.Screen() != fakeusdx.ScreenNextUp {
		t.Fatalf("screen = %q, want ScreenNextUp", fake.Screen())
	}

	// 3. Enter starts the song
	if err := fake.PressEnter(); err != nil {
		t.Fatalf("PressEnter A: %v", err)
	}
	if fake.Screen() != fakeusdx.ScreenSing {
		t.Fatalf("screen = %q, want ScreenSing", fake.Screen())
	}

	// 4. Time runs out → ScreenScore
	fake.AdvanceElapsed(1.0)
	if fake.Screen() != fakeusdx.ScreenScore {
		t.Fatalf("screen = %q, want ScreenScore", fake.Screen())
	}

	// 5. /queue songB while on ScreenScore → push handoff to ScreenNextUp
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    idB,
		"requester": "Bob",
	}); rec.Code != http.StatusOK {
		t.Fatalf("queue B: %d", rec.Code)
	}
	if fake.Screen() != fakeusdx.ScreenNextUp {
		t.Fatalf("screen = %q, want ScreenNextUp (push handoff from ScreenScore)", fake.Screen())
	}

	// 6. Enter starts song B
	if err := fake.PressEnter(); err != nil {
		t.Fatalf("PressEnter B: %v", err)
	}
	if fake.Screen() != fakeusdx.ScreenSing {
		t.Errorf("screen = %q, want ScreenSing", fake.Screen())
	}

	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	var np struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &np)
	if np.ID != idB {
		t.Errorf("now-playing.id = %q, want %q (song B)", np.ID, idB)
	}
}
