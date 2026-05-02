package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/archive"
	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
	"github.com/joshsymonds/sound-stage/usdb"
)

type mockDownloader struct {
	youTubeIDs []string
	notReady   bool // default false (= ready)
}

func (m *mockDownloader) GetSongDetails(_ context.Context, _ int) (*usdb.SongDetails, error) {
	return &usdb.SongDetails{Artist: "Test", Title: "Song", YouTubeIDs: m.youTubeIDs}, nil
}

func (m *mockDownloader) GetSongTxt(_ context.Context, _ int) (string, error) {
	return "#TITLE:Song\n#ARTIST:Test\n#MP3:audio.webm\n: 0 5 10 Hello\nE\n", nil
}

func (m *mockDownloader) DownloadCover(_ int, _ string) error {
	return nil
}

func (m *mockDownloader) Ready() bool { return !m.notReady }

// mockYtDlp is a no-op yt-dlp stand-in — its methods return nil without
// spawning the yt-dlp binary so tests can exercise the full download happy
// path in-process.
type mockYtDlp struct{}

func (m *mockYtDlp) DownloadAudio(_, _, _ string) error { return nil }
func (m *mockYtDlp) DownloadVideo(_, _, _ string) error { return nil }

// blockingYtDlp gates DownloadAudio on a release channel so a test can hold
// a download mid-flight while it issues a second request from another guest.
type blockingYtDlp struct {
	release chan struct{}
}

func (b *blockingYtDlp) DownloadAudio(_, _, _ string) error {
	<-b.release
	return nil
}
func (b *blockingYtDlp) DownloadVideo(_, _, _ string) error { return nil }

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

		body := `{"songId": 99999, "guest": "Alice"}`
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

		// Wait for the goroutine to finish so t.TempDir cleanup doesn't race
		// with the in-flight PrepareSong write.
		waitForArchived(t, outputDir, 99999)
	})

	t.Run("rejects invalid songId", func(t *testing.T) {
		t.Parallel()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
		})

		body := `{"songId": 0, "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("rejects missing guest", func(t *testing.T) {
		t.Parallel()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
		})

		body := `{"songId": 99999}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("503 with Retry-After when not ready", func(t *testing.T) {
		t.Parallel()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{notReady: true},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
		})
		body := `{"songId": 99999, "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got == "" {
			t.Error("expected Retry-After header on 503 response")
		}
	})

	t.Run("rejects empty guest", func(t *testing.T) {
		t.Parallel()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
		})

		body := `{"songId": 99999, "guest": ""}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
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

		body := `{"songId": 99999, "guest": "Alice"}`
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

		body := `{"songId": 99999, "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", rec.Code)
		}
		// No deck to verify against — just confirm the 202 response. The
		// goroutine reaching notifyDeck with empty URL must not panic.
		waitForArchived(t, outputDir, 99999)
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

		body := `{"songId": 99999, "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", rec.Code)
		}
		// Goroutine's notifyDeck call will fail; test asserts no panic and
		// that the download HTTP response still succeeds.
		waitForArchived(t, outputDir, 99999)
	})
}

