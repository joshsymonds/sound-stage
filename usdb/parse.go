package usdb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// songRowRegex matches song rows in the USDB search results HTML.
// The HTML structure uses onclick="show_detail(N)" on <td> elements.
var songRowRegex = regexp.MustCompile(
	`<tr class="list_tr\d"\s+data-songid="(\d+)"\s+data-lastchange="\d+"[^>]*?>` +
		`.*?` + // sample/cover columns
		`<td[^>]*?>([^<]*?)</td>\n` + // artist
		`<td[^>]*?><a href=[^>]*?>([^<]*?)</td>\n` + // title (wrapped in <a>)
		`<td[^>]*?>[^<]*?</td>\n` + // genre
		`<td[^>]*?>[^<]*?</td>\n` + // year
		`<td[^>]*?>[^<]*?</td>\n` + // edition
		`<td[^>]*?>[^<]*?</td>\n` + // golden notes
		`<td[^>]*?>([^<]*?)</td>\n`, // language
)

// parseSearchResults extracts songs from USDB search results HTML.
func parseSearchResults(html string) []Song {
	matches := songRowRegex.FindAllStringSubmatch(html, -1)
	if matches == nil {
		return nil
	}

	songs := make([]Song, 0, len(matches))
	for _, match := range matches {
		id, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		songs = append(songs, Song{
			ID:       id,
			Artist:   strings.TrimSpace(match[2]),
			Title:    strings.TrimSpace(match[3]),
			Language: strings.TrimSpace(match[4]),
		})
	}
	return songs
}

// textareaRegex extracts content from a <textarea> tag.
var textareaRegex = regexp.MustCompile(`(?s)<textarea[^>]*>(.*?)</textarea>`)

// extractTextarea pulls the UltraStar txt content from a USDB gettxt response.
func extractTextarea(html string) (string, error) {
	match := textareaRegex.FindStringSubmatch(html)
	if match == nil {
		return "", fmt.Errorf("no textarea found in response (song may not exist)")
	}
	return strings.TrimSpace(match[1]), nil
}

// artistTitleRegex extracts artist and title from the detail page header.
var artistTitleRegex = regexp.MustCompile(
	`<tr class="list_head">\s*<td>([^<]+)</td><td>([^<]+)</td>`,
)

// parseDetailPage extracts song metadata and YouTube IDs from a USDB detail page.
func parseDetailPage(html string, songID int) *SongDetails {
	details := &SongDetails{
		HasCover: strings.Contains(html, fmt.Sprintf("data/cover/%d.jpg", songID)),
	}

	// Extract artist and title from the detail table header row.
	if match := artistTitleRegex.FindStringSubmatch(html); match != nil {
		details.Artist = strings.TrimSpace(match[1])
		details.Title = strings.TrimSpace(match[2])
	}

	// Multi-stage YouTube extraction from comments, matching usdb_syncer's approach:
	// 1. <embed src="..."> tags (old Flash embeds)
	// 2. <iframe src="..."> tags
	// 3. <a href="..."> tags
	// 4. Plain text URLs (regex fallback)
	seen := make(map[string]bool)

	// Stage 1: <embed> tags.
	for _, match := range embedSrcRegex.FindAllStringSubmatch(html, -1) {
		if id := extractYouTubeID(match[1]); id != "" && !seen[id] {
			details.YouTubeIDs = append(details.YouTubeIDs, id)
			seen[id] = true
		}
	}

	// Stage 2: <iframe> tags.
	for _, match := range iframeSrcRegex.FindAllStringSubmatch(html, -1) {
		if id := extractYouTubeID(match[1]); id != "" && !seen[id] {
			details.YouTubeIDs = append(details.YouTubeIDs, id)
			seen[id] = true
		}
	}

	// Stage 3: <a href="..."> tags.
	for _, match := range anchorHrefRegex.FindAllStringSubmatch(html, -1) {
		if id := extractYouTubeID(match[1]); id != "" && !seen[id] {
			details.YouTubeIDs = append(details.YouTubeIDs, id)
			seen[id] = true
		}
	}

	// Stage 4: Plain text URLs.
	for _, rawURL := range videoURLRegex.FindAllString(html, -1) {
		if id := extractYouTubeID(rawURL); id != "" && !seen[id] {
			details.YouTubeIDs = append(details.YouTubeIDs, id)
			seen[id] = true
		}
	}

	return details
}

// HTML tag extraction regexes for multi-stage comment parsing.
var (
	embedSrcRegex   = regexp.MustCompile(`(?i)<embed[^>]+src="([^"]+)"`)
	iframeSrcRegex  = regexp.MustCompile(`(?i)<iframe[^>]+src="([^"]+)"`)
	anchorHrefRegex = regexp.MustCompile(`(?i)<a[^>]+href="([^"]+)"`)
	// videoURLRegex matches YouTube and other video platform URLs in plain text.
	videoURLRegex = regexp.MustCompile(
		`(?:https?://)?(?:www\.)?(?:m\.)?` +
			`(?:youtube\.com|youtube-nocookie\.com|youtu\.be)/\S+`,
	)
)

// youtubeIDRegex extracts the 11-character YouTube video ID from various URL formats.
// Uses \S*? (non-greedy) to avoid consuming the v= parameter.
// Terminators include < for URLs embedded in HTML.
var youtubeIDRegex = regexp.MustCompile(
	`(?i)(?:https?://)?(?:www\.)?(?:m\.)?` +
		`(?:youtube\.com/|youtube-nocookie\.com/|youtu\.be)` +
		`\S*?(?:/|%3D|v=|vi=)` +
		`([0-9A-Za-z_-]{11})` +
		`(?:[%#?&<]|$)`,
)

// extractYouTubeID extracts the YouTube video ID from a URL, or returns empty string.
func extractYouTubeID(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	match := youtubeIDRegex.FindStringSubmatch(rawURL)
	if match == nil {
		return ""
	}
	return match[1]
}
