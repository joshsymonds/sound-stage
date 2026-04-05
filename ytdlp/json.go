// Package ytdlp wraps yt-dlp for downloading audio and video from YouTube.
package ytdlp

import (
	"encoding/json"
	"fmt"
)

func extractURLFromJSON(data []byte) (string, error) {
	var result struct {
		URL        string `json:"url"`
		WebpageURL string `json:"webpage_url"` //nolint:tagliatelle // yt-dlp output uses snake_case
		ID         string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing yt-dlp output: %w", err)
	}

	if result.WebpageURL != "" {
		return result.WebpageURL, nil
	}
	if result.URL != "" {
		return result.URL, nil
	}
	if result.ID != "" {
		return "https://www.youtube.com/watch?v=" + result.ID, nil
	}
	return "", fmt.Errorf("no URL found in yt-dlp output")
}
