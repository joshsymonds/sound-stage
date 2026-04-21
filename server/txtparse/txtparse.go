// Package txtparse parses USDX .txt song files to extract metadata used by
// both sound-stage's library scanner and the fakeusdx refresh handler.
//
// The parser recognizes:
//   - #ARTIST: and #TITLE: header tags (required)
//   - Duet marker via #DUETSINGERP1: / #DUETSINGERP2: headers OR standalone
//     "P1" / "P2" lines anywhere in the file.
//
// It is deliberately minimal — it does not validate BPM, notes, or any other
// USDX constructs. Callers can consult the raw file for richer parsing if
// they need it.
package txtparse

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Song is the minimal metadata subset this package extracts. It intentionally
// does not carry computed IDs, filesystem paths, or anything else a caller
// might layer on top. Edition and Year are optional — Parse returns zero
// values when the corresponding header is absent.
type Song struct {
	Artist  string
	Title   string
	Duet    bool
	Edition string
	Year    int
}

// Parse reads the .txt file at path and extracts its metadata.
// Returns an error when the file is unreadable, when required headers
// (#ARTIST, #TITLE) are missing, or when either required header is empty.
func Parse(path string) (Song, error) {
	data, err := os.ReadFile(path) //nolint:gosec // .txt paths are derived from a trusted library root
	if err != nil {
		return Song{}, fmt.Errorf("read %s: %w", path, err)
	}

	song, scanErr := parseBytes(data)
	if scanErr != nil {
		return Song{}, fmt.Errorf("scan %s: %w", path, scanErr)
	}

	if song.Artist == "" || song.Title == "" {
		return Song{}, errors.New("missing #ARTIST or #TITLE")
	}
	return song, nil
}

func parseBytes(data []byte) (Song, error) {
	var song Song
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		applyLine(line, &song)
	}
	return song, scanner.Err() //nolint:wrapcheck // scanner.Err carries position context already
}

// applyLine updates song with the metadata from a single .txt line.
// Standalone "P1"/"P2" markers flip duet. Lines starting with "#" are
// parsed as KEY:VALUE headers; other lines are ignored.
func applyLine(line string, song *Song) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "P1" || trimmed == "P2" {
		song.Duet = true
		return
	}
	if !strings.HasPrefix(line, "#") {
		return
	}
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return
	}
	key := strings.ToUpper(strings.TrimSpace(line[1:colon]))
	val := strings.TrimSpace(line[colon+1:])
	switch key {
	case "ARTIST":
		song.Artist = val
	case "TITLE":
		song.Title = val
	case "DUETSINGERP1", "DUETSINGERP2":
		song.Duet = true
	case "EDITION":
		song.Edition = val
	case "YEAR":
		if y, err := strconv.Atoi(val); err == nil {
			song.Year = y
		}
	}
}
