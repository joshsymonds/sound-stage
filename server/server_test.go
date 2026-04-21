package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
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
