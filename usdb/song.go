package usdb

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// youtubeIDValidRegex matches a valid 11-character YouTube video ID.
var youtubeIDValidRegex = regexp.MustCompile(`^[0-9A-Za-z_-]{11}$`)

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

	// Determine artist/title from txt headers, fall back to detail page.
	// Normalize to replace smart quotes and HTML entities from USDB.
	artist := NormalizeText(headerValue(headers, "ARTIST"))
	if artist == "" {
		artist = NormalizeText(details.Artist)
	}
	title := NormalizeText(headerValue(headers, "TITLE"))
	if title == "" {
		title = NormalizeText(details.Title)
	}
	headers = setHeader(headers, "ARTIST", artist)
	headers = setHeader(headers, "TITLE", title)

	audioFile := "audio.webm"
	videoFile := "video.webm"

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

	// Find best YouTube URL, skipping invalid IDs
	var youtubeURL string
	for _, id := range details.YouTubeIDs {
		if youtubeIDValidRegex.MatchString(id) {
			youtubeURL = "https://www.youtube.com/watch?v=" + id
			break
		}
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

// SanitizePath removes characters that are invalid in file/directory names
// and normalizes Unicode text for safe filesystem paths.
func SanitizePath(input string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	return strings.TrimSpace(replacer.Replace(NormalizeText(input)))
}

// NormalizeText replaces Unicode smart quotes and other typographic characters
// with their ASCII equivalents, and decodes HTML entities. USDB metadata often
// contains these, and UltraStar players may not handle them correctly.
func NormalizeText(text string) string {
	text = html.UnescapeString(text)
	replacer := strings.NewReplacer(
		"\u2018", "'", // left single quotation mark
		"\u2019", "'", // right single quotation mark
		"\u201A", "'", // single low-9 quotation mark
		"\u201B", "'", // single high-reversed-9 quotation mark
		"\u201C", "\"", // left double quotation mark
		"\u201D", "\"", // right double quotation mark
		"\u201E", "\"", // double low-9 quotation mark
		"\u201F", "\"", // double high-reversed-9 quotation mark
		"\u2032", "'", // prime
		"\u2033", "\"", // double prime
		"\u2013", "-", // en dash
		"\u2014", "-", // em dash
		"\u2026", "...", // ellipsis
	)
	return replacer.Replace(text)
}
