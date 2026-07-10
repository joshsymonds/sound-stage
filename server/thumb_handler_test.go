package server_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

// thumbMux mounts the thumbnail handler so r.PathValue resolves correctly.
func thumbMux(cache *server.LibraryCache, libraryDir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/library/{id}/thumb", server.LibraryThumbHandler(cache, libraryDir))
	return mux
}

// testArtist and testTitle back every writeTestSong fixture in this file;
// expectedID mirrors them via stableid.Compute at each call site.
const (
	testArtist = "Test"
	testTitle  = "Song"
)

// writeTestSong creates a minimal valid library entry (song.txt + audio file)
// under libraryDir so LibraryCache.Get's scan recognizes it, matching the
// fixture shape used by TestLibraryCoverHandler.
func writeTestSong(t *testing.T, libraryDir string) string {
	t.Helper()
	songDir := filepath.Join(libraryDir, testArtist+" - "+testTitle)
	if err := os.MkdirAll(songDir, 0o750); err != nil {
		t.Fatal(err)
	}
	txt := "#TITLE:" + testTitle + "\n#ARTIST:" + testArtist + "\n#MP3:audio.webm\n: 0 5 10 Hello\nE\n"
	if err := os.WriteFile(filepath.Join(songDir, "song.txt"), []byte(txt), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(songDir, "audio.webm"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return songDir
}

// writeTestJPEG writes a decodable width x height JPEG cover to path.
func writeTestJPEG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			shade := color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255}
			img.Set(x, y, shade)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test cover: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test cover: %v", err)
	}
}

