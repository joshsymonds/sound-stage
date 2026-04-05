package usdb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PreparedSong holds the parsed song info ready for media download.
type PreparedSong struct {
	Artist     string
	Title      string
	YouTubeURL string
	AudioFile  string
	VideoFile  string
	TxtPath    string
}

// PrepareSong parses the UltraStar txt, writes it to disk with corrected media
// references, and returns info needed for media downloads.
func PrepareSong(rawTxt string, details *SongDetails, songDir string) (*PreparedSong, error) {
	if err := os.MkdirAll(songDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating song directory: %w", err)
	}

	headers, body := parseTxt(rawTxt)

	// Determine artist/title from txt headers, fall back to detail page
	artist := headerValue(headers, "ARTIST")
	if artist == "" {
		artist = details.Artist
	}
	title := headerValue(headers, "TITLE")
	if title == "" {
		title = details.Title
	}

	audioFile := "audio.mp3"
	videoFile := "video.mp4"

	// Rewrite media headers to point to local files
	headers = setHeader(headers, "MP3", audioFile)
	headers = setHeader(headers, "VIDEO", videoFile)
	headers = setHeader(headers, "COVER", "cover.jpg")

	// Write the corrected txt
	txtContent := formatTxt(headers, body)
	txtPath := filepath.Join(songDir, "song.txt")
	if err := os.WriteFile(txtPath, []byte(txtContent), 0o600); err != nil {
		return nil, fmt.Errorf("writing song txt: %w", err)
	}

	// Find best YouTube URL
	var youtubeURL string
	if len(details.YouTubeIDs) > 0 {
		youtubeURL = "https://www.youtube.com/watch?v=" + details.YouTubeIDs[0]
	}

	return &PreparedSong{
		Artist:     artist,
		Title:      title,
		YouTubeURL: youtubeURL,
		AudioFile:  audioFile,
		VideoFile:  videoFile,
		TxtPath:    txtPath,
	}, nil
}

type header struct {
	key   string
	value string
}

func parseTxt(txt string) ([]header, string) {
	lines := strings.Split(txt, "\n")
	var headers []header
	var bodyStart int

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line[1:], ":", 2)
			if len(parts) == 2 {
				headers = append(headers, header{
					key:   strings.TrimSpace(parts[0]),
					value: strings.TrimSpace(parts[1]),
				})
				continue
			}
		}
		bodyStart = i
		break
	}

	body := strings.Join(lines[bodyStart:], "\n")
	return headers, body
}

func headerValue(headers []header, key string) string {
	for _, h := range headers {
		if strings.EqualFold(h.key, key) {
			return h.value
		}
	}
	return ""
}

func setHeader(headers []header, key, value string) []header {
	for i, h := range headers {
		if strings.EqualFold(h.key, key) {
			headers[i].value = value
			return headers
		}
	}
	return append(headers, header{key: key, value: value})
}

func formatTxt(headers []header, body string) string {
	var b strings.Builder
	for _, h := range headers {
		fmt.Fprintf(&b, "#%s:%s\n", h.key, h.value)
	}
	b.WriteString(body)
	return b.String()
}
