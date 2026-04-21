// Package server implements the SoundStage HTTP server.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

	logger := slog.Default()
	songs := make([]Song, 0, len(entries))
	seenIDs := make(map[string]string, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		txtPath := filepath.Join(dir, entry.Name(), "song.txt")
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
		songs = append(songs, song)
	}

	return songs, nil
}

func parseSongFile(path string) (Song, error) {
	parsed, err := txtparse.Parse(path)
	if err != nil {
		return Song{}, fmt.Errorf("parse: %w", err)
	}

	edition, year := readOptionalHeaders(path)

	return Song{
		ID:      stableid.Compute(parsed.Artist, parsed.Title, parsed.Duet),
		Title:   parsed.Title,
		Artist:  parsed.Artist,
		Duet:    parsed.Duet,
		Edition: edition,
		Year:    year,
	}, nil
}

// readOptionalHeaders extracts #EDITION and #YEAR from the .txt header block.
// Streams with a bufio.Scanner and exits at the first non-# line so we don't
// pull the (typically 10-100 KB) notes section into memory for every song.
// Missing values return "" and 0; errors are swallowed — the song is still
// usable without them.
func readOptionalHeaders(path string) (edition string, year int) {
	file, err := os.Open(path) //nolint:gosec // path constructed from library dir + entry name
	if err != nil {
		return "", 0
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") {
			return edition, year
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(line[1:colon]))
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "EDITION":
			edition = val
		case "YEAR":
			y, convErr := strconv.Atoi(val)
			if convErr == nil {
				year = y
			}
		}
	}
	return edition, year
}
