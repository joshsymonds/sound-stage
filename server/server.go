package server

import (
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config holds the server configuration.
type Config struct {
	Port       string
	LibraryDir string
	StaticDir  string
	Searcher   USDBSearcher
	Download   *DownloadConfig
	DeckURL    string // Pascal API base URL (e.g. "http://172.31.0.39:9000")
	DelyricURL string // Delyric worker base URL (e.g. "http://172.31.0.98:9001")
}

// Handler creates the HTTP handler with all routes configured.
func Handler(cfg Config) http.Handler {
	return HandlerWithQueue(cfg, NewQueue())
}

// HandlerWithQueue creates the HTTP handler with a provided queue instance.
func HandlerWithQueue(cfg Config, queue *Queue) http.Handler {
	mux := http.NewServeMux()

	// API routes.
	mux.Handle("GET /api/songs", SongsHandler(cfg.LibraryDir))
	mux.Handle("GET /api/queue", QueueListHandler(queue))
	mux.Handle("POST /api/queue", QueueAddHandler(queue))
	mux.Handle("POST /api/queue/skip", QueueSkipHandler(queue))

	// USDB search proxy (optional — requires credentials).
	if cfg.Searcher != nil {
		mux.Handle("GET /api/usdb/search", USDBSearchHandler(cfg.Searcher))
	}

	// Download trigger (optional — requires USDB client + yt-dlp).
	if cfg.Download != nil {
		mux.Handle("POST /api/download", DownloadHandler(*cfg.Download))
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
	if cfg.StaticDir != "" {
		mux.Handle("/", spaHandler(cfg.StaticDir))
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

// spaHandler serves static files from dir, falling back to index.html
// for non-file routes (SPA client-side routing).
func spaHandler(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the file exists on disk.
		cleanPath := strings.TrimPrefix(path, "/")
		if _, err := fs.Stat(os.DirFS(dir), cleanPath); err == nil {
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
