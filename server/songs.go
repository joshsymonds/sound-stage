// Package server implements the SoundStage HTTP server.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/joshsymonds/sound-stage/server/stableid"
	"github.com/joshsymonds/sound-stage/server/txtparse"
)

// Song represents a song in the library, matching the frontend Song interface.
// ID is the 16-hex stableid.Compute(Artist, Title, Duet) hash — matches the
// identity USDX uses, so POST /queue can resolve it on the Deck side.
type Song struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Duet    bool   `json:"duet"`
	Edition string `json:"edition,omitempty"`
	Year    int    `json:"year,omitempty"`
}

// LibraryCache holds a scanned []Song in memory plus a stableid → song-dir
// map for cover lookups. Invalidates on demand: the first Get scans the
// library directory; subsequent calls return the cached slice without
// re-reading the filesystem. Invalidate flips the cache stale so the next
// Get re-scans — call it after downloads complete.
//
// Safe for concurrent use. Reads take an RLock fast path; scans take a
// write lock with a double-check to avoid concurrent scans.
type LibraryCache struct {
	mu     sync.RWMutex
	songs  []Song
	paths  map[string]string // stableid → absolute song directory
	loaded bool
}

// NewLibraryCache returns an empty, unloaded cache.
func NewLibraryCache() *LibraryCache {
	return &LibraryCache{}
}

// Get returns the cached songs, scanning libraryDir on first use.
func (c *LibraryCache) Get(libraryDir string) ([]Song, error) {
	c.mu.RLock()
	if c.loaded {
		songs := c.songs
		c.mu.RUnlock()
		return songs, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return c.songs, nil
	}
	songs, paths, err := scanLibrary(libraryDir)
	if err != nil {
		return nil, err
	}
	c.songs = songs
	c.paths = paths
	c.loaded = true
	return c.songs, nil
}

// Path returns the absolute directory for the song with the given stableid,
// or false if the cache hasn't loaded yet or the id isn't present.
func (c *LibraryCache) Path(id string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.loaded {
		return "", false
	}
	dir, ok := c.paths[id]
	return dir, ok
}

// Invalidate marks the cache stale so the next Get rescans libraryDir.
// Call this after a successful download or any operation that changes the
// on-disk library.
func (c *LibraryCache) Invalidate() {
	c.mu.Lock()
	c.songs, c.paths, c.loaded = nil, nil, false
	c.mu.Unlock()
}

// SongsHandler returns an http.Handler that serves the library via the given
// cache. Empty library returns [].
func SongsHandler(cache *LibraryCache, libraryDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		songs, err := cache.Get(libraryDir)
		if err != nil {
			slog.Default().Error("scanning library", "error", err)
			http.Error(w, "failed to scan library", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if encodeErr := json.NewEncoder(w).Encode(songs); encodeErr != nil {
			slog.Default().Error("encoding songs response", "error", encodeErr)
		}
	})
}

func scanLibrary(dir string) ([]Song, map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("reading library directory: %w", err)
	}

	logger := slog.Default()
	songs := make([]Song, 0, len(entries))
	paths := make(map[string]string, len(entries))
	seenIDs := make(map[string]string, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		songDir := filepath.Join(dir, entry.Name())
		txtPath := filepath.Join(songDir, "song.txt")
		song, parseErr := parseSongFile(txtPath)
		if parseErr != nil {
			continue
		}

		if existingPath, collides := seenIDs[song.ID]; collides {
			logger.Warn("song id collision",
				"id", song.ID,
				"keeping_path", existingPath,
				"discarded_path", txtPath,
			)
			continue
		}
		seenIDs[song.ID] = txtPath
		paths[song.ID] = songDir
		songs = append(songs, song)
	}

	return songs, paths, nil
}

func parseSongFile(path string) (Song, error) {
	parsed, err := txtparse.Parse(path)
	if err != nil {
		return Song{}, fmt.Errorf("parse: %w", err)
	}

	return Song{
		ID:      stableid.Compute(parsed.Artist, parsed.Title, parsed.Duet),
		Title:   parsed.Title,
		Artist:  parsed.Artist,
		Duet:    parsed.Duet,
		Edition: parsed.Edition,
		Year:    parsed.Year,
	}, nil
}
