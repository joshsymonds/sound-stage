package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/joshsymonds/sound-stage/server"
)

func TestSPAFallback(t *testing.T) {
	t.Parallel()

	staticFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>SPA</html>")},
		"style.css":  &fstest.MapFile{Data: []byte("body{}")},
		"_app/immutable/abc.js": &fstest.MapFile{
			Data: []byte("console.log('app')"),
		},
	}
	songDir := t.TempDir()

	handler := server.Handler(server.Config{
		Port:       "0",
		LibraryDir: songDir,
		StaticFS:   staticFS,
	})

	t.Run("serves index.html at root", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if string(body) != "<html>SPA</html>" {
			t.Fatalf("unexpected body: %s", body)
		}
	})

	t.Run("serves arbitrary asset at top level", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/style.css", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("serves underscore-prefixed SvelteKit asset", func(t *testing.T) {
		t.Parallel()
		// SvelteKit emits hashed assets under _app/ — go:embed defaults to
		// excluding underscore-prefixed paths, so this confirms the all:
		// prefix on the embed directive includes them.
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_app/immutable/abc.js", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("falls back to index.html for SPA routes", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/queue", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 (SPA fallback), got %d", rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if string(body) != "<html>SPA</html>" {
			t.Fatalf("expected SPA content, got: %s", body)
		}
	})

	t.Run("API routes return proper responses", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/songs", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json, got %s", ct)
		}
	})

	t.Run("unknown API routes return 404", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 (no SPA fallback), got %d", rec.Code)
		}
	})
}

// TestUSDBNotReady_NonUSDBRoutesUnaffected exercises the full mux with a
// searcher whose Ready() reports false to prove that non-USDB endpoints
// keep working while USDB login is still in flight. This is the property
// that makes serve.go's "bind first, login asynchronously" arrangement
// useful — without it the user would see a frozen app at startup.
func TestUSDBNotReady_NonUSDBRoutesUnaffected(t *testing.T) {
	t.Parallel()

	libraryDir := t.TempDir()
	handler := server.Handler(server.Config{
		Port:       "0",
		LibraryDir: libraryDir,
		Searcher:   &mockSearcher{notReady: true},
	})

	t.Run("library is reachable", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/songs", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("queue is reachable", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/queue", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("USDB search returns 503", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/usdb/search?title=test", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got == "" {
			t.Error("expected Retry-After header on 503")
		}
		// Body is human-readable; just confirm it's not empty so a curl
		// user gets a hint about what's happening.
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "USDB") {
			t.Errorf("expected body to mention USDB, got %q", body)
		}
	})
}
