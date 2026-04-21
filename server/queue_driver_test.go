package server_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

const tickInterval = 20 * time.Millisecond

// setupDriver stands up a fakeusdx httptest server, a real server.Queue, and
// a QueueDriver configured to tick fast enough for tests. The httptest
// lifecycle is managed via t.Cleanup so the caller only needs the fake, the
// queue, and the driver.
func setupDriver(t *testing.T) (*fakeusdx.Fake, *server.Queue, *server.QueueDriver) {
	t.Helper()
	fake := fakeusdx.New()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	queue := server.NewQueue()
	driver := server.NewQueueDriver(srv.URL, queue, tickInterval)
	if driver == nil {
		t.Fatal("NewQueueDriver returned nil; deck URL was empty?")
	}
	t.Cleanup(driver.Stop)
	return fake, queue, driver
}

// waitFor polls fn every 5ms for up to timeout; returns true if fn returned true.
func waitFor(timeout time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fn()
}

func TestQueueDriver_StagesFromIdleDeck(t *testing.T) {
	t.Parallel()
	fake, queue, driver := setupDriver(t)

	fake.LoadSongs([]fakeusdx.Song{{Title: "Dancing Queen", Artist: "ABBA", Duet: false}})
	id := stableid.Compute("ABBA", "Dancing Queen", false)
	queue.Add(server.Song{ID: id, Title: "Dancing Queen", Artist: "ABBA"}, "Alice")

	driver.Start()

	if !waitFor(500*time.Millisecond, func() bool { return fake.Slot() != nil }) {
		t.Fatal("slot never populated")
	}
	slot := fake.Slot()
	if slot.SongID != id {
		t.Errorf("slot.SongID = %q, want %q", slot.SongID, id)
	}
	if slot.Requester != "Alice" {
		t.Errorf("slot.Requester = %q, want Alice", slot.Requester)
	}
}

func TestQueueDriver_BusyDeckSkipsTick(t *testing.T) {
	t.Parallel()
	fake, queue, driver := setupDriver(t)

	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	busyID := stableid.Compute("A", "T", false)
	// Put the fake into a playing state so /now-playing returns non-null.
	if err := fake.SetCurrentPlaying(busyID, 10, 200); err != nil {
		t.Fatal(err)
	}

	// Queue a different song — driver should not stage it while the deck is busy.
	queueID := "deadbeefdeadbeef"
	queue.Add(server.Song{ID: queueID, Title: "Queued", Artist: "Q"}, "Alice")

	driver.Start()
	time.Sleep(3 * tickInterval)
	if fake.Slot() != nil {
		t.Errorf("slot = %+v, want nil (busy deck must skip tick)", fake.Slot())
	}
}

func TestQueueDriver_EmptyQueueNoOp(t *testing.T) {
	t.Parallel()
	fake, _, driver := setupDriver(t)

	driver.Start()
	time.Sleep(3 * tickInterval)
	if fake.Slot() != nil {
		t.Errorf("slot = %+v, want nil (empty queue must not POST)", fake.Slot())
	}
}

func TestQueueDriver_UnknownSongIDDropsEntry(t *testing.T) {
	t.Parallel()
	fake, queue, driver := setupDriver(t)

	// Queue an entry whose ID doesn't exist in the fake's library.
	queue.Add(server.Song{ID: "deadbeefdeadbeef", Title: "Ghost", Artist: "None"}, "Alice")

	driver.Start()
	// Wait long enough that the 404 response has been handled.
	time.Sleep(3 * tickInterval)

	// Slot stays empty (the fake 404'd), AND the entry was consumed from the
	// queue (not re-queued). Next() returns nil.
	if fake.Slot() != nil {
		t.Errorf("slot = %+v, want nil", fake.Slot())
	}
	if next := queue.Next(); next != nil {
		t.Errorf("queue still has entry %+v after 404 drop", next)
	}
}

func TestQueueDriver_5xxRequeuesSong(t *testing.T) {
	t.Parallel()
	fake, queue, driver := setupDriver(t)

	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)
	// Inject enough 500 responses that every tick during the test window
	// hits a 5xx. ReAdd should fire each time, leaving the song in the queue.
	fake.QueueInjection("/queue", http.StatusInternalServerError, 100)

	queue.Add(server.Song{ID: id, Title: "T", Artist: "A"}, "Alice")

	driver.Start()
	time.Sleep(3 * tickInterval)
	driver.Stop()

	// Song must be back in the queue via ReAdd after the 500.
	next := queue.Next()
	if next == nil {
		t.Fatal("queue empty; 500 should have re-queued the song via ReAdd")
	}
	if next.Song.ID != id {
		t.Errorf("re-queued song.ID = %q, want %q", next.Song.ID, id)
	}
}

func TestQueueDriver_409RequeuesSong(t *testing.T) {
	t.Parallel()
	fake, queue, driver := setupDriver(t)

	fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
	id := stableid.Compute("A", "T", false)

	// Drive fake into the "ScreenSing + playing=nil" state: /now-playing
	// returns null (so driver proceeds), but POST /queue returns 409.
	if err := fake.SetCurrentPlaying(id, 0, 100); err != nil {
		t.Fatal(err)
	}
	fake.SetIdle() // clears playing; screen stays ScreenSing

	queue.Add(server.Song{ID: id, Title: "T", Artist: "A"}, "Alice")

	driver.Start()
	// Give the driver enough time to make at least one 409-ing attempt.
	time.Sleep(3 * tickInterval)
	driver.Stop()

	// Song must still be in the queue (re-added after 409).
	next := queue.Next()
	if next == nil {
		t.Fatal("queue is empty; 409 should have re-queued the song")
	}
	if next.Song.ID != id {
		t.Errorf("re-queued song.ID = %q, want %q", next.Song.ID, id)
	}
	// Slot must not have been populated (every attempt 409s).
	if fake.Slot() != nil {
		t.Errorf("slot = %+v, want nil", fake.Slot())
	}
}

