package server_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

// fakeCoverFetcher counts FetchCover calls so tests can assert that a cache
// hit truly skips upstream.
type fakeCoverFetcher struct {
	body  string
	err   error
	calls atomic.Int32
}

func (f *fakeCoverFetcher) FetchCover(_ context.Context, _ int) (io.ReadCloser, string, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, "", f.err
	}
	return io.NopCloser(strings.NewReader(f.body)), "image/jpeg", nil
}

// usdbCoverMux mounts the cover handler so r.PathValue resolves correctly.
func usdbCoverMux(fetcher server.CoverFetcher, cacheDir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/usdb/cover/{id}", server.USDBCoverHandler(fetcher, cacheDir))
	return mux
}

func TestUSDBCoverHandler(t *testing.T) {
	t.Parallel()

	t.Run("400 on non-numeric id", func(t *testing.T) {
		t.Parallel()
		fetcher := &fakeCoverFetcher{}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/usdb/cover/abc", nil)
		usdbCoverMux(fetcher, t.TempDir()).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		if calls := fetcher.calls.Load(); calls != 0 {
			t.Errorf("fetcher called %d times; want 0", calls)
		}
	})

	t.Run("cache miss fetches upstream and writes to disk", func(t *testing.T) {
		t.Parallel()
		cacheDir := t.TempDir()
		fetcher := &fakeCoverFetcher{body: "JPEGBYTES"}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/usdb/cover/42", nil)
		usdbCoverMux(fetcher, cacheDir).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "JPEGBYTES" {
			t.Errorf("body = %q, want JPEGBYTES", rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("Content-Type = %q, want image/jpeg", ct)
		}
		// Cached file present.
		cached, err := os.ReadFile(filepath.Join(cacheDir, "42.jpg"))
		if err != nil {
			t.Fatalf("expected cached file: %v", err)
		}
		if string(cached) != "JPEGBYTES" {
			t.Errorf("cached body = %q, want JPEGBYTES", cached)
		}
	})

	t.Run("cache hit serves from disk without calling upstream", func(t *testing.T) {
		t.Parallel()
		cacheDir := t.TempDir()
		// Pre-seed.
		if err := os.WriteFile(filepath.Join(cacheDir, "42.jpg"), []byte("CACHED"), 0o600); err != nil {
			t.Fatal(err)
		}
		fetcher := &fakeCoverFetcher{}
		rec := httptest.NewRecorder()
		usdbCoverMux(fetcher, cacheDir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/usdb/cover/42", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "CACHED" {
			t.Errorf("body = %q, want CACHED", rec.Body.String())
		}
		if calls := fetcher.calls.Load(); calls != 0 {
			t.Errorf("fetcher called %d times; want 0 (cache hit)", calls)
		}
	})

	t.Run("upstream 404 writes miss marker and 404s thereafter", func(t *testing.T) {
		t.Parallel()
		cacheDir := t.TempDir()
		fetcher := &fakeCoverFetcher{err: os.ErrNotExist}

		// First request: fetcher reports miss, handler 404s.
		rec1 := httptest.NewRecorder()
		usdbCoverMux(fetcher, cacheDir).ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/api/usdb/cover/99", nil))
		if rec1.Code != http.StatusNotFound {
			t.Fatalf("first request: expected 404, got %d", rec1.Code)
		}
		// Marker file written.
		if _, err := os.Stat(filepath.Join(cacheDir, "99.miss")); err != nil {
			t.Fatalf("expected miss marker: %v", err)
		}

		// Second request: served from miss cache, fetcher NOT called again.
		fetcher.calls.Store(0)
		rec2 := httptest.NewRecorder()
		usdbCoverMux(fetcher, cacheDir).ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/usdb/cover/99", nil))
		if rec2.Code != http.StatusNotFound {
			t.Fatalf("second request: expected 404, got %d", rec2.Code)
		}
		if calls := fetcher.calls.Load(); calls != 0 {
			t.Errorf("fetcher called %d times on cached miss; want 0", calls)
		}
	})

	t.Run("upstream error returns 502", func(t *testing.T) {
		t.Parallel()
		fetcher := &fakeCoverFetcher{err: errors.New("network down")}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/usdb/cover/42", nil)
		usdbCoverMux(fetcher, t.TempDir()).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})
}

// libraryCoverMux mounts the library cover handler so r.PathValue resolves.
func libraryCoverMux(cache *server.LibraryCache, libraryDir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/library/{id}/cover", server.LibraryCoverHandler(cache, libraryDir))
	return mux
}

func TestLibraryCoverHandler(t *testing.T) {
	t.Parallel()

	t.Run("serves cover.jpg for a song in the library", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		songDir := filepath.Join(libraryDir, "Test - Song")
		if err := os.MkdirAll(songDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(songDir, "song.txt"),
			[]byte("#TITLE:Song\n#ARTIST:Test\n#MP3:audio.webm\n: 0 5 10 Hello\nE\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(songDir, "cover.jpg"), []byte("COVERBYTES"), 0o600); err != nil {
			t.Fatal(err)
		}

		expectedID := stableid.Compute("Test", "Song", false)
		cache := server.NewLibraryCache()
		rec := httptest.NewRecorder()
		libraryCoverMux(cache, libraryDir).ServeHTTP(rec,
			httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/cover", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "COVERBYTES" {
			t.Errorf("body = %q, want COVERBYTES", rec.Body.String())
		}
	})

	t.Run("404 for unknown song id", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		cache := server.NewLibraryCache()
		rec := httptest.NewRecorder()
		libraryCoverMux(cache, libraryDir).ServeHTTP(rec,
			httptest.NewRequest(http.MethodGet, "/api/library/deadbeefdeadbeef/cover", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("404 for song without cover.jpg on disk", func(t *testing.T) {
		t.Parallel()
		libraryDir := t.TempDir()
		songDir := filepath.Join(libraryDir, "Test - Song")
		if err := os.MkdirAll(songDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(songDir, "song.txt"),
			[]byte("#TITLE:Song\n#ARTIST:Test\n#MP3:audio.webm\n: 0 5 10 Hello\nE\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		// No cover.jpg.

		expectedID := stableid.Compute("Test", "Song", false)
		cache := server.NewLibraryCache()
		rec := httptest.NewRecorder()
		libraryCoverMux(cache, libraryDir).ServeHTTP(rec,
			httptest.NewRequest(http.MethodGet, "/api/library/"+expectedID+"/cover", nil))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}
