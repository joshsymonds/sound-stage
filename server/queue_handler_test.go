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

// removeReq builds a DELETE request that exercises the route's path-value
// matcher — the handler reads position via r.PathValue("position"), which
// requires the request to come through the actual mux, not a raw
// httptest.NewRequest with a query-string-only URL. We mount the handler
// on a small mux to mirror server.Handler's wiring.
func removeReq(position, guest string) *http.Request {
	url := "/api/queue/" + position
	if guest != "" {
		url += "?guest=" + guest
	}
	return httptest.NewRequest(http.MethodDelete, url, nil)
}

func newRemoveMux(q *server.Queue) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("DELETE /api/queue/{position}", server.QueueRemoveHandler(q))
	return mux
}

func TestQueueRemoveHandler(t *testing.T) {
	t.Parallel()

	t.Run("DELETE removes the entry when guest matches", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(server.Song{ID: "abc1", Title: "First", Artist: "A"}, "Alice")
		q.Add(server.Song{ID: "abc2", Title: "Second", Artist: "B"}, "Bob")

		rec := httptest.NewRecorder()
		newRemoveMux(q).ServeHTTP(rec, removeReq("1", "Alice"))

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
		}
		if got := len(q.List()); got != 1 {
			t.Errorf("queue length = %d, want 1", got)
		}
	})

	t.Run("DELETE returns 403 when guest does not match", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(server.Song{ID: "abc1", Title: "First", Artist: "A"}, "Alice")
		q.Add(server.Song{ID: "abc2", Title: "Second", Artist: "B"}, "Bob")

		rec := httptest.NewRecorder()
		newRemoveMux(q).ServeHTTP(rec, removeReq("1", "Bob"))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
		if got := len(q.List()); got != 2 {
			t.Errorf("queue length = %d, want 2 (unchanged)", got)
		}
	})

	t.Run("DELETE returns 404 when position is out of range", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(server.Song{ID: "abc1", Title: "First", Artist: "A"}, "Alice")

		rec := httptest.NewRecorder()
		newRemoveMux(q).ServeHTTP(rec, removeReq("99", "Alice"))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
		if got := len(q.List()); got != 1 {
			t.Errorf("queue length = %d, want 1 (unchanged)", got)
		}
	})

	t.Run("DELETE returns 400 when position is zero", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		rec := httptest.NewRecorder()
		newRemoveMux(q).ServeHTTP(rec, removeReq("0", "Alice"))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("DELETE returns 400 when position is non-numeric", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		rec := httptest.NewRecorder()
		newRemoveMux(q).ServeHTTP(rec, removeReq("abc", "Alice"))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("DELETE returns 400 when guest query is missing", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(server.Song{ID: "abc1", Title: "First", Artist: "A"}, "Alice")

		rec := httptest.NewRecorder()
		newRemoveMux(q).ServeHTTP(rec, removeReq("1", ""))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}
