// Package server implements the SoundStage HTTP server.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Song represents a song in the library, matching the frontend Song interface.
type Song struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Edition string `json:"edition,omitempty"`
	Year    int    `json:"year,omitempty"`
}

// SongsHandler returns an http.Handler that scans the library directory
// and returns a JSON array of songs.
func SongsHandler(libraryDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		songs, scanErr := scanLibrary(libraryDir)
		if scanErr != nil {
			slog.Error("scanning library", "error", scanErr)
			http.Error(w, "failed to scan library", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if encodeErr := json.NewEncoder(w).Encode(songs); encodeErr != nil {
			slog.Error("encoding songs response", "error", encodeErr)
		}
	})
}

func scanLibrary(dir string) ([]Song, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading library directory: %w", err)
	}

	songs := make([]Song, 0, len(entries))
	songID := 1

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		txtPath := filepath.Join(dir, entry.Name(), "song.txt")
		if song, parseErr := parseSongFile(txtPath, songID); parseErr == nil {
			songs = append(songs, song)
			songID++
		}
	}

	return songs, nil
}

func parseSongFile(path string, id int) (Song, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is constructed from library dir + entry name, not user input
	if err != nil {
		return Song{}, fmt.Errorf("reading song file: %w", err)
	}

	headers := parseHeaders(string(data))

	year, _ := strconv.Atoi(headerVal(headers, "YEAR")) //nolint:errcheck // invalid year is fine, defaults to 0

	return Song{
		ID:      id,
		Title:   headerVal(headers, "TITLE"),
		Artist:  headerVal(headers, "ARTIST"),
		Edition: headerVal(headers, "EDITION"),
		Year:    year,
	}, nil
}

type hdr struct {
	key   string
	value string
}

func parseHeaders(txt string) []hdr {
	var headers []hdr
	for _, line := range strings.Split(txt, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			break
		}
		parts := strings.SplitN(line[1:], ":", 2)
		if len(parts) == 2 {
			headers = append(headers, hdr{
				key:   strings.TrimSpace(parts[0]),
				value: strings.TrimSpace(parts[1]),
			})
		}
	}
	return headers
}

func headerVal(headers []hdr, key string) string {
	for _, h := range headers {
		if strings.EqualFold(h.key, key) {
			return h.value
		}
	}
	return ""
}
