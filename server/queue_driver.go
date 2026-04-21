package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// QueueDriver polls the Deck's /now-playing endpoint; when the Deck is idle
// and the queue is non-empty, it stages the next song via POST /queue.
// USDX handles the stage-and-pull semantics from there (ScreenScore auto-
// transitions to ScreenNextUp; ScreenMain waits for the user to press Sing).
type QueueDriver struct {
	deckURL  string
	queue    *Queue
	interval time.Duration
	client   *http.Client
	logger   *slog.Logger
	// lifecycle state; all writes go through lifecycleMu. A running driver has
	// non-nil stop/done; a stopped driver has nil/nil. Idempotency checks read
	// under the lock, so Start/Stop can be called in any order any number of
	// times without leaking goroutines.
	lifecycleMu sync.Mutex
	stop        chan struct{}
	done        chan struct{}
	// tickSem bounds in-flight ticks to 1. When a tick takes longer than the
	// tick interval, subsequent ticks skip rather than queue up behind it.
	tickSem chan struct{}
	// tickWG tracks spawned tick goroutines so Stop can wait for them before
	// returning.
	tickWG sync.WaitGroup
}

// NewQueueDriver returns a driver configured to talk to deckURL, or nil if
// deckURL is empty (the server accepts "no deck" as a valid configuration).
// Uses a dedicated http.Client with its own connection pool so a busy or
// misbehaving Deck can't interfere with http.DefaultClient traffic.
func NewQueueDriver(deckURL string, queue *Queue, interval time.Duration) *QueueDriver {
	if deckURL == "" {
		return nil
	}
	return &QueueDriver{
		deckURL:  deckURL,
		queue:    queue,
		interval: interval,
		client:   newDeckClient(),
		logger:   slog.Default(),
		tickSem:  make(chan struct{}, 1),
	}
}

const (
	deckMaxIdleConns        = 4
	deckMaxIdleConnsPerHost = 2
	deckIdleConnTimeout     = 90 * time.Second
	// driverRequestTimeout is intentionally shorter than proxyTimeout (5 s,
	// used by user-facing playback proxies). The driver ticks every 2 s; a 5 s
	// per-request timeout would stall multiple ticks on a single slow call.
	// 1500 ms keeps the tick's worst-case block comfortably under the cadence.
	driverRequestTimeout = 1500 * time.Millisecond
)

// newDeckClient builds an http.Client scoped to Deck communication: its own
// connection pool, idle-connection reaper, and a wall-clock timeout that
// acts as a backstop if a request ignores its context deadline.
func newDeckClient() *http.Client {
	return &http.Client{
		Timeout: driverRequestTimeout + time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        deckMaxIdleConns,
			MaxIdleConnsPerHost: deckMaxIdleConnsPerHost,
			IdleConnTimeout:     deckIdleConnTimeout,
		},
	}
}

// Start begins ticking in a background goroutine. Idempotent: a second Start
// while already running is a no-op.
func (d *QueueDriver) Start() {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()
	if d.stop != nil {
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	d.stop = stop
	d.done = done
	// Pass channels into run by value; never re-read from struct fields in
	// the goroutine. This side-steps the race where a fast Start→Stop
	// sequence could clear d.stop before run() observes it.
	go d.run(stop, done)
}

// Stop signals the ticker to exit and waits for the goroutine to finish.
// Idempotent: Stop on a never-started driver is a no-op, and consecutive
// Stop calls after Start are safe (first closes, subsequent see nil channels).
func (d *QueueDriver) Stop() {
	d.lifecycleMu.Lock()
	stop, done := d.stop, d.done
	d.stop, d.done = nil, nil
	d.lifecycleMu.Unlock()

	if stop == nil {
		return
	}
	close(stop)
	<-done
	// Any tick goroutines spawned before the close must drain too.
	d.tickWG.Wait()
}

func (d *QueueDriver) run(stop, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			d.launchTick()
		}
	}
}

