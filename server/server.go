package server

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Config holds the server configuration.
type Config struct {
	Port       string
	LibraryDir string
	// StaticFS holds the SPA assets to serve at /. In production, this is the
	// embed.FS sub-tree wired in main.go. Tests pass an fstest.MapFS. A nil
	// StaticFS disables the SPA route entirely (API-only mode).
	StaticFS fs.FS
	Searcher USDBSearcher
	// CoverFetcher exposes USDB's authenticated cover endpoint to the web
	// client via /api/usdb/cover/{id}. The serve command wires the same
	// usdb.Client used for Searcher/Download.
	CoverFetcher CoverFetcher
	Download     *DownloadConfig
	DeckURL      string // Pascal API base URL (e.g. "http://172.31.0.39:9000")
	DelyricURL   string // Delyric worker base URL (e.g. "http://172.31.0.98:9001")
	// DeckStatus, when set, drives /api/deck-status. The serve command wires
	// the QueueDriver here so the UI can surface offline state.
	DeckStatus deckStatusReporter
}

// Handler creates the HTTP handler with all routes configured.
func Handler(cfg Config) http.Handler {
	return HandlerWithQueue(cfg, NewQueue())
}

// HandlerWithQueue creates the HTTP handler with a provided queue instance.
func HandlerWithQueue(cfg Config, queue *Queue) http.Handler {
	mux := http.NewServeMux()

	// Shared library cache: scanned once lazily, invalidated by the download
	// pipeline when new songs land.
	libCache := NewLibraryCache()

	// API routes.
	mux.Handle("GET /api/songs", SongsHandler(libCache, cfg.LibraryDir))
	mux.Handle("GET /api/library/{id}/cover", LibraryCoverHandler(libCache, cfg.LibraryDir))
	mux.Handle("GET /api/queue", QueueListHandler(queue))
	mux.Handle("POST /api/queue", QueueAddHandler(queue))
	mux.Handle("DELETE /api/queue", QueueRemoveByGuestHandler(queue))
	mux.Handle("DELETE /api/queue/{position}", QueueRemoveHandler(queue))
	mux.Handle("GET /api/deck-status", DeckStatusHandler(cfg.DeckStatus))

	// USDB search proxy (optional — requires credentials).
	if cfg.Searcher != nil {
		mux.Handle("GET /api/usdb/search", USDBSearchHandler(cfg.Searcher))
	}

	// USDB cover proxy (optional — requires authenticated client). Cache
	// to disk under <libraryDir>/.usdb-cache so we hit USDB at most once
	// per cover ID across the lifetime of the deployment.
	if cfg.CoverFetcher != nil {
		cacheDir := filepath.Join(cfg.LibraryDir, ".usdb-cache")
		mux.Handle("GET /api/usdb/cover/{id}", USDBCoverHandler(cfg.CoverFetcher, cacheDir))
	}

	// Download trigger (optional — requires USDB client + yt-dlp).
	if cfg.Download != nil {
		dl := *cfg.Download
		dl.InvalidateLibrary = libCache.Invalidate
		dl.Queue = queue
		mux.Handle("POST /api/download", DownloadHandler(dl))
	}

	// Playback proxy to Steam Deck Pascal API.
	mux.Handle("GET /api/now-playing", NowPlayingProxyHandler(cfg.DeckURL))
	mux.Handle("POST /api/playback/pause", PlaybackProxyHandler(cfg.DeckURL, "/pause"))
	mux.Handle("POST /api/playback/resume", PlaybackProxyHandler(cfg.DeckURL, "/resume"))

	// Delyric worker proxy (optional).
	if cfg.DelyricURL != "" {
		mux.Handle("POST /api/delyric/process",
			DelyricProxyHandler(cfg.DelyricURL, http.MethodPost, "/process"))
	}

	// SPA static file server with fallback to index.html.
	if cfg.StaticFS != nil {
		mux.Handle("/", spaHandler(cfg.StaticFS))
	}

	return mux
}

// New creates a configured HTTP server with a new queue.
func New(cfg Config) *http.Server {
	return NewWithQueue(cfg, NewQueue())
}

// NewWithQueue creates a configured HTTP server with a shared queue.
func NewWithQueue(cfg Config, queue *Queue) *http.Server {
	const readHeaderTimeout = 10 * time.Second

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           HandlerWithQueue(cfg, queue),
		ReadHeaderTimeout: readHeaderTimeout,
	}
}

// spaHandler serves static files from staticFS, falling back to index.html
// for non-file routes (SPA client-side routing).
func spaHandler(staticFS fs.FS) http.Handler {
	fileServer := http.FileServerFS(staticFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the file exists in the embedded/passed FS.
		cleanPath := strings.TrimPrefix(path, "/")
		if _, err := fs.Stat(staticFS, cleanPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// API routes should not fall through to SPA.
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Fallback: serve index.html for SPA routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