// TestDownloadAutoQueue exercises the auto-queue behavior: when a download
// completes (or already exists), the requesting guest's name is added to the
// queue with a Song whose ID equals stableid.Compute over the parsed .txt.
func TestDownloadAutoQueue(t *testing.T) {
	t.Parallel()

	expectedID := stableid.Compute("Test", "Song", false)

	t.Run("queues requester after fresh download succeeds", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		deck := httptest.NewServer(fake)
		defer deck.Close()

		queue := server.NewQueue()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{youTubeIDs: []string{"dQw4w9WgXcQ"}},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
			DeckURL:   deck.URL,
			Queue:     queue,
		})

		body := `{"songId": 99999, "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
		}

		entries := waitForQueueLen(t, queue, 1)
		if entries[0].Guest != "Alice" {
			t.Errorf("guest = %q, want Alice", entries[0].Guest)
		}
		if entries[0].Song.ID != expectedID {
			t.Errorf("song id = %q, want %q (stableid.Compute(Test, Song, false))", entries[0].Song.ID, expectedID)
		}
		if entries[0].Song.Title != "Song" {
			t.Errorf("title = %q, want Song", entries[0].Song.Title)
		}
		if entries[0].Song.Artist != "Test" {
			t.Errorf("artist = %q, want Test", entries[0].Song.Artist)
		}
	})

	t.Run("queues both guests when same song requested in flight", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		deck := httptest.NewServer(fake)
		defer deck.Close()

		blocker := &blockingYtDlp{release: make(chan struct{})}
		queue := server.NewQueue()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{youTubeIDs: []string{"dQw4w9WgXcQ"}},
			YtDlp:     blocker,
			OutputDir: t.TempDir(),
			DeckURL:   deck.URL,
			Queue:     queue,
		})

		// Alice requests first; download goroutine starts and blocks on YtDlp.
		body1 := `{"songId": 99999, "guest": "Alice"}`
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body1)))
		if rec1.Code != http.StatusAccepted {
			t.Fatalf("alice: expected 202, got %d", rec1.Code)
		}

		// Give the goroutine a moment to enter blockingYtDlp.DownloadAudio.
		// Then Bob requests the same song — should join the waiters list.
		time.Sleep(50 * time.Millisecond)

		body2 := `{"songId": 99999, "guest": "Bob"}`
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body2)))
		if rec2.Code != http.StatusOK {
			t.Fatalf("bob: expected 200 (in_progress), got %d: %s", rec2.Code, rec2.Body.String())
		}
		var resp2 map[string]string
		_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
		if resp2["status"] != "in_progress" {
			t.Errorf("bob: expected status 'in_progress', got %q", resp2["status"])
		}

		// Release the download; both guests should be queued.
		close(blocker.release)

		entries := waitForQueueLen(t, queue, 2)

		guests := []string{entries[0].Guest, entries[1].Guest}
		if !containsBoth(guests, "Alice", "Bob") {
			t.Errorf("queued guests = %v, want both Alice and Bob", guests)
		}
		for _, e := range entries {
			if e.Song.ID != expectedID {
				t.Errorf("song id = %q, want %q", e.Song.ID, expectedID)
			}
		}
	})

	t.Run("does not queue when /refresh fails", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		// Force /refresh to return 500 once.
		fake.QueueInjection("/refresh", http.StatusInternalServerError, 1)
		deck := httptest.NewServer(fake)
		defer deck.Close()

		queue := server.NewQueue()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{youTubeIDs: []string{"dQw4w9WgXcQ"}},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
			DeckURL:   deck.URL,
			Queue:     queue,
		})

		body := `{"songId": 99999, "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", rec.Code)
		}

		// Wait long enough for the goroutine to complete (and observe the
		// /refresh failure). Queue should remain empty.
		time.Sleep(300 * time.Millisecond)
		if got := len(queue.List()); got != 0 {
			t.Errorf("queue length = %d, want 0 (refresh failed)", got)
		}
	})

	t.Run("does not queue when no YouTube URL", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		deck := httptest.NewServer(fake)
		defer deck.Close()

		queue := server.NewQueue()
		handler := server.DownloadHandler(server.DownloadConfig{
			// No YouTube IDs → runDownload short-circuits before notifyDeck.
			Client:    &mockDownloader{youTubeIDs: nil},
			YtDlp:     &mockYtDlp{},
			OutputDir: t.TempDir(),
			DeckURL:   deck.URL,
			Queue:     queue,
		})

		body := `{"songId": 99999, "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", rec.Code)
		}

		time.Sleep(200 * time.Millisecond)
		if got := len(queue.List()); got != 0 {
			t.Errorf("queue length = %d, want 0 (no YouTube URL)", got)
		}
	})

	t.Run("queues guest when song already downloaded", func(t *testing.T) {
		t.Parallel()
		fake := fakeusdx.New()
		deck := httptest.NewServer(fake)
		defer deck.Close()

		outputDir := t.TempDir()
		// Pre-seed the archive AND drop the .txt where parseSongFile expects
		// it: <outputDir>/<dirName>/song.txt where dirName matches what
		// usdb.SanitizePath("Test - Song") produces.
		if err := archive.MarkDownloaded(outputDir, 99999); err != nil {
			t.Fatal(err)
		}
		songDir := filepath.Join(outputDir, usdb.SanitizePath("Test - Song"))
		if err := os.MkdirAll(songDir, 0o755); err != nil {
			t.Fatal(err)
		}
		txtPath := filepath.Join(songDir, "song.txt")
		if err := os.WriteFile(txtPath,
			[]byte("#TITLE:Song\n#ARTIST:Test\n#MP3:audio.webm\n: 0 5 10 Hello\nE\n"),
			0o600); err != nil {
			t.Fatal(err)
		}

		queue := server.NewQueue()
		handler := server.DownloadHandler(server.DownloadConfig{
			Client:    &mockDownloader{},
			YtDlp:     &mockYtDlp{},
			OutputDir: outputDir,
			DeckURL:   deck.URL,
			Queue:     queue,
		})

		body := `{"songId": 99999, "guest": "Alice"}`
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 (already_downloaded), got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["status"] != "already_downloaded" {
			t.Errorf("expected status 'already_downloaded', got %q", resp["status"])
		}

		// Already-downloaded path queues inline (no goroutine), so the entry
		// is present immediately on return. Allow a tiny window anyway.
		entries := waitForQueueLen(t, queue, 1)
		if entries[0].Guest != "Alice" {
			t.Errorf("guest = %q, want Alice", entries[0].Guest)
		}
		if entries[0].Song.ID != expectedID {
			t.Errorf("song id = %q, want %q", entries[0].Song.ID, expectedID)
		}
	})
}

// waitForQueueLen polls queue.List() until it has the expected length, with
// a short deadline so test failures fail fast rather than hanging.
func waitForQueueLen(t *testing.T, q *server.Queue, want int) []server.QueueEntry {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries := q.List()
		if len(entries) == want {
			return entries
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("queue.List() length never reached %d (got %d)", want, len(q.List()))
	return nil
}

// waitForArchived blocks until the download goroutine has called
// archive.MarkDownloaded for songID — the deferred parse/queue step still
// touches outputDir afterwards, but archive presence + a short grace window
// is enough to keep t.TempDir's cleanup from racing with the goroutine.
func waitForArchived(t *testing.T, outputDir string, songID int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if archive.IsDownloaded(outputDir, songID) {
			// Brief grace for the deferred parse/queue tail to finish.
			time.Sleep(50 * time.Millisecond)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("song %d never marked as downloaded", songID)
}

func containsBoth(s []string, a, b string) bool {
	var hasA, hasB bool
	for _, v := range s {
		if v == a {
			hasA = true
		}
		if v == b {
			hasB = true
		}
	}
	return hasA && hasB
}
