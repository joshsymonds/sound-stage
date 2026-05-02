package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// CoverFetcher abstracts USDB.FetchCover for testability. Ready reports
// whether the underlying client has logged in.
type CoverFetcher interface {
	Ready() bool
	FetchCover(ctx context.Context, songID int) (io.ReadCloser, string, error)
}

const (
	// coverCacheControl tells the browser to keep covers in its HTTP cache for
	// 5 minutes. Covers are content-addressed (USDB id / library stableid),
	// so 5 minutes is plenty without making evictions awkward when a song's
	// metadata changes.
	coverCacheControl = "public, max-age=300"

	// coverFetchTimeout caps the upstream USDB request. USDB serves images
	// from the same host that hosts the API; if it's slow enough to hit
	// this we'd rather fail and let the browser fall back to the placeholder.
	coverFetchTimeout = 10 * time.Second

	// coverMissTTL bounds the negative-cache lifetime. Without this, a song
	// that ever 404'd would be marked "no cover" forever — but USDB
	// occasionally backfills covers for older songs. 7 days is a forgiving
	// retry window without giving up the rate-limit win.
	coverMissTTL = 7 * 24 * time.Hour

	// coverCacheDirPerm / coverCacheFilePerm are the perms applied to the
	// cache dir and the files inside. Sensitive content isn't an issue
	// (these are just album art) so 0o755 / 0o644 is fine.
	coverCacheDirPerm  = 0o755
	coverCacheFilePerm = 0o644
)

// USDBCoverHandler proxies cover images from USDB through our authenticated
// session, caching successful fetches under cacheDir as `<id>.jpg` and
// negative responses as `<id>.miss` (empty file). Browsers can't hit USDB
// directly because covers require a logged-in session, and USDB doesn't
// rotate cover IDs, so on-disk caching is essentially free.
//
// cacheDir == "" disables caching (every request hits upstream). Production
// always passes a real directory; this fallback is for tests.
func USDBCoverHandler(fetcher CoverFetcher, cacheDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		songID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil || songID <= 0 {
			http.Error(w, "valid id is required", http.StatusBadRequest)
			return
		}
		// Cached hits/misses don't need an authenticated session, so they
		// short-circuit the readiness gate. This keeps already-fetched
		// covers visible during a USDB outage.
		if cacheDir != "" && serveFromCoverCache(w, r, cacheDir, songID) {
			return
		}
		if !fetcher.Ready() {
			writeUSDBNotReady(w)
			return
		}
		fetchAndStreamCover(w, r, fetcher, cacheDir, songID)
	})
}

// fetchAndStreamCover does the upstream fetch and tees into the cache file.
// All failure paths fall back to streaming the upstream body if available,
// or returning the appropriate HTTP error otherwise.
func fetchAndStreamCover(
	w http.ResponseWriter,
	r *http.Request,
	fetcher CoverFetcher,
	cacheDir string,
	songID int,
) {
	ctx, cancel := context.WithTimeout(r.Context(), coverFetchTimeout)
	defer cancel()

	body, contentType, err := fetcher.FetchCover(ctx, songID)
	if errors.Is(err, os.ErrNotExist) {
		markCoverMiss(cacheDir, songID)
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "cover unavailable", http.StatusBadGateway)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", coverCacheControl)
	streamCoverWithCache(w, body, cacheDir, songID)
}

// streamCoverWithCache copies body to w while teeing to a cache tmp file.
// On copy success the tmp is renamed in place; on failure the tmp is
// removed. If the cache can't be opened at all, the body is streamed to
// the client uncached.
func streamCoverWithCache(w http.ResponseWriter, body io.Reader, cacheDir string, songID int) {
	cachePath, ok := openCoverCacheTmp(cacheDir, songID)
	if !ok {
		_, _ = io.Copy(w, body)
		return
	}
	tmp, openErr := os.OpenFile( //nolint:gosec // path under controlled cacheDir
		cachePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, coverCacheFilePerm,
	)
	if openErr != nil {
		_, _ = io.Copy(w, body)
		return
	}
	_, copyErr := io.Copy(w, io.TeeReader(body, tmp))
	_ = tmp.Close()
	if copyErr != nil {
		_ = os.Remove(cachePath) //nolint:errcheck // best-effort cache cleanup
		return
	}
	_ = os.Rename(cachePath, coverHitPath(cacheDir, songID)) //nolint:errcheck // best-effort cache write
}

func coverHitPath(cacheDir string, songID int) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%d.jpg", songID))
}

func coverMissPath(cacheDir string, songID int) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%d.miss", songID))
}

// serveFromCoverCache returns true if the request was satisfied from disk
// (either a cached hit or a cached miss within TTL). Returns false to let
// the caller fall through to the upstream fetch.
func serveFromCoverCache(w http.ResponseWriter, r *http.Request, cacheDir string, songID int) bool {
	missPath := coverMissPath(cacheDir, songID)
	if info, err := os.Stat(missPath); err == nil {
		if time.Since(info.ModTime()) < coverMissTTL {
			http.NotFound(w, r)
			return true
		}
		// TTL elapsed — let the caller re-attempt upstream. The next
		// markCoverMiss (or successful fetch) will replace this file.
	}
	hitPath := coverHitPath(cacheDir, songID)
	if _, err := os.Stat(hitPath); err == nil {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", coverCacheControl)
		http.ServeFile(w, r, hitPath)
		return true
	}
	return false
}

// openCoverCacheTmp ensures the cache dir exists and returns the temp path
// the caller should write to before atomically renaming to the final name.
func openCoverCacheTmp(cacheDir string, songID int) (string, bool) {
	if cacheDir == "" {
		return "", false
	}
	if err := os.MkdirAll(cacheDir, coverCacheDirPerm); err != nil {
		slog.Default().Warn("cover cache mkdir", "dir", cacheDir, "error", err)
		return "", false
	}
	return coverHitPath(cacheDir, songID) + ".tmp", true
}

// markCoverMiss writes an empty <id>.miss marker so future requests for
// the same id short-circuit without hitting USDB. Best-effort: a failed
// write just means we'll re-attempt next time.
func markCoverMiss(cacheDir string, songID int) {
	if cacheDir == "" {
		return
	}
	if err := os.MkdirAll(cacheDir, coverCacheDirPerm); err != nil {
		return
	}
	_ = os.WriteFile( //nolint:errcheck // best-effort negative-cache marker
		coverMissPath(cacheDir, songID), nil, coverCacheFilePerm,
	)
}

// LibraryCoverHandler serves cover.jpg from a song's directory under the
// configured library. Lookup is by stableid via the LibraryCache.
func LibraryCoverHandler(cache *LibraryCache, libraryDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		// Get triggers a scan if needed; Path then resolves the directory.
		if _, err := cache.Get(libraryDir); err != nil {
			http.Error(w, "library scan failed", http.StatusInternalServerError)
			return
		}
		dir, ok := cache.Path(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		// filepath.Join cleans the path; the dir came from os.ReadDir entries
		// inside libraryDir so there's no traversal opening here.
		path := filepath.Join(dir, "cover.jpg")
		if _, err := os.Stat(path); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", coverCacheControl)
		http.ServeFile(w, r, path)
	})
}
