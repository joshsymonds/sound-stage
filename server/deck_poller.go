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

// PlayRequest is the JSON body sent to the Deck's POST /play endpoint.
type PlayRequest struct {
	SongID int    `json:"songId"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Singer string `json:"singer"`
}

type deckNowPlaying struct {
	Title    string `json:"title"`
	Duration int    `json:"duration"`
}

// DeckPoller polls the Deck's now-playing endpoint and auto-advances the queue.
type DeckPoller struct {
	deckURL  string
	queue    *Queue
	interval time.Duration
	logger   *slog.Logger
	stop     chan struct{}
	once     sync.Once
}

// NewDeckPoller creates a new poller that checks the Deck every interval.
func NewDeckPoller(deckURL string, queue *Queue, interval time.Duration) *DeckPoller {
	return &DeckPoller{
		deckURL:  deckURL,
		queue:    queue,
		interval: interval,
		logger:   slog.Default(),
		stop:     make(chan struct{}),
	}
}

// Start begins polling in a background goroutine.
func (dp *DeckPoller) Start() {
	go dp.run()
}

// Stop signals the poller to stop and waits for it to finish the current tick.
func (dp *DeckPoller) Stop() {
	dp.once.Do(func() {
		close(dp.stop)
	})
}

func (dp *DeckPoller) run() {
	ticker := time.NewTicker(dp.interval)
	defer ticker.Stop()

	for {
		select {
		case <-dp.stop:
			return
		case <-ticker.C:
			dp.tick()
		}
	}
}

func (dp *DeckPoller) tick() {
	if dp.isDeckPlaying() {
		return
	}

	entry := dp.queue.Next()
	if entry == nil {
		return
	}

	dp.sendPlay(entry)
}

func (dp *DeckPoller) isDeckPlaying() bool {
	ctx, cancel := context.WithTimeout(context.Background(), proxyTimeout)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, dp.deckURL+"/now-playing", nil)
	if reqErr != nil {
		return false
	}

	resp, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		return false
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return false
	}

	// "null" means nothing is playing.
	if string(bytes.TrimSpace(body)) == "null" {
		return false
	}

	var nowPlaying deckNowPlaying
	if unmarshalErr := json.Unmarshal(body, &nowPlaying); unmarshalErr != nil {
		return false
	}

	return nowPlaying.Title != ""
}

func (dp *DeckPoller) sendPlay(entry *QueueEntry) {
	playReq := PlayRequest{
		SongID: entry.Song.ID,
		Title:  entry.Song.Title,
		Artist: entry.Song.Artist,
		Singer: entry.Guest,
	}

	body, marshalErr := json.Marshal(playReq)
	if marshalErr != nil {
		dp.logger.Error("marshaling play request", "error", marshalErr)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), proxyTimeout)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, dp.deckURL+"/play", bytes.NewReader(body))
	if reqErr != nil {
		dp.logger.Error("creating play request", "error", reqErr)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		dp.logger.Warn("deck unreachable for play", "error", doErr)
		return
	}
	defer resp.Body.Close()

	dp.logger.Info("sent play command to deck",
		"song_id", playReq.SongID,
		"title", playReq.Title,
		"singer", playReq.Singer,
	)
}
