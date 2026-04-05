// Package archive tracks which songs have been downloaded to avoid re-downloading.
package archive

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const archiveFile = ".downloaded.txt"

// LoadDownloaded reads the archive and returns the set of all downloaded song IDs.
// Returns an empty set if the archive does not exist yet.
func LoadDownloaded(outputDir string) (map[int]struct{}, error) {
	return readArchive(outputDir)
}

// IsDownloaded checks if a song ID has already been downloaded.
func IsDownloaded(outputDir string, songID int) bool {
	ids, err := readArchive(outputDir)
	if err != nil {
		return false
	}
	_, found := ids[songID]
	return found
}

// MarkDownloaded appends a song ID to the download archive.
func MarkDownloaded(outputDir string, songID int) error {
	archivePath := filepath.Join(outputDir, archiveFile)

	file, err := os.OpenFile( //nolint:gosec // path constructed from our output dir config
		archivePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer file.Close()

	if _, writeErr := fmt.Fprintf(file, "%d\n", songID); writeErr != nil {
		return fmt.Errorf("writing to archive: %w", writeErr)
	}

	return nil
}

func readArchive(outputDir string) (map[int]struct{}, error) {
	archivePath := filepath.Join(outputDir, archiveFile)

	file, err := os.Open(archivePath) //nolint:gosec // path is from our config
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[int]struct{}), nil
		}
		return nil, fmt.Errorf("opening archive: %w", err)
	}
	defer file.Close()

	ids := make(map[int]struct{})
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if parsed, parseErr := strconv.Atoi(line); parseErr == nil {
			ids[parsed] = struct{}{}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return ids, fmt.Errorf("reading archive: %w", scanErr)
	}

	return ids, nil
}