// launchTick dispatches tick in a bounded goroutine. If the previous tick is
// still running when the ticker fires, the new one is skipped (logged at debug)
// rather than queued — the 2 s cadence matters less than not building backlog
// under a slow Deck. Stop waits for in-flight ticks via d.tickWG.
func (d *QueueDriver) launchTick() {
	select {
	case d.tickSem <- struct{}{}:
	default:
		d.logger.Debug("driver tick skipped; previous still running")
		return
	}
	d.tickWG.Go(func() {
		defer func() { <-d.tickSem }()
		d.tick()
	})
}

// tick runs one cycle: if the Deck is idle AND has an empty next-up slot,
// pops the next queued entry and stages it via POST /queue. The slot check
// prevents the driver from overwriting a staged-but-not-yet-pulled song
// when two guests queue back-to-back while the Deck waits on ScreenMain.
// Failures at the stage layer are logged and, where transient, re-queued.
func (d *QueueDriver) tick() {
	if d.isDeckBusy() {
		return
	}
	if !d.slotEmpty() {
		return
	}
	entry := d.queue.Next()
	if entry == nil {
		return
	}
	d.stage(entry)
}

// slotEmpty checks /debug/state for a populated queuedSong. Returns true if
// the slot is empty (safe to stage). Any network/parse error → false, so the
// driver backs off rather than risk overwriting an un-pulled song.
//
// Depends on /debug/state being available; per API.md that endpoint is
// "not part of the stable integration surface" but it IS documented and
// the fake always serves it. When the real USDX moves /debug/state to a
// different shape, this is the one place to update.
func (d *QueueDriver) slotEmpty() bool {
	ctx, cancel := context.WithTimeout(context.Background(), driverRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.deckURL+"/debug/state", nil)
	if err != nil {
		return false
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var state struct {
		QueuedSong any `json:"queuedSong"`
	}
	if decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxProxyResponse)).Decode(&state); decodeErr != nil {
		return false
	}
	return state.QueuedSong == nil
}

// isDeckBusy parses GET /now-playing and returns true iff the Deck is
// actively playing a song. A JSON `null` body means idle. Any network or
// parse error is treated as "busy" so the driver backs off rather than
// staging into a possibly-broken Deck.
func (d *QueueDriver) isDeckBusy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), driverRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.deckURL+"/now-playing", nil)
	if err != nil {
		return true
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return true
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProxyResponse))
	if err != nil {
		return true
	}
	var parsed any
	if jsonErr := json.Unmarshal(body, &parsed); jsonErr != nil {
		return true
	}
	return parsed != nil
}

type queueStagePayload struct {
	SongID    string `json:"songId"`
	Requester string `json:"requester"`
	Players   int    `json:"players"`
}

func (d *QueueDriver) stage(entry *QueueEntry) {
	// Always stages as 1-player; the UI doesn't yet expose 2P selection on
	// per-queue-entry basis. When it does, QueueEntry gains a Players field
	// and this line reads from it.
	payload := queueStagePayload{
		SongID:    entry.Song.ID,
		Requester: entry.Guest,
		Players:   1,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		d.logger.Error("marshal queue stage payload", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), driverRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.deckURL+"/queue", bytes.NewReader(body))
	if err != nil {
		d.logger.Error("build queue request", "error", err)
		d.queue.ReAdd(entry.Song, entry.Guest)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		d.logger.Warn("deck unreachable for queue", "error", err)
		d.queue.ReAdd(entry.Song, entry.Guest)
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		d.logger.Info("staged song to deck",
			"song_id", payload.SongID,
			"requester", payload.Requester,
		)
	case http.StatusNotFound:
		// Library drift: the Deck doesn't know this song. Drop it — retrying
		// won't help until the library is refreshed on the Deck side.
		d.logger.Warn("deck rejected unknown songId; dropping",
			"song_id", payload.SongID,
			"requester", payload.Requester,
		)
	case http.StatusConflict:
		// "song in progress" — the Deck is briefly in ScreenSing state.
		// Next tick should succeed once the song completes.
		d.logger.Debug("deck 409 on queue stage; re-queueing",
			"song_id", payload.SongID,
		)
		d.queue.ReAdd(entry.Song, entry.Guest)
	default:
		d.logger.Warn("deck returned unexpected status on queue stage",
			"status", resp.StatusCode,
			"song_id", payload.SongID,
		)
		d.queue.ReAdd(entry.Song, entry.Guest)
	}
}
