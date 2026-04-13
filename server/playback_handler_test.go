package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
)

func TestPlaybackProxyHandler(t *testing.T) {
	t.Parallel()

	t.Run("proxies to deck and returns response", func(t *testing.T) {
		t.Parallel()
		deck := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"paused"}`))
		}))
		defer deck.Close()

		handler := server.PlaybackProxyHandler(deck.URL, "/pause")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/playback/pause", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != `{"status":"paused"}` {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("returns 503 when deck is unreachable", func(t *testing.T) {
		t.Parallel()
		handler := server.PlaybackProxyHandler("http://127.0.0.1:1", "/pause")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/playback/pause", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("returns 503 when deck not configured", func(t *testing.T) {
		t.Parallel()
		handler := server.PlaybackProxyHandler("", "/pause")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/playback/pause", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})
}

func TestNowPlayingProxyHandler(t *testing.T) {
	t.Parallel()

	t.Run("proxies now-playing from deck", func(t *testing.T) {
		t.Parallel()
		deck := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"title":"Test","artist":"Artist","elapsed":10,"duration":200}`))
		}))
		defer deck.Close()

		handler := server.NowPlayingProxyHandler(deck.URL)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/now-playing", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("returns null when deck offline", func(t *testing.T) {
		t.Parallel()
		handler := server.NowPlayingProxyHandler("http://127.0.0.1:1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/now-playing", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "null" {
			t.Fatalf("expected null, got %s", rec.Body.String())
		}
	})

	t.Run("returns null when deck not configured", func(t *testing.T) {
		t.Parallel()
		handler := server.NowPlayingProxyHandler("")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/now-playing", nil))

		if rec.Body.String() != "null" {
			t.Fatalf("expected null, got %s", rec.Body.String())
		}
	})
}
