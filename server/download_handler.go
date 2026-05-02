package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/joshsymonds/sound-stage/archive"
	"github.com/joshsymonds/sound-stage/server/stableid"
	"github.com/joshsymonds/sound-stage/server/txtparse"
	"github.com/joshsymonds/sound-stage/usdb"
)

// Downloader abstracts the USDB client for testing. Ready reports whether
// the underlying client has logged in; the handler short-circuits with
// HTTP 503 while it's false.
type Downloader interface {
	Ready() bool
	GetSongDetails(songID int) (*usdb.SongDetails, error)
	GetSongTxt(songID int) (string, error)
	DownloadCover(songID int, songDir string) error
}

// YtDlp abstracts the yt-dlp wrapper for testing. The real implementation is
// ytdlp.Downloader; tests inject a no-op mock so they can exercise the full
// download happy path (including notifyDeck) without spawning yt-dlp.
type YtDlp interface {
	DownloadAudio(videoURL, destDir, filename string) error
	DownloadVideo(videoURL, destDir, filename string) error
}

// DownloadConfig holds configuration for the download handler.
type DownloadConfig struct {
	Client    Downloader
	YtDlp     YtDlp
	OutputDir string
	// DeckURL is the Deck's USDX HTTP API base (e.g. "http://172.31.0.39:9000").
	// When set, runDownload POSTs /refresh after each successful download so
	// the Deck's in-memory library picks up the new song immediately.
	// Empty → skip the notify step (and skip auto-queue, since a song USDX
	// hasn't been told about would 404 when the queue driver tries to stage).
	DeckURL string
	// InvalidateLibrary is called after each successful download so a cached
	// library snapshot (LibraryCache) re-scans on the next GET /api/songs.
	// Optional — a nil hook is a no-op.
	InvalidateLibrary func()
	// Queue is the shared in-memory queue. After a successful download +
	// /refresh, the requesting guest's name is added to the queue with a
	// Song whose ID matches USDX's stableid for the parsed .txt. Optional —
	// a nil queue disables auto-queue (handler still services downloads).
	Queue *Queue
	// HTTPClient is the http.Client used for notifyDeck POSTs. Optional —
	// nil falls back to http.DefaultClient. server.HandlerWithQueue wires
	// its shared deck-proxy client here so /refresh shares a pool with the
	// other Deck-bound handlers.
	HTTPClient *http.Client
	Logger     *slog.Logger
}

type downloadRequest struct {
	SongID int    `json:"songId"`
	Guest  string `json:"guest"`
}

// downloadStatus tracks in-flight downloads and the guests waiting on each.
// When two guests request the same song concurrently, only one download
// actually runs but both names land in the queue when it completes.
type downloadStatus struct {
	mu      sync.Mutex
	pending map[int][]string
}

func newDownloadStatus() *downloadStatus {
	return &downloadStatus{pending: make(map[int][]string)}
}

// archiveCache memoises archive.LoadDownloaded so the request handler
// doesn't read .downloaded.txt from disk on every POST. The map is
// invalidated after each successful MarkDownloaded call so a song that
// just finished downloading shows up on the next request.
type archiveCache struct {
	mu      sync.RWMutex
	dir     string
	loaded  bool
	entries map[int]struct{}
	logger  *slog.Logger
}

func newArchiveCache(dir string, logger *slog.Logger) *archiveCache {
	return &archiveCache{dir: dir, logger: logger}
}

func (a *archiveCache) has(songID int) bool {
	a.mu.RLock()
	if a.loaded {
		_, ok := a.entries[songID]
		a.mu.RUnlock()
		return ok
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loaded {
		_, ok := a.entries[songID]
		return ok
	}
	entries, err := archive.LoadDownloaded(a.dir)
	if err != nil {
		// Surface the error so a corrupted archive doesn't silently turn
		// every request into a re-download. Treat as "not in archive" for
		// this request only — don't memoise the failure.
		a.logger.Error("loading archive", "dir", a.dir, "error", err)
		return false
	}
	a.entries = entries
	a.loaded = true
	_, ok := a.entries[songID]
	return ok
}

func (a *archiveCache) invalidate() {
	a.mu.Lock()
	a.entries, a.loaded = nil, false
	a.mu.Unlock()
}

// start records guest as waiting on this song. Returns true iff this is the
// first requester (caller is responsible for spawning the download goroutine).
// Returns false if a download is already running; the existing goroutine will
// pick up this guest via finish.
func (ds *downloadStatus) start(songID int, guest string) bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	waiters, exists := ds.pending[songID]
	ds.pending[songID] = append(waiters, guest)
	return !exists
}

// finish drains and returns all guests waiting on this song. Always called
// by the goroutine that issued start so a future request for the same song
// can re-trigger if needed (e.g., after a download failure).
func (ds *downloadStatus) finish(songID int) []string {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	waiters := ds.pending[songID]
	delete(ds.pending, songID)
	return waiters
}

