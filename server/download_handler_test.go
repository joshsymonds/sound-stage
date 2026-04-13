package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/usdb"
	"github.com/joshsymonds/sound-stage/ytdlp"
)

type mockDownloader struct{}

func (m *mockDownloader) GetSongDetails(_ int) (*usdb.SongDetails, error) {
	return &usdb.SongDetails{Artist: "Test", Title: "Song"}, nil
}

func (m *mockDownloader) GetSongTxt(_ int) (string, error) {
	return "#TITLE:Song\n#ARTIST:Test\n#MP3:audio.webm\n: 0 5 10 Hello\nE\n", nil
}

func (m *mockDownloader) DownloadCover(_ int, _ string) error {
	return nil
}

func TestDownloadHandler(t *testing.T) {
	t.Parallel()

	t.Run("returns 202 for valid request", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     ytdlp.Downloader{},
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
			OutputDir: t.TempDir(),
		})

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader("not json")))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}