func TestLibraryThumbHandler(t *testing.T) {
	t.Parallel()

	t.Run("resizes a wide cover to 320px preserving aspect", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		songDir := writeTestSong(t, libraryDir)
		writeTestJPEG(t, filepath.Join(songDir, "cover.jpg"), 800, 400)

		expectedID := stableid.Compute(testArtist, testTitle, false)
		cache := server.NewLibraryCache()
		rec := httptest.NewRecorder()
		thumbMux(cache, libraryDir).ServeHTTP(rec,
			httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/thumb", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("Content-Type = %q, want image/jpeg", ct)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
			t.Errorf("Cache-Control = %q, want public, max-age=86400", cc)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("decode thumbnail: %v", err)
		}
		if cfg.Width != 320 {
			t.Errorf("width = %d, want 320", cfg.Width)
		}
		if cfg.Height != 160 {
			t.Errorf("height = %d, want 160 (aspect preserved from 800x400)", cfg.Height)
		}
	})

	t.Run("cache hit serves stale-source thumbnail without re-reading cover.jpg", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		songDir := writeTestSong(t, libraryDir)
		writeTestJPEG(t, filepath.Join(songDir, "cover.jpg"), 800, 800)

		expectedID := stableid.Compute(testArtist, testTitle, false)
		cache := server.NewLibraryCache()
		handler := thumbMux(cache, libraryDir)

		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/thumb", nil))
		if rec1.Code != http.StatusOK {
			t.Fatalf("first request: expected 200, got %d", rec1.Code)
		}

		if err := os.Remove(filepath.Join(songDir, "cover.jpg")); err != nil {
			t.Fatal(err)
		}

		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/thumb", nil))
		if rec2.Code != http.StatusOK {
			t.Fatalf("second request: expected 200, got %d", rec2.Code)
		}
		if !bytes.Equal(rec1.Body.Bytes(), rec2.Body.Bytes()) {
			t.Error("second response bytes differ from first; expected identical cached thumbnail")
		}
	})

	t.Run("does not upscale a source narrower than 320px", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		songDir := writeTestSong(t, libraryDir)
		writeTestJPEG(t, filepath.Join(songDir, "cover.jpg"), 200, 200)

		expectedID := stableid.Compute(testArtist, testTitle, false)
		cache := server.NewLibraryCache()
		rec := httptest.NewRecorder()
		thumbMux(cache, libraryDir).ServeHTTP(rec,
			httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/thumb", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("decode thumbnail: %v", err)
		}
		if cfg.Width != 200 {
			t.Errorf("width = %d, want 200 (no upscale)", cfg.Width)
		}
	})

	t.Run("404 for unknown song id", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		cache := server.NewLibraryCache()
		rec := httptest.NewRecorder()
		thumbMux(cache, libraryDir).ServeHTTP(rec,
			httptest.NewRequest(http.MethodGet, "/api/library/deadbeefdeadbeef/thumb", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("404 for song without cover.jpg on disk", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		writeTestSong(t, libraryDir)
		// No cover.jpg written.

		expectedID := stableid.Compute(testArtist, testTitle, false)
		cache := server.NewLibraryCache()
		rec := httptest.NewRecorder()
		thumbMux(cache, libraryDir).ServeHTTP(rec,
			httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/thumb", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("corrupt cover data 404s and leaves no thumbnail cached", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		songDir := writeTestSong(t, libraryDir)
		if err := os.WriteFile(filepath.Join(songDir, "cover.jpg"), []byte("not a jpeg"), 0o600); err != nil {
			t.Fatal(err)
		}

		expectedID := stableid.Compute(testArtist, testTitle, false)
		cache := server.NewLibraryCache()
		rec := httptest.NewRecorder()
		thumbMux(cache, libraryDir).ServeHTTP(rec,
			httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/thumb", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		if _, err := os.Stat(filepath.Join(libraryDir, ".thumbs", expectedID+".jpg")); err == nil {
			t.Error("expected no cached thumbnail file for a corrupt cover")
		}
	})

	t.Run("concurrent cold requests for the same id coalesce to one resize", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		songDir := writeTestSong(t, libraryDir)

		// A FIFO in place of cover.jpg blocks the reading goroutine until a
		// writer connects. Because LibraryThumbHandler funnels population
		// through a single singleflight.Group, at most one goroutine ever
		// calls os.Open on it — a second, uncoalesced attempt would block
		// forever (no second writer) and trip the timeout below.
		coverPath := filepath.Join(songDir, "cover.jpg")
		if err := syscall.Mkfifo(coverPath, 0o600); err != nil {
			t.Fatalf("mkfifo: %v", err)
		}

		expectedID := stableid.Compute(testArtist, testTitle, false)
		cache := server.NewLibraryCache()
		handler := thumbMux(cache, libraryDir)

		const concurrency = 4
		var wg sync.WaitGroup
		codes := make([]int, concurrency)
		wg.Add(concurrency)
		for i := range concurrency {
			go func() {
				defer wg.Done()
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/thumb", nil)
				handler.ServeHTTP(rec, req)
				codes[i] = rec.Code
			}()
		}

		// Give the goroutines time to reach inflight.Do: the leader blocks
		// opening the FIFO, the rest wait on the leader's call.
		time.Sleep(50 * time.Millisecond)

		go func() {
			w, err := os.OpenFile(coverPath, os.O_WRONLY, 0)
			if err != nil {
				return
			}
			defer w.Close()
			img := image.NewRGBA(image.Rect(0, 0, 800, 400))
			_ = jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
		}()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("requests did not complete: a second populate attempt likely blocked opening the FIFO, " +
				"meaning singleflight failed to coalesce")
		}

		for i, code := range codes {
			if code != http.StatusOK {
				t.Errorf("request %d: expected 200, got %d", i, code)
			}
		}

		thumb, err := os.ReadFile(filepath.Join(libraryDir, ".thumbs", expectedID+".jpg"))
		if err != nil {
			t.Fatalf("expected cached thumbnail: %v", err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(thumb))
		if err != nil {
			t.Fatalf("decode cached thumbnail: %v", err)
		}
		if cfg.Width != 320 {
			t.Errorf("thumbnail width = %d, want 320", cfg.Width)
		}
	})
}
