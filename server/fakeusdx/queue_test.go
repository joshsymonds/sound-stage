package fakeusdx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

// postQueue serves a POST /queue with the given body (already-encoded bytes).
func postQueue(t *testing.T, fake *fakeusdx.Fake, body []byte) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/queue", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	return rec, rec.Body.Bytes()
}

// postQueueJSON JSON-encodes body and POSTs it.
func postQueueJSON(t *testing.T, fake *fakeusdx.Fake, body any) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return postQueue(t, fake, raw)
}

func assertErrorBody(t *testing.T, raw []byte, want string) {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("response body not JSON: %v, body=%q", err, raw)
	}
	if got["error"] != want {
		t.Errorf("error = %q, want %q", got["error"], want)
	}
}

// seedFake loads one well-known song and returns its ID.
func seedFake(fake *fakeusdx.Fake) string {
	fake.LoadSongs([]fakeusdx.Song{
		{Title: "Dancing Queen", Artist: "ABBA", Duet: false},
	})
	return stableid.Compute("ABBA", "Dancing Queen", false)
}

// --- Body validation matrix ---

func TestQueue_MalformedJSON(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueue(t, fake, []byte("not json at all"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "malformed json")
}

func TestQueue_BodyIsArray(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueue(t, fake, []byte(`[1,2,3]`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "body must be an object")
}

func TestQueue_BodyIsNumber(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueue(t, fake, []byte(`42`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "body must be an object")
}

func TestQueue_LegacySinger(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	// Otherwise valid request — legacy rejection must fire before canonical validation succeeds.
	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
		"singer":    "Alice",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "legacy fields removed; use requester")
}

func TestQueue_LegacySingers(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    "abc",
		"requester": "Alice",
		"singers":   []string{"Alice"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "legacy fields removed; use requester")
}

func TestQueue_MissingSongID(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueueJSON(t, fake, map[string]any{"requester": "Alice"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "songId required (string)")
}

func TestQueue_SongIDNumber(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    123,
		"requester": "Alice",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "songId required (string)")
}

func TestQueue_SongIDEmpty(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    "",
		"requester": "Alice",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "songId required (string)")
}

func TestQueue_MissingRequester(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueueJSON(t, fake, map[string]any{"songId": "abc"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "requester required")
}

func TestQueue_RequesterEmpty(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    "abc",
		"requester": "",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "requester required")
}

func TestQueue_PlayersOutOfRange(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    "abc",
		"requester": "Alice",
		"players":   3,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "players must be 1 or 2")
}

func TestQueue_PlayersString(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    "abc",
		"requester": "Alice",
		"players":   "1",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "players must be 1 or 2")
}

func TestQueue_BodyTooLarge(t *testing.T) {
	fake := fakeusdx.New()
	// Pad with 65 KB of requester data so body exceeds 64 KB cap.
	big := strings.Repeat("x", 65*1024)
	raw := []byte(`{"songId":"abc","requester":"` + big + `"}`)
	rec, body := postQueue(t, fake, raw)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
	assertErrorBody(t, body, "body too large")
}

// --- Screen-dependent behavior ---

func TestQueue_ScreenSing_Returns409(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if err := fake.SetScreen(fakeusdx.ScreenSing); err != nil {
		t.Fatalf("SetScreen: %v", err)
	}

	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	})
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertErrorBody(t, body, "song in progress")
	if fake.Slot() != nil {
		t.Errorf("slot = %+v, want nil (must not mutate on 409)", fake.Slot())
	}
}

func TestQueue_ScreenMain_UnknownSongID(t *testing.T) {
	fake := fakeusdx.New()
	seedFake(fake)

	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    "deadbeefdeadbeef",
		"requester": "Alice",
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	assertErrorBody(t, body, "unknown songId")
	if fake.Slot() != nil {
		t.Errorf("slot = %+v, want nil (must not mutate on 404)", fake.Slot())
	}
}

func TestQueue_ScreenMain_Success(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)

	rec, body := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
		"players":   2,
	})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["status"] != "playing" {
		t.Errorf(`status field = %q, want "playing"`, got["status"])
	}

	slot := fake.Slot()
	if slot == nil {
		t.Fatalf("slot not populated")
	}
	if slot.SongID != id {
		t.Errorf("slot.SongID = %q, want %q", slot.SongID, id)
	}
	if slot.Requester != "Alice" {
		t.Errorf("slot.Requester = %q, want %q", slot.Requester, "Alice")
	}
	if slot.Players != 2 {
		t.Errorf("slot.Players = %d, want 2", slot.Players)
	}
	if fake.Screen() != fakeusdx.ScreenMain {
		t.Errorf("screen = %q, want ScreenMain (no auto-transition from ScreenMain)", fake.Screen())
	}
}

func TestQueue_ScreenScore_AutoTransitionsToNextUp(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)
	if err := fake.SetScreen(fakeusdx.ScreenScore); err != nil {
		t.Fatalf("SetScreen: %v", err)
	}

	rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if fake.Slot() == nil {
		t.Fatalf("slot not populated")
	}
	if fake.Screen() != fakeusdx.ScreenNextUp {
		t.Errorf("screen = %q, want ScreenNextUp (push handoff from ScreenScore)", fake.Screen())
	}
}