func TestQueueDriver_StopIsIdempotent(t *testing.T) {
	t.Parallel()
	_, _, driver := setupDriver(t)

	// Stop before Start is a no-op.
	driver.Stop()

	// Start → Stop must fully tear down the run goroutine. If it leaks, the
	// second Start below would spawn a second goroutine sharing d.queue's
	// mutex, and the third Stop would hang waiting on the first goroutine's
	// never-closed d.done. Test success implies clean tear-down.
	driver.Start()
	driver.Stop()

	// Second Stop after tear-down is a no-op (no channels to close).
	driver.Stop()

	// Start again — channels must be re-created. If sync.Once had fired on
	// the first Stop, this Start would spawn a goroutine that Stop could no
	// longer signal, and the next Stop would hang.
	driver.Start()
	driver.Stop()
}

func TestQueueDriver_SlowDeckDoesNotStallTicker(t *testing.T) {
	t.Parallel()
	// Wrap fakeusdx in a handler that sleeps on /now-playing, simulating a
	// flaky Deck. The driver's bounded tickSem should prevent ticks from
	// queueing up behind a slow request.
	fake := fakeusdx.New()
	var requestCount atomic.Int32
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/now-playing" {
			requestCount.Add(1)
			time.Sleep(150 * time.Millisecond)
		}
		fake.ServeHTTP(w, r)
	}))
	t.Cleanup(slow.Close)

	queue := server.NewQueue()
	driver := server.NewQueueDriver(slow.URL, queue, 20*time.Millisecond)
	if driver == nil {
		t.Fatal("NewQueueDriver returned nil")
	}
	driver.Start()

	// Let the ticker fire many times; with a 20 ms interval over 300 ms that's
	// ~15 potential ticks, but each /now-playing takes 150 ms. Without the
	// tickSem bound, the run goroutine would serialize behind the slow
	// requests; with it, most ticks skip.
	time.Sleep(300 * time.Millisecond)
	driver.Stop()

	got := int(requestCount.Load())
	if got == 0 {
		t.Fatal("no /now-playing requests fired at all")
	}
	// In-flight ticks are bounded to 1; 300 ms / 150 ms ≈ 2 slow round-trips.
	// Allow up to 4 to account for scheduling variance. If this ever sees 15+
	// it means we're queueing ticks behind the slow call.
	if got > 4 {
		t.Errorf("request count = %d; bounded tickSem should cap in-flight to ~2", got)
	}
}

func TestQueueDriver_SlotBackPressure(t *testing.T) {
	t.Parallel()
	fake, queue, driver := setupDriver(t)
	fake.LoadSongs([]fakeusdx.Song{
		{Title: "A", Artist: "X", Duet: false},
		{Title: "B", Artist: "Y", Duet: false},
	})

	idA := stableid.Compute("X", "A", false)
	idB := stableid.Compute("Y", "B", false)

	queue.Add(server.Song{ID: idA, Title: "A", Artist: "X"}, "Alice")

	driver.Start()
	if !waitFor(500*time.Millisecond, func() bool {
		slot := fake.Slot()
		return slot != nil && slot.SongID == idA
	}) {
		t.Fatal("slot A never populated")
	}

	// Queue a second song while slot A sits un-pulled. Driver must NOT
	// overwrite the slot — that would silently drop song A. Slot stays A;
	// song B waits in the internal queue.
	queue.Add(server.Song{ID: idB, Title: "B", Artist: "Y"}, "Bob")
	time.Sleep(3 * tickInterval)

	if slot := fake.Slot(); slot == nil || slot.SongID != idA {
		t.Errorf("slot = %+v, want still song A (back-pressure on populated slot)", slot)
	}
	// Song B must still be waiting in the internal queue.
	if next := queue.Next(); next == nil || next.Song.ID != idB {
		t.Errorf("queue.Next() = %+v, want song B", next)
	}

	// Drain the slot (simulate user pulling via Sing + Enter). Next tick
	// should now stage song B — but we already popped it above, so re-add
	// and verify the driver picks it up.
	queue.Add(server.Song{ID: idB, Title: "B", Artist: "Y"}, "Bob")
	if err := fake.PressSing(); err != nil {
		t.Fatal(err)
	}
	if err := fake.PressEnter(); err != nil {
		t.Fatal(err)
	}
	// After PressEnter the fake is on ScreenSing; to let the driver stage
	// song B we need the fake idle again. SetIdle + SetScreen(ScreenMain)
	// brings us back to a stage-ready state.
	fake.SetIdle()
	if err := fake.SetScreen(fakeusdx.ScreenMain); err != nil {
		t.Fatal(err)
	}

	if !waitFor(500*time.Millisecond, func() bool {
		slot := fake.Slot()
		return slot != nil && slot.SongID == idB
	}) {
		t.Fatalf("slot B never populated after slot A was consumed; got %+v", fake.Slot())
	}
}

func TestNewQueueDriver_EmptyDeckURLReturnsNil(t *testing.T) {
	t.Parallel()
	driver := server.NewQueueDriver("", server.NewQueue(), tickInterval)
	if driver != nil {
		t.Errorf("NewQueueDriver(\"\", ...) = %v, want nil", driver)
	}
}
