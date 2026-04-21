package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

// TestEndToEnd_TwoSongAutoAdvance walks the entire sound-stage stack
// end-to-end against a real fakeusdx USDX stand-in:
//
//  1. Library scanner picks up on-disk .txt files and exposes them via
//     GET /api/songs with stableid-computed IDs.
//  2. POST /api/queue accepts a songId string; Queue.Add enqueues.
//  3. QueueDriver ticks, GETs /now-playing (null), POSTs /queue to fake.
//  4. Fake stages the slot; PressSing + PressEnter promotes the song to
//     ScreenSing and starts playback.
//  5. GET /api/now-playing proxies the fake's response back through the
//     real server.
//  6. Auto-clock advances elapsed past duration; fake transitions to
//     ScreenScore.
//  7. Second /api/queue POST while ScreenScore triggers the push-handoff
//     transition on the fake (ScreenScore → ScreenNextUp).
//  8. PressEnter starts song B.
//
// If the stableid formula drifts between sound-stage's scanner and the
// fakeusdx LoadSongs path, step 4 404s on /queue. If the driver misreads
// /now-playing, it stalls. If the proxy mangles body shape, step 5 fails.
// The test is the safety net that catches integration-level drift that
// unit tests miss.
func TestEndToEnd_TwoSongAutoAdvance(t *testing.T) {
	// Stand up the fake with short auto-clock and duration so the test
	// completes in a few hundred ms rather than multiple seconds.
	fake := fakeusdx.New()
	fake.SetDefaultDuration(0.1)
	fake.SetClockInterval(5 * time.Millisecond)

	deckSrv := httptest.NewServer(fake)
	defer deckSrv.Close()

	// On-disk library: the scanner picks these up via GET /api/songs.
	// Also seed the fake's in-memory library — real USDX does the same scan,
	// producing identical stableid hashes.
	libraryDir := t.TempDir()
	writeSongTxt(t, libraryDir, "ABBA", "Dancing Queen")
	writeSongTxt(t, libraryDir, "a-ha", "Take On Me")
	fake.LoadSongs([]fakeusdx.Song{
		{Title: "Dancing Queen", Artist: "ABBA", Duet: false},
		{Title: "Take On Me", Artist: "a-ha", Duet: false},
	})

	queue := server.NewQueue()
	appHandler := server.HandlerWithQueue(server.Config{
		LibraryDir: libraryDir,
		DeckURL:    deckSrv.URL,
	}, queue)
	appSrv := httptest.NewServer(appHandler)
	defer appSrv.Close()

	driver := server.NewQueueDriver(deckSrv.URL, queue, 20*time.Millisecond)
	if driver == nil {
		t.Fatal("NewQueueDriver returned nil")
	}
	driver.Start()
	defer driver.Stop()

	fake.StartClock()
	defer fake.StopClock()

	// Step 1: GET /api/songs — expect 2 entries with stableid hashes.
	songs := fetchSongs(t, appSrv.URL)
	if len(songs) != 2 {
		t.Fatalf("songs = %d, want 2", len(songs))
	}
	idsByTitle := map[string]string{}
	for _, s := range songs {
		idsByTitle[s.Title] = s.ID
	}
	idA := stableid.Compute("ABBA", "Dancing Queen", false)
	idB := stableid.Compute("a-ha", "Take On Me", false)
	if idsByTitle["Dancing Queen"] != idA {
		t.Fatalf("song A id = %q, want %q (scanner must match stableid)",
			idsByTitle["Dancing Queen"], idA)
	}
	if idsByTitle["Take On Me"] != idB {
		t.Fatalf("song B id = %q, want %q", idsByTitle["Take On Me"], idB)
	}

	// Step 2: POST /api/queue for song A.
	postQueueAdd(t, appSrv.URL, idA, "Dancing Queen", "ABBA", "Alice")

	// Step 3: driver should stage the song within a few ticks.
	if !waitFor(2*time.Second, func() bool {
		slot := fake.Slot()
		return slot != nil && slot.SongID == idA
	}) {
		t.Fatalf("slot A never staged; fake state: %s", dumpFakeState(t, deckSrv.URL))
	}

	// Step 4: Deck user pulls the slot.
	if err := fake.PressSing(); err != nil {
		t.Fatalf("PressSing: %v", err)
	}
	if err := fake.PressEnter(); err != nil {
		t.Fatalf("PressEnter: %v", err)
	}

	// Step 5: GET /api/now-playing returns song A.
	if !waitFor(1*time.Second, func() bool {
		return nowPlayingID(t, appSrv.URL) == idA
	}) {
		t.Fatalf("now-playing never reported song A; got %q", nowPlayingID(t, appSrv.URL))
	}

	// Step 6: song auto-completes (duration 0.1s, clock ticks every 5ms).
	if !waitFor(1*time.Second, func() bool {
		return fake.Screen() == fakeusdx.ScreenScore
	}) {
		t.Fatalf("never reached ScreenScore; fake state: %s", dumpFakeState(t, deckSrv.URL))
	}

	// Step 7: queue song B while on ScreenScore — push handoff should
	// auto-transition the fake's screen to ScreenNextUp.
	postQueueAdd(t, appSrv.URL, idB, "Take On Me", "a-ha", "Bob")

	if !waitFor(2*time.Second, func() bool {
		slot := fake.Slot()
		return slot != nil && slot.SongID == idB && fake.Screen() == fakeusdx.ScreenNextUp
	}) {
		t.Fatalf("slot B never staged with ScreenNextUp transition; fake state: %s",
			dumpFakeState(t, deckSrv.URL))
	}

	// Step 8: PressEnter starts song B.
	if err := fake.PressEnter(); err != nil {
		t.Fatalf("PressEnter B: %v", err)
	}
	if !waitFor(1*time.Second, func() bool {
		return nowPlayingID(t, appSrv.URL) == idB
	}) {
		t.Fatalf("now-playing never reported song B; got %q", nowPlayingID(t, appSrv.URL))
	}
}

// fetchSongs does GET /api/songs and decodes the response.
func fetchSongs(t *testing.T, appURL string) []server.Song {
	t.Helper()
	ctx := t.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appURL+"/api/songs", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/songs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/songs: %d", resp.StatusCode)
	}
	var songs []server.Song
	if err := json.NewDecoder(resp.Body).Decode(&songs); err != nil {
		t.Fatalf("decode songs: %v", err)
	}
	return songs
}

// postQueueAdd does POST /api/queue with the given songId + metadata + guest.
func postQueueAdd(t *testing.T, appURL, songID, title, artist, guest string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"songId": songID,
		"title":  title,
		"artist": artist,
		"guest":  guest,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appURL+"/api/queue", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/queue: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/queue: %d", resp.StatusCode)
	}
}

// nowPlayingID returns the "id" field from GET /api/now-playing, or "" on null.
func nowPlayingID(t *testing.T, appURL string) string {
	t.Helper()
	ctx := t.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appURL+"/api/now-playing", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil || string(bytes.TrimSpace(raw)) == "null" {
		return ""
	}
	var np struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &np); err != nil {
		return ""
	}
	return np.ID
}

// dumpFakeState returns the fake's /debug/state as a string (best-effort;
// empty on error). Used only for test failure messages.
func dumpFakeState(t *testing.T, deckURL string) string {
	t.Helper()
	ctx := t.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, deckURL+"/debug/state", nil)
	if err != nil {
		return fmt.Sprintf("(request build failed: %v)", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf("(request failed: %v)", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return string(raw)
}
