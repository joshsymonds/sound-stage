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
}
