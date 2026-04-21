package server_test

import (
	"net/http/httptest"
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

	driver.Stop() // never started — no-op
	driver.Start()
	driver.Stop()
	driver.Stop() // already stopped — no-op
}

func TestQueueDriver_TickInterval(t *testing.T) {
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

	// Queue a second song; the next tick should stage it (replacing slot A,
	// newest-wins per API.md).
	queue.Add(server.Song{ID: idB, Title: "B", Artist: "Y"}, "Bob")

	if !waitFor(500*time.Millisecond, func() bool {
		slot := fake.Slot()
		return slot != nil && slot.SongID == idB
	}) {
		t.Fatalf("slot B never populated; got %+v", fake.Slot())
	}
}

func TestNewQueueDriver_EmptyDeckURLReturnsNil(t *testing.T) {
	t.Parallel()
	driver := server.NewQueueDriver("", server.NewQueue(), tickInterval)
	if driver != nil {
		t.Errorf("NewQueueDriver(\"\", ...) = %v, want nil", driver)
	}
}
