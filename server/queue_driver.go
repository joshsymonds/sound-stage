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
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
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
	}
}

const (
	deckMaxIdleConns        = 4
	deckMaxIdleConnsPerHost = 2
	deckIdleConnTimeout     = 90 * time.Second
)

// newDeckClient builds an http.Client scoped to Deck communication: its own
// connection pool, idle-connection reaper, and a wall-clock timeout that
// acts as a backstop if a request ignores its context deadline.
func newDeckClient() *http.Client {
	return &http.Client{
		Timeout: proxyTimeout + time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        deckMaxIdleConns,
			MaxIdleConnsPerHost: deckMaxIdleConnsPerHost,
			IdleConnTimeout:     deckIdleConnTimeout,
		},
	}
}

// Start begins ticking in a background goroutine. Safe to call once.
func (d *QueueDriver) Start() {
	d.stop = make(chan struct{})
	d.done = make(chan struct{})
	go d.run()
}

// Stop signals the ticker to exit and waits for the goroutine to finish.
// Idempotent.
func (d *QueueDriver) Stop() {
	d.once.Do(func() {
		if d.stop == nil {
			return
		}
		close(d.stop)
		<-d.done
	})
}

func (d *QueueDriver) run() {
	defer close(d.done)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
			d.tick()
		}
	}
}

// tick runs one cycle: if the Deck is idle and the queue is non-empty, pops
// the next entry and stages it via POST /queue. Failures at this layer are
// logged and, where transient, re-queued.
func (d *QueueDriver) tick() {
	if d.isDeckBusy() {
		return
	}
	entry := d.queue.Next()
	if entry == nil {
		return
	}
	d.stage(entry)
}

// isDeckBusy parses GET /now-playing and returns true iff the Deck is
// actively playing a song. A JSON `null` body means idle. Any network or
// parse error is treated as "busy" so the driver backs off rather than
// staging into a possibly-broken Deck.
func (d *QueueDriver) isDeckBusy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), proxyTimeout)
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

	ctx, cancel := context.WithTimeout(context.Background(), proxyTimeout)
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
