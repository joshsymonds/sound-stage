package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/joshsymonds/sound-stage/archive"
	"github.com/joshsymonds/sound-stage/usdb"
	"github.com/joshsymonds/sound-stage/ytdlp"
)

// Downloader abstracts the download pipeline for testing.
type Downloader interface {
	GetSongDetails(songID int) (*usdb.SongDetails, error)
	GetSongTxt(songID int) (string, error)
	DownloadCover(songID int, songDir string) error
}

// DownloadConfig holds configuration for the download handler.
type DownloadConfig struct {
	Client    Downloader
	YtDlp     ytdlp.Downloader
	OutputDir string
	// DeckURL is the Deck's USDX HTTP API base (e.g. "http://172.31.0.39:9000").
	// When set, runDownload POSTs /refresh after each successful download so
	// the Deck's in-memory library picks up the new song immediately.
	// Empty → skip the notify step.
	DeckURL string
	Logger  *slog.Logger
}

type downloadRequest struct {
	SongID int `json:"songId"`
}

type downloadStatus struct {
	mu     sync.Mutex
	active map[int]bool
}

func newDownloadStatus() *downloadStatus {
	return &downloadStatus{active: make(map[int]bool)}
}

func (ds *downloadStatus) start(songID int) bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.active[songID] {
		return false
	}
	ds.active[songID] = true
	return true
}

func (ds *downloadStatus) finish(songID int) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.active, songID)
}

// DownloadHandler triggers a background download of a USDB song.
func DownloadHandler(dlConfig DownloadConfig) http.Handler {
	status := newDownloadStatus()
	logger := dlConfig.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		// Check if already downloaded.
		downloaded, err := archive.LoadDownloaded(dlConfig.OutputDir)
		if err == nil {
			if _, ok := downloaded[req.SongID]; ok {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode( //nolint:gosec // best-effort
					map[string]string{"status": "already_downloaded"},
				)
				return
			}
		}

		// Check if download is already in progress.
		if !status.start(req.SongID) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode( //nolint:gosec // best-effort
				map[string]string{"status": "in_progress"},
			)
			return
		}

		// Kick off background download (intentionally detached from request context).
		go func() { //nolint:contextcheck // background download outlives the HTTP request
			defer status.finish(req.SongID)
			if downloadErr := runDownload(dlConfig, req.SongID, logger); downloadErr != nil {
				logger.Error("download failed",
					"song_id", req.SongID,
					"error", downloadErr,
				)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode( //nolint:gosec // best-effort
			map[string]string{"status": "downloading"},
		)
	})
}

func runDownload(dlConfig DownloadConfig, songID int, logger *slog.Logger) error {
	logger.Info("starting download", "song_id", songID)

	details, err := dlConfig.Client.GetSongDetails(songID)
	if err != nil {
		return fmt.Errorf("fetching song details: %w", err)
	}

	txt, err := dlConfig.Client.GetSongTxt(songID)
	if err != nil {
		return fmt.Errorf("getting song txt: %w", err)
	}

	dirName := usdb.SanitizePath(fmt.Sprintf("%s - %s", details.Artist, details.Title))
	songDir := filepath.Join(dlConfig.OutputDir, dirName)

	song, err := usdb.PrepareSong(txt, details, songDir)
	if err != nil {
		return fmt.Errorf("preparing song: %w", err)
	}

	if details.HasCover {
		if coverErr := dlConfig.Client.DownloadCover(songID, songDir); coverErr != nil {
			logger.Warn("cover download failed", "song_id", songID, "error", coverErr)
		}
	}

	if song.YouTubeURL == "" {
		logger.Warn("no YouTube URL found", "song_id", songID)

		if markErr := archive.MarkDownloaded(dlConfig.OutputDir, songID); markErr != nil {
			return fmt.Errorf("marking downloaded: %w", markErr)
		}

		notifyDeck(dlConfig.DeckURL, song.TxtPath, logger)
		return nil
	}

	var audioErr, videoErr error
	var wg sync.WaitGroup

	wg.Add(2) // audio + video goroutines
	go func() {
		defer wg.Done()
		audioErr = dlConfig.YtDlp.DownloadAudio(song.YouTubeURL, songDir, song.AudioFile)
	}()
	go func() {
		defer wg.Done()
		videoErr = dlConfig.YtDlp.DownloadVideo(song.YouTubeURL, songDir, song.VideoFile)
	}()
	wg.Wait()

	if audioErr != nil {
		return fmt.Errorf("downloading audio: %w", audioErr)
	}
	if videoErr != nil {
		logger.Warn("video download failed", "song_id", songID, "error", videoErr)
	}

	if markErr := archive.MarkDownloaded(dlConfig.OutputDir, songID); markErr != nil {
		return fmt.Errorf("marking downloaded: %w", markErr)
	}

	notifyDeck(dlConfig.DeckURL, song.TxtPath, logger)
	return nil
}

// notifyDeck POSTs /refresh to the Deck so its in-memory library picks up
// the newly-downloaded song. Best-effort: failures log a warning but don't
// interrupt the caller — the download itself succeeded regardless.
func notifyDeck(deckURL, txtPath string, logger *slog.Logger) {
	if deckURL == "" {
		return
	}

	body, err := json.Marshal(map[string]string{"path": txtPath})
	if err != nil {
		logger.Warn("marshal /refresh payload", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), proxyTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deckURL+"/refresh", bytes.NewReader(body))
	if err != nil {
		logger.Warn("build /refresh request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("deck unreachable for /refresh", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("deck /refresh returned non-200",
			"status", resp.StatusCode,
			"path", txtPath,
		)
		return
	}
	logger.Debug("deck accepted /refresh", "path", txtPath)
}