// DownloadHandler triggers a background download of a USDB song and queues
// the requesting guest(s) once the song is available on USDX.
func DownloadHandler(dlConfig DownloadConfig) http.Handler {
	status := newDownloadStatus()
	logger := dlConfig.Logger
	if logger == nil {
		logger = slog.Default()
	}
	archiveSet := newArchiveCache(dlConfig.OutputDir, logger)
	// Wrap the existing InvalidateLibrary hook so the archive cache is
	// dropped alongside the library cache after each successful download.
	prevInvalidate := dlConfig.InvalidateLibrary
	dlConfig.InvalidateLibrary = func() {
		archiveSet.invalidate()
		if prevInvalidate != nil {
			prevInvalidate()
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !dlConfig.Client.Ready() {
			writeUSDBNotReady(w)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		var req downloadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.SongID <= 0 {
			http.Error(w, "valid songId is required", http.StatusBadRequest)
			return
		}
		if req.Guest == "" {
			http.Error(w, "guest is required", http.StatusBadRequest)
			return
		}

		// Already on disk: queue inline (no goroutine needed) and return.
		if archiveSet.has(req.SongID) {
			queueAlreadyDownloaded(dlConfig, req.SongID, req.Guest, logger)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode( //nolint:gosec // best-effort
				map[string]string{"status": "already_downloaded"},
			)
			return
		}

		// Always record the guest as a waiter — the existing goroutine (if
		// any) will pick them up at finish time.
		firstRequester := status.start(req.SongID, req.Guest)
		if !firstRequester {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode( //nolint:gosec // best-effort
				map[string]string{"status": "in_progress"},
			)
			return
		}

		// Kick off background download (intentionally detached from request context).
		// runDownload owns the waiters list lifecycle — it always calls
		// status.finish at the end (success or failure) so the pending map
		// can't leak.
		go func() { //nolint:contextcheck // background download outlives the HTTP request
			runDownload(dlConfig, req.SongID, status, logger)
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode( //nolint:gosec // best-effort
			map[string]string{"status": "downloading"},
		)
	})
}

// runDownload executes the full pipeline for songID. It always drains the
// waiter list at the end (deferred) so the pending map can't leak. On full
// success (download → /refresh OK), the drained guests are added to the
// queue with a Song whose ID matches what USDX computes for the .txt content.
//
// Returns no error — failures are logged and the queue is simply not updated.
func runDownload(dlConfig DownloadConfig, songID int, status *downloadStatus, logger *slog.Logger) {
	var queued *Song // populated only on full success
	defer func() {
		guests := status.finish(songID)
		if queued == nil || dlConfig.Queue == nil {
			return
		}
		for _, guest := range guests {
			dlConfig.Queue.Add(*queued, guest)
		}
	}()

	logger.Info("starting download", "song_id", songID)

	details, err := dlConfig.Client.GetSongDetails(songID)
	if err != nil {
		logger.Error("download failed", "song_id", songID, "error", fmt.Errorf("fetching song details: %w", err))
		return
	}

	txt, err := dlConfig.Client.GetSongTxt(songID)
	if err != nil {
		logger.Error("download failed", "song_id", songID, "error", fmt.Errorf("getting song txt: %w", err))
		return
	}

	dirName := usdb.SanitizePath(fmt.Sprintf("%s - %s", details.Artist, details.Title))
	songDir := filepath.Join(dlConfig.OutputDir, dirName)

	song, err := usdb.PrepareSong(txt, details, songDir)
	if err != nil {
		logger.Error("download failed", "song_id", songID, "error", fmt.Errorf("preparing song: %w", err))
		return
	}

	if details.HasCover {
		if coverErr := dlConfig.Client.DownloadCover(songID, songDir); coverErr != nil {
			// Missing covers are normal (USDB serves nocover.png for many
			// songs, and FetchCover translates HTTP 404 to os.ErrNotExist).
			// Demote those to Debug so production logs aren't drowned in
			// false-alarm Warns; reserve Warn for transient failures.
			if errors.Is(coverErr, os.ErrNotExist) {
				logger.Debug("no cover available", "song_id", songID)
			} else {
				logger.Warn("cover download failed", "song_id", songID, "error", coverErr)
			}
		}
	}

	if song.YouTubeURL == "" {
		logger.Warn("no YouTube URL found", "song_id", songID)

		if markErr := archive.MarkDownloaded(dlConfig.OutputDir, songID); markErr != nil {
			logger.Error("marking downloaded", "song_id", songID, "error", markErr)
		}

		// Skip notifyDeck and skip queueing: without audio/video the Deck
		// would accept the song into its library only to 404 when the user
		// tries to play it. The archive mark keeps the download handler
		// from retrying this known-unplayable song.
		return
	}

	if !fetchMedia(dlConfig.YtDlp, song, songDir, songID, logger) {
		return
	}
	queued = finalizeDownload(dlConfig, song.TxtPath, songID, logger)
}

// finalizeDownload runs the post-media-download tail: marks the archive,
// invalidates the library cache, notifies the Deck, and parses the .txt to
// build a Song for auto-queue. Returns nil if any step that gates queueing
// failed (mark error, /refresh error, parse error). Library invalidate and
// notifyDeck are skipped when their respective config is unset.
func finalizeDownload(dlConfig DownloadConfig, txtPath string, songID int, logger *slog.Logger) *Song {
	if markErr := archive.MarkDownloaded(dlConfig.OutputDir, songID); markErr != nil {
		logger.Error("marking downloaded", "song_id", songID, "error", markErr)
		return nil
	}
	if dlConfig.InvalidateLibrary != nil {
		dlConfig.InvalidateLibrary()
	}
	// With a configured Deck, /refresh must succeed before queueing — otherwise
	// USDX doesn't know the song and the queue driver would 404 on stage.
	if dlConfig.DeckURL != "" && !notifyDeck(dlConfig.HTTPClient, dlConfig.DeckURL, txtPath, logger) {
		return nil
	}
	parsed, parseErr := txtparse.Parse(txtPath)
	if parseErr != nil {
		logger.Error("parse txt for auto-queue", "song_id", songID, "error", parseErr)
		return nil
	}
	return &Song{
		ID:      stableid.Compute(parsed.Artist, parsed.Title, parsed.Duet),
		Title:   parsed.Title,
		Artist:  parsed.Artist,
		Duet:    parsed.Duet,
		Edition: parsed.Edition,
		Year:    parsed.Year,
	}
}

// queueAlreadyDownloaded handles the "guest tapped a USDB result for a song
// already on disk" path. It looks up the existing .txt to derive a Song that
// matches USDX's identity (artist/title/duet via stableid.Compute) so the
// queue driver can stage it without a 404.
//
// Failure modes are logged but never returned: the HTTP response is already
// "already_downloaded" — surfacing a 500 for an edge case (archive entry
// without a .txt) would be more confusing than a silent skip.
func queueAlreadyDownloaded(dlConfig DownloadConfig, songID int, guest string, logger *slog.Logger) {
	if dlConfig.Queue == nil {
		return
	}
	details, err := dlConfig.Client.GetSongDetails(songID)
	if err != nil {
		logger.Warn("auto-queue: get song details failed", "song_id", songID, "error", err)
		return
	}
	dirName := usdb.SanitizePath(fmt.Sprintf("%s - %s", details.Artist, details.Title))
	txtPath := filepath.Join(dlConfig.OutputDir, dirName, "song.txt")
	parsed, parseErr := txtparse.Parse(txtPath)
	if parseErr != nil {
		logger.Warn("auto-queue: parse existing .txt failed",
			"song_id", songID, "path", txtPath, "error", parseErr)
		return
	}
	dlConfig.Queue.Add(Song{
		ID:      stableid.Compute(parsed.Artist, parsed.Title, parsed.Duet),
		Title:   parsed.Title,
		Artist:  parsed.Artist,
		Duet:    parsed.Duet,
		Edition: parsed.Edition,
		Year:    parsed.Year,
	}, guest)
}

// fetchMedia runs the parallel yt-dlp audio + video downloads. Returns true
// iff audio succeeded (video failure is non-fatal — songs work without it).
func fetchMedia(dl YtDlp, song *usdb.PreparedSong, songDir string, songID int, logger *slog.Logger) bool {
	var audioErr, videoErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		audioErr = dl.DownloadAudio(song.YouTubeURL, songDir, song.AudioFile)
	}()
	go func() {
		defer wg.Done()
		videoErr = dl.DownloadVideo(song.YouTubeURL, songDir, song.VideoFile)
	}()
	wg.Wait()

	if audioErr != nil {
		logger.Error("download failed", "song_id", songID, "error", fmt.Errorf("downloading audio: %w", audioErr))
		return false
	}
	if videoErr != nil {
		logger.Warn("video download failed", "song_id", songID, "error", videoErr)
	}
	return true
}

// notifyDeck POSTs /refresh to the Deck so its in-memory library picks up
// the newly-downloaded song. Returns true iff the Deck responded HTTP 200.
// Failures are logged but do not panic — the download itself succeeded.
// A nil client falls back to http.DefaultClient (test convenience).
func notifyDeck(client *http.Client, deckURL, txtPath string, logger *slog.Logger) bool {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(map[string]string{"path": txtPath})
	if err != nil {
		logger.Warn("marshal /refresh payload", "error", err)
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), proxyTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deckURL+"/refresh", bytes.NewReader(body))
	if err != nil {
		logger.Warn("build /refresh request", "error", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("deck unreachable for /refresh", "error", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("deck /refresh returned non-200",
			"status", resp.StatusCode,
			"path", txtPath,
		)
		return false
	}
	logger.Debug("deck accepted /refresh", "path", txtPath)
	return true
}
