package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server"
)

func TestDeckPoller(t *testing.T) {
	t.Parallel()

	t.Run("sends next song when deck reports idle", func(t *testing.T) {
		t.Parallel()

		var playRequests []server.PlayRequest
		var mu sync.Mutex

		deck := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/now-playing":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("null"))
			case "/play":
				var req server.PlayRequest
				json.NewDecoder(r.Body).Decode(&req)
				mu.Lock()
				playRequests = append(playRequests, req)
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer deck.Close()

		queue := server.NewQueue()
		queue.Add(server.Song{ID: 1, Title: "Test Song", Artist: "Artist"}, "Alice")

		poller := server.NewDeckPoller(deck.URL, queue, 50*time.Millisecond)
		poller.Start()
		time.Sleep(200 * time.Millisecond)
		poller.Stop()

		mu.Lock()
		defer mu.Unlock()
		if len(playRequests) == 0 {
			t.Fatal("expected at least one play request")
		}
		if playRequests[0].Title != "Test Song" {
			t.Errorf("expected Test Song, got %s", playRequests[0].Title)
		}
		if playRequests[0].Singer != "Alice" {
			t.Errorf("expected Alice, got %s", playRequests[0].Singer)
		}
	})

	t.Run("does nothing when queue is empty", func(t *testing.T) {
		t.Parallel()

		var playCalled atomic.Int32

		deck := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/now-playing":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("null"))
			case "/play":
				playCalled.Add(1)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer deck.Close()

		queue := server.NewQueue()

		poller := server.NewDeckPoller(deck.URL, queue, 50*time.Millisecond)
		poller.Start()
		time.Sleep(200 * time.Millisecond)
		poller.Stop()

		if playCalled.Load() != 0 {
			t.Errorf("expected 0 play calls, got %d", playCalled.Load())
		}
	})

	t.Run("does not send when deck is playing", func(t *testing.T) {
		t.Parallel()

		var playCalled atomic.Int32

		nowPlaying := map[string]any{
			"title":    "Current Song",
			"artist":   "Current Artist",
			"elapsed":  50,
			"duration": 200,
		}

		deck := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/now-playing":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(nowPlaying)
			case "/play":
				playCalled.Add(1)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer deck.Close()

		queue := server.NewQueue()
		queue.Add(server.Song{ID: 1, Title: "Waiting", Artist: "Artist"}, "Bob")

		poller := server.NewDeckPoller(deck.URL, queue, 50*time.Millisecond)
		poller.Start()
		time.Sleep(200 * time.Millisecond)
		poller.Stop()

		if playCalled.Load() != 0 {
			t.Errorf("expected 0 play calls while playing, got %d", playCalled.Load())
		}
	})

	t.Run("handles deck offline gracefully", func(t *testing.T) {
		t.Parallel()

		queue := server.NewQueue()
		queue.Add(server.Song{ID: 1, Title: "Waiting", Artist: "Artist"}, "Bob")

		poller := server.NewDeckPoller("http://127.0.0.1:1", queue, 50*time.Millisecond)
		poller.Start()
		time.Sleep(150 * time.Millisecond)
		poller.Stop()
		// No panic = pass.
	})

	t.Run("stops cleanly", func(t *testing.T) {
		t.Parallel()

		deck := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("null"))
		}))
		defer deck.Close()

		poller := server.NewDeckPoller(deck.URL, server.NewQueue(), 50*time.Millisecond)
		poller.Start()
		poller.Stop()
		// Double stop should not panic.
		poller.Stop()
	})
}
