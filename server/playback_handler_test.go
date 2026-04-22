package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

func TestPlaybackProxyHandler(t *testing.T) {
	t.Parallel()

	t.Run("proxies pause to fake deck", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		fake.LoadSongs([]fakeusdx.Song{{Title: "T", Artist: "A", Duet: false}})
		id := stableid.Compute("A", "T", false)
		if err := fake.SetCurrentPlaying(id, 10, 200); err != nil {
			t.Fatal(err)
		}
		deck := httptest.NewServer(fake)
		defer deck.Close()

		handler := server.PlaybackProxyHandler(http.DefaultClient, deck.URL, "/pause")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/playback/pause", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var got map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("body not JSON: %v, body=%s", err, rec.Body.String())
		}
		if got["status"] != "paused" {
			t.Errorf(`status = %q, want "paused"`, got["status"])
		}
	})

	t.Run("forwards 409 from fake when not playing", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		deck := httptest.NewServer(fake)
		defer deck.Close()

		handler := server.PlaybackProxyHandler(http.DefaultClient, deck.URL, "/pause")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/playback/pause", nil))

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 (not playing), got %d", rec.Code)
		}
	})

	t.Run("returns 503 when deck is unreachable", func(t *testing.T) {
		t.Parallel()
		handler := server.PlaybackProxyHandler(http.DefaultClient, "http://127.0.0.1:1", "/pause")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/playback/pause", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("returns 503 when deck not configured", func(t *testing.T) {
		t.Parallel()
		handler := server.PlaybackProxyHandler(http.DefaultClient, "", "/pause")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/playback/pause", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})
}

func TestNowPlayingProxyHandler(t *testing.T) {
	t.Parallel()

	t.Run("proxies new-shape body from fake deck", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		fake.LoadSongs([]fakeusdx.Song{{Title: "Take On Me", Artist: "a-ha", Duet: false}})
		id := stableid.Compute("a-ha", "Take On Me", false)
		if err := fake.SetCurrentPlaying(id, 42.5, 243.5); err != nil {
			t.Fatal(err)
		}
		deck := httptest.NewServer(fake)
		defer deck.Close()

		handler := server.NowPlayingProxyHandler(http.DefaultClient, deck.URL)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/now-playing", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var got struct {
			ID       string  `json:"id"`
			Title    string  `json:"title"`
			Artist   string  `json:"artist"`
			Elapsed  float64 `json:"elapsed"`
			Duration float64 `json:"duration"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal: %v, body=%s", err, rec.Body.String())
		}
		if got.ID != id || got.Title != "Take On Me" || got.Artist != "a-ha" {
			t.Errorf("response = %+v, want id/title/artist to match fake state", got)
		}
		if got.Elapsed != 42.5 || got.Duration != 243.5 {
			t.Errorf("timing = %v/%v, want 42.5/243.5", got.Elapsed, got.Duration)
		}
	})

	t.Run("returns null when fake deck is idle", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		deck := httptest.NewServer(fake)
		defer deck.Close()

		handler := server.NowPlayingProxyHandler(http.DefaultClient, deck.URL)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/now-playing", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "null" {
			t.Fatalf("expected null, got %s", rec.Body.String())
		}
	})

	t.Run("returns null when deck offline", func(t *testing.T) {
		t.Parallel()
		handler := server.NowPlayingProxyHandler(http.DefaultClient, "http://127.0.0.1:1")
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
		handler := server.NowPlayingProxyHandler(http.DefaultClient, "")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/now-playing", nil))

		if rec.Body.String() != "null" {
			t.Fatalf("expected null, got %s", rec.Body.String())
		}
	})
}
