package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
)

func TestQueueHandlers(t *testing.T) {
	t.Parallel()

	t.Run("GET /api/queue returns empty array", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		handler := server.QueueListHandler(q)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/queue", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var entries []server.QueueEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("POST /api/queue adds a song", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		handler := server.QueueAddHandler(q)

		body := `{"songId": "deadbeef00000001", "title": "Bohemian Rhapsody", "artist": "Queen", "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/queue", strings.NewReader(body)))

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}

		entries := q.List()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Song.Title != "Bohemian Rhapsody" {
			t.Errorf("expected Bohemian Rhapsody, got %s", entries[0].Song.Title)
		}
		if entries[0].Guest != "Alice" {
			t.Errorf("expected Alice, got %s", entries[0].Guest)
		}
	})

	t.Run("POST /api/queue rejects missing guest", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		handler := server.QueueAddHandler(q)

		body := `{"songId": "deadbeef00000001", "title": "Test", "artist": "Test"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/queue", strings.NewReader(body)))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("POST /api/queue/skip returns next song", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(server.Song{ID: "deadbeef00000001", Title: "First", Artist: "A"}, "Alice")
		q.Add(server.Song{ID: "deadbeef00000002", Title: "Second", Artist: "B"}, "Bob")

		handler := server.QueueSkipHandler(q)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/queue/skip", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var entry server.QueueEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Song.Title != "First" {
			t.Errorf("expected First, got %s", entry.Song.Title)
		}

		// Queue should now have 1 entry.
		remaining := q.List()
		if len(remaining) != 1 {
			t.Fatalf("expected 1 remaining, got %d", len(remaining))
		}
	})

	t.Run("POST /api/queue/skip on empty returns 204", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		handler := server.QueueSkipHandler(q)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/queue/skip", nil))

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})
}