func TestQueue_ScreenNextUp_NewestWins(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{
		{Title: "Song A", Artist: "X", Duet: false},
		{Title: "Song B", Artist: "Y", Duet: false},
	})
	idA := stableid.Compute("X", "Song A", false)
	idB := stableid.Compute("Y", "Song B", false)

	// First queue from ScreenScore → transitions to ScreenNextUp.
	if err := fake.SetScreen(fakeusdx.ScreenScore); err != nil {
		t.Fatalf("SetScreen: %v", err)
	}
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    idA,
		"requester": "Alice",
	}); rec.Code != http.StatusOK {
		t.Fatalf("first queue: status = %d", rec.Code)
	}

	// Second queue on ScreenNextUp → replaces slot, screen unchanged.
	rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    idB,
		"requester": "Bob",
	})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	slot := fake.Slot()
	if slot == nil || slot.SongID != idB {
		t.Errorf("slot.SongID = %v, want %q (newest-wins)", slot, idB)
	}
	if slot != nil && slot.Requester != "Bob" {
		t.Errorf("slot.Requester = %q, want %q", slot.Requester, "Bob")
	}
	if fake.Screen() != fakeusdx.ScreenNextUp {
		t.Errorf("screen = %q, want ScreenNextUp", fake.Screen())
	}
}

// --- Session lock ---

func TestSessionLock_FirstQueueSetsPlayers(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)

	if fake.SessionPlayers() != 0 {
		t.Errorf("initial SessionPlayers = %d, want 0", fake.SessionPlayers())
	}

	rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
		"players":   2,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if fake.SessionPlayers() != 2 {
		t.Errorf("SessionPlayers after first queue = %d, want 2", fake.SessionPlayers())
	}
}

func TestSessionLock_DefaultPlayersIs1(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)

	rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if fake.SessionPlayers() != 1 {
		t.Errorf("SessionPlayers = %d, want 1 (default)", fake.SessionPlayers())
	}
}

func TestSessionLock_PersistsAcrossScreenTransitions(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)

	// First /queue on ScreenMain with players=2 locks the session.
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
		"players":   2,
	}); rec.Code != http.StatusOK {
		t.Fatalf("first queue: %d", rec.Code)
	}
	if fake.SessionPlayers() != 2 {
		t.Fatalf("after first queue SessionPlayers = %d, want 2", fake.SessionPlayers())
	}

	// Simulate progression into ScreenScore (real USDX would do this after song ends).
	if err := fake.SetScreen(fakeusdx.ScreenScore); err != nil {
		t.Fatalf("SetScreen: %v", err)
	}

	// Second /queue with players=1 — lock holds.
	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Bob",
		"players":   1,
	}); rec.Code != http.StatusOK {
		t.Fatalf("second queue: %d", rec.Code)
	}
	if fake.SessionPlayers() != 2 {
		t.Errorf("SessionPlayers after 2nd queue = %d, want 2 (lock must hold)", fake.SessionPlayers())
	}
}

func TestSessionLock_ResetsOnReturnToScreenMain(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)

	if rec, _ := postQueueJSON(t, fake, map[string]any{
		"songId":    id,
		"requester": "Alice",
		"players":   2,
	}); rec.Code != http.StatusOK {
		t.Fatalf("first queue: %d", rec.Code)
	}
	if fake.SessionPlayers() != 2 {
		t.Fatalf("after first queue: %d", fake.SessionPlayers())
	}

	// User returns to ScreenMain → session ends.
	if err := fake.SetScreen(fakeusdx.ScreenScore); err != nil {
		t.Fatalf("SetScreen ScreenScore: %v", err)
	}
	if err := fake.SetScreen(fakeusdx.ScreenMain); err != nil {
		t.Fatalf("SetScreen ScreenMain: %v", err)
	}
	if fake.SessionPlayers() != 0 {
		t.Errorf("after return to ScreenMain SessionPlayers = %d, want 0", fake.SessionPlayers())
	}
}

// --- Method / screen validation ---

func TestQueue_GetReturns405(t *testing.T) {
	fake := fakeusdx.New()
	req := httptest.NewRequest(http.MethodGet, "/queue", nil)
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestSetScreen_InvalidValue(t *testing.T) {
	fake := fakeusdx.New()
	if err := fake.SetScreen(fakeusdx.Screen("bogus")); err == nil {
		t.Error("SetScreen with invalid value: err = nil, want error")
	}
	if fake.Screen() != fakeusdx.ScreenMain {
		t.Errorf("screen changed after invalid SetScreen: %q", fake.Screen())
	}
}

func TestInitialState(t *testing.T) {
	fake := fakeusdx.New()
	if fake.Screen() != fakeusdx.ScreenMain {
		t.Errorf("initial screen = %q, want ScreenMain", fake.Screen())
	}
	if fake.Slot() != nil {
		t.Errorf("initial slot = %+v, want nil", fake.Slot())
	}
	if fake.SessionPlayers() != 0 {
		t.Errorf("initial SessionPlayers = %d, want 0", fake.SessionPlayers())
	}
}

// --- Concurrency ---

func TestConcurrentQueueAndReads(t *testing.T) {
	fake := fakeusdx.New()
	id := seedFake(fake)

	validBody, err := json.Marshal(map[string]any{"songId": id, "requester": "Alice"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			req := httptest.NewRequest(http.MethodPost, "/queue", bytes.NewReader(validBody))
			req.Header.Set("Content-Type", "application/json")
			fake.ServeHTTP(httptest.NewRecorder(), req)
		}
	})

	wg.Go(func() {
		screens := []fakeusdx.Screen{fakeusdx.ScreenMain, fakeusdx.ScreenScore, fakeusdx.ScreenNextUp}
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = fake.SetScreen(screens[i%len(screens)])
			i++
		}
	})

	for range 5 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				doRequest(t, fake, http.MethodGet, "/songs")
				_ = fake.Slot()
				_ = fake.SessionPlayers()
			}
		})
	}

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}
