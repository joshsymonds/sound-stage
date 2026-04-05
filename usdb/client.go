// Package usdb provides an authenticated client for the USDB song database.
package usdb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const baseURL = "https://usdb.animux.de/"

// Client is an authenticated USDB HTTP client.
type Client struct {
	http *http.Client
}

// Song represents a search result from USDB.
type Song struct {
	ID       int
	Artist   string
	Title    string
	Language string
}

// SongDetails contains metadata from a song's detail page.
type SongDetails struct {
	Artist     string
	Title      string
	HasCover   bool
	YouTubeIDs []string
}

// SearchParams controls a USDB song search.
type SearchParams struct {
	Artist string
	Title  string
	Limit  int
}

// NewClient logs in to USDB and returns an authenticated client.
func NewClient(username, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	client := &Client{
		http: &http.Client{Jar: jar},
	}

	data := url.Values{
		"user":  {username},
		"pass":  {password},
		"login": {"Login"},
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		baseURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading login response: %w", err)
	}

	if strings.Contains(string(body), "Login or Password invalid") {
		return nil, fmt.Errorf("invalid USDB credentials")
	}

	return client, nil
}

// Search queries USDB for songs matching the given params.
func (c *Client) Search(params SearchParams) ([]Song, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 25
	}

	data := url.Values{
		"order": {"lastchange"},
		"ud":    {"desc"},
		"limit": {strconv.Itoa(limit)},
		"start": {"0"},
	}
	if params.Artist != "" {
		data.Set("interpret", params.Artist)
	}
	if params.Title != "" {
		data.Set("title", params.Title)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		baseURL+"?link=list",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating search request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading search response: %w", err)
	}

	return parseSearchResults(string(body)), nil
}

// GetSongTxt downloads the raw UltraStar txt for a song.
func (c *Client) GetSongTxt(songID int) (string, error) {
	data := url.Values{"wd": {"1"}}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		fmt.Sprintf("%s?link=gettxt&id=%d", baseURL, songID),
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("creating gettxt request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gettxt request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading txt response: %w", err)
	}

	return extractTextarea(string(body))
}

// GetSongDetails fetches the detail page and extracts metadata + YouTube IDs from comments.
func (c *Client) GetSongDetails(songID int) (*SongDetails, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("%s?link=detail&id=%d", baseURL, songID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("creating detail request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("detail request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading detail response: %w", err)
	}

	return parseDetailPage(string(body), songID), nil
}

// DownloadCover saves the cover image for a song.
func (c *Client) DownloadCover(songID int, destDir string) error {
	coverURL := fmt.Sprintf("%sdata/cover/%d.jpg", baseURL, songID)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, coverURL, nil)
	if err != nil {
		return fmt.Errorf("creating cover request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cover request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cover returned HTTP %d", resp.StatusCode)
	}

	destPath := filepath.Join(destDir, "cover.jpg")

	file, err := os.Create(destPath) //nolint:gosec // path constructed from song metadata
	if err != nil {
		return fmt.Errorf("creating cover file: %w", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("writing cover file: %w", err)
	}

	return nil
}
