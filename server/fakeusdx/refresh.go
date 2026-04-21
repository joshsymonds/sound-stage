package fakeusdx

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshsymonds/sound-stage/server/stableid"
)

// parseRefreshBody validates the POST /refresh request shape. The returned
// path is raw (not validated against the filesystem).
func parseRefreshBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxQueueBodySize))
	if err != nil {
		var mbErr *http.MaxBytesError
		if errors.As(err, &mbErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "body too large")
			return "", false
		}
		writeError(w, http.StatusBadRequest, "malformed json")
		return "", false
	}

	var raw any
	if jsonErr := json.Unmarshal(body, &raw); jsonErr != nil {
		writeError(w, http.StatusBadRequest, "malformed json")
		return "", false
	}
	obj, isObject := raw.(map[string]any)
	if !isObject {
		writeError(w, http.StatusBadRequest, "body must be an object")
		return "", false
	}

	path, ok := obj["path"].(string)
	if !ok || path == "" {
		writeError(w, http.StatusBadRequest, "path required (string)")
		return "", false
	}

	return path, true
}

func (f *Fake) handleRefresh(w http.ResponseWriter, r *http.Request) {
	path, ok := parseRefreshBody(w, r)
	if !ok {
		return
	}

	if !filepath.IsAbs(path) {
		writeAddedFalseError(w, http.StatusBadRequest, "path must be absolute")
		return
	}
	if !strings.EqualFold(filepath.Ext(path), ".txt") {
		writeAddedFalseError(w, http.StatusBadRequest, "path must end in .txt")
		return
	}

	// Cheap screen check before disk I/O to avoid wasted NFS round-trips on 409.
	if f.Screen() == ScreenSing {
		writeAddedFalseError(w, http.StatusConflict, "song in progress")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeAddedFalseError(w, http.StatusNotFound, "path not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if info.IsDir() {
		writeAddedFalseError(w, http.StatusNotFound, "path not found")
		return
	}

	song, err := parseTxtFile(path)
	if err != nil {
		writeAddedFalseError(w, http.StatusBadRequest, "parse failed")
		return
	}

	id := stableid.Compute(song.Artist, song.Title, song.Duet)
	entry := songEntry{
		ID:     id,
		Title:  song.Title,
		Artist: song.Artist,
		Duet:   song.Duet,
		Path:   path,
	}

	f.mu.Lock()
	// Re-check screen under lock (can't lose a race with SetScreen).
	if f.screen == ScreenSing {
		f.mu.Unlock()
		writeAddedFalseError(w, http.StatusConflict, "song in progress")
		return
	}
	f.applyRefreshLocked(entry)
	f.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"added": true,
		"id":    id,
		"title": song.Title,
	})
}

// applyRefreshLocked adds or replaces entry in f.songs. Caller must hold f.mu.
//
// Ordering rules:
//  1. If any existing entry has the same Path, replace it in place (keeps
//     positional index).
//  2. Otherwise, if any existing entry has the same ID (collision — different
//     path, same normalized metadata), remove it and append the new entry.
//  3. Otherwise, append the new entry.
func (f *Fake) applyRefreshLocked(entry songEntry) {
	for i, existing := range f.songs {
		if existing.Path == entry.Path {
			f.songs[i] = entry
			return
		}
	}
	for i, existing := range f.songs {
		if existing.ID == entry.ID {
			f.songs = append(f.songs[:i], f.songs[i+1:]...)
			break
		}
	}
	f.songs = append(f.songs, entry)
}

// writeAddedFalseError emits {"added":false,"error":"<msg>"} with the given
// status. Used for errors that surface after path extraction succeeds — the
// different body shape is mandated by API.md.
func writeAddedFalseError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"added": false, "error": msg})
}

// parseTxtFile reads and parses a USDX .txt file, extracting #ARTIST, #TITLE,
// and duet state. Returns an error for any missing or empty required field.
func parseTxtFile(path string) (Song, error) {
	data, err := os.ReadFile(path) //nolint:gosec // dev-only fake; path comes from trusted LAN POST body
	if err != nil {
		return Song{}, fmt.Errorf("read %s: %w", path, err)
	}

	var artist, title string
	duet := false

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		trimmed := strings.TrimSpace(line)

		// Standalone P1 / P2 markers indicate duet songs.
		if trimmed == "P1" || trimmed == "P2" {
			duet = true
			continue
		}
		if !strings.HasPrefix(line, "#") {
			continue
		}

		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(line[1:colon]))
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "ARTIST":
			artist = val
		case "TITLE":
			title = val
		case "DUETSINGERP1", "DUETSINGERP2":
			duet = true
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return Song{}, fmt.Errorf("scan %s: %w", path, scanErr)
	}

	if artist == "" || title == "" {
		return Song{}, errors.New("missing #ARTIST or #TITLE")
	}
	return Song{Artist: artist, Title: title, Duet: duet}, nil
}
