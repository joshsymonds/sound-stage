package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/usdb"
)

type mockDownloader struct {
	youTubeIDs []string
}

func (m *mockDownloader) GetSongDetails(_ int) (*usdb.SongDetails, error) {
	return &usdb.SongDetails{Artist: "Test", Title: "Song", YouTubeIDs: m.youTubeIDs}, nil
}

func (m *mockDownloader) GetSongTxt(_ int) (string, error) {
	return "#TITLE:Song\n#ARTIST:Test\n#MP3:audio.webm\n: 0 5 10 Hello\nE\n", nil
}

func (m *mockDownloader) DownloadCover(_ int, _ string) error {
	return nil
}

// mockYtDlp is a no-op yt-dlp stand-in — its methods return nil without
// spawning the yt-dlp binary so tests can exercise the full download happy
// path in-process.
type mockYtDlp struct{}

func (m *mockYtDlp) DownloadAudio(_, _, _ string) error { return nil }
func (m *mockYtDlp) DownloadVideo(_, _, _ string) error { return nil }

func TestDownloadHandler(t *testing.T) {
	t.Parallel()

	t.Run("returns 202 for valid request", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: outputDir,
		})

		body := `{"songId": 99999}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["status"] != "downloading" {
			t.Errorf("expected status 'downloading', got %q", resp["status"])
		}
	})

	t.Run("rejects invalid songId", func(t *testing.T) {
		t.Parallel()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
		})

		body := `{"songId": 0}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		t.Parallel()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
		})

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader("not json")))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("notifies fake deck via /refresh after successful download", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		deck := httptest.NewServer(fake)
		defer deck.Close()

		outputDir := t.TempDir()
		// Mock downloader with a valid-format YouTube ID so runDownload takes
		// the full-success branch (audio/video → notifyDeck). mockYtDlp
		// returns nil for the downloads without spawning yt-dlp.
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{youTubeIDs: []string{"dQw4w9WgXcQ"}},
			YtDlp:     &mockYtDlp{},
			OutputDir: outputDir,
			DeckURL:   deck.URL,
		})

		body := `{"songId": 99999}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
		}

		// Poll the fake's /songs endpoint for the new song — the download
		// goroutine writes the .txt, then POSTs /refresh, then marks done.
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get(deck.URL + "/songs")
			if err != nil {
				t.Fatal(err)
			}
			var songs []struct {
				Title  string `json:"title"`
				Artist string `json:"artist"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&songs)
			resp.Body.Close()
			if len(songs) == 1 && songs[0].Title == "Song" && songs[0].Artist == "Test" {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("fake library never grew — /refresh was not called or failed")
	})

	t.Run("no deck URL skips /refresh but download still succeeds", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: outputDir,
			DeckURL:   "",
		})

		body := `{"songId": 99999}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", rec.Code)
		}
		// No deck to verify against — just confirm the 202 response. The
		// goroutine reaching notifyDeck with empty URL must not panic.
	})

	t.Run("deck unreachable does not fail the download", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: outputDir,
			DeckURL:   "http://127.0.0.1:1", // unreachable port
		})

		body := `{"songId": 99999}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", rec.Code)
		}
		// Goroutine's notifyDeck call will fail; test asserts no panic and
		// that the download HTTP response still succeeds.
	})
}
