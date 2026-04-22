package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/usdb"
)

type mockSearcher struct {
	results []usdb.Song
	err     error
}

func (m *mockSearcher) Search(_ usdb.SearchParams) ([]usdb.Song, error) {
	return m.results, m.err
}

func TestUSDBSearchHandler(t *testing.T) {
	t.Parallel()

	t.Run("returns search results", func(t *testing.T) {
		t.Parallel()
		searcher := &mockSearcher{
			results: []usdb.Song{
				{ID: 1, Artist: "Queen", Title: "Bohemian Rhapsody"},
				{ID: 2, Artist: "Queen", Title: "Don't Stop Me Now"},
			},
		}
		handler := server.USDBSearchHandler(searcher)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/usdb/search?artist=Queen", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var results []usdb.Song
		if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("requires at least one search parameter", func(t *testing.T) {
		t.Parallel()
		searcher := &mockSearcher{}
		handler := server.USDBSearchHandler(searcher)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/usdb/search", nil))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns literal [] when no results", func(t *testing.T) {
		t.Parallel()
		// Searcher returns nil — same shape as a no-match search. Body must
		// be the literal "[]", not "null", so the web client (which expects
		// USDBResult[]) doesn't crash on .length.
		searcher := &mockSearcher{results: nil}
		handler := server.USDBSearchHandler(searcher)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/usdb/search?title=nothing", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body := strings.TrimSpace(rec.Body.String())
		if body != "[]" {
			t.Fatalf("expected body %q, got %q", "[]", body)
		}
	})

	t.Run("returns 502 on USDB error", func(t *testing.T) {
		t.Parallel()
		searcher := &mockSearcher{err: fmt.Errorf("USDB down")}
		handler := server.USDBSearchHandler(searcher)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/usdb/search?title=test", nil))

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})
}
