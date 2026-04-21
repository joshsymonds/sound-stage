package fakeusdx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshsymonds/sound-stage/server/stableid"
	"github.com/joshsymonds/sound-stage/server/txtparse"
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

	parsed, err := txtparse.Parse(path)
	if err != nil {
		writeAddedFalseError(w, http.StatusBadRequest, "parse failed")
		return
	}

	id := stableid.Compute(parsed.Artist, parsed.Title, parsed.Duet)
	entry := songEntry{
		ID:     id,
		Title:  parsed.Title,
		Artist: parsed.Artist,
		Duet:   parsed.Duet,
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
		"title": parsed.Title,
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
