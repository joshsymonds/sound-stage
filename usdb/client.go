// Package usdb provides an authenticated client for the USDB song database.
package usdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://usdb.animux.de/"

// httpTimeout is the timeout for all USDB HTTP requests.
const httpTimeout = 30 * time.Second

// Client is an authenticated USDB HTTP client.
type Client struct {
	http    *http.Client
	baseURL string
}

// Song represents a search result from USDB.
type Song struct {
	ID       int    `json:"id"`
	Artist   string `json:"artist"`
	Title    string `json:"title"`
	Language string `json:"language"`
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
	Artist  string
	Title   string
	Edition string
	Limit   int
}

// NewClient logs in to USDB and returns an authenticated client.
func NewClient(username, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	client := &Client{
		http: &http.Client{
			Jar:     jar,
			Timeout: httpTimeout,
		},
		baseURL: baseURL,
	}

	data := url.Values{
		"user":  {username},
		"pass":  {password},
		"login": {"Login"},
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		client.baseURL,
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
	if params.Edition != "" {
		data.Set("edition", params.Edition)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		c.baseURL+"?link=list",
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

// maxTxtRetries is the maximum number of times to retry a rate-limited gettxt request.
const maxTxtRetries = 5

// GetSongTxt downloads the raw UltraStar txt for a song.
// If USDB returns a rate-limit page, it waits and retries automatically.
func (c *Client) GetSongTxt(songID int) (string, error) {
	return c.getSongTxt(songID, func(d time.Duration) {
		timer := time.NewTimer(d)
		<-timer.C
	})
}

// getSongTxt is the internal implementation that accepts a sleep function for testing.
func (c *Client) getSongTxt(songID int, sleepFn func(time.Duration)) (string, error) {
	txtURL := fmt.Sprintf("%s?link=gettxt&id=%d", c.baseURL, songID)

	for attempt := range maxTxtRetries {
		data := url.Values{"wd": {"1"}}
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			txtURL,
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

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("reading txt response: %w", err)
		}

		txt, err := extractTextarea(string(body))
		if err == nil {
			return txt, nil
		}

		var rlErr *RateLimitError
		if !errors.As(err, &rlErr) {
			return "", fmt.Errorf("fetching song txt (response len=%d): %w", len(body), err)
		}

		if attempt == maxTxtRetries-1 {
			return "", fmt.Errorf("fetching song txt: still rate limited after %d retries", maxTxtRetries)
		}

		// Add 1s buffer to the server's requested wait time.
		wait := rlErr.Wait + time.Second
		fmt.Fprintf(os.Stderr, "  Rate limited, waiting %s before retry...\n", wait)
		sleepFn(wait)
	}

	// Unreachable, but satisfies the compiler.
	return "", fmt.Errorf("fetching song txt: exhausted retries")
}

// GetSongDetails fetches the detail page and extracts metadata + YouTube IDs from comments.
func (c *Client) GetSongDetails(songID int) (*SongDetails, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("%s?link=detail&id=%d", c.baseURL, songID),
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

// FetchCover returns the cover image as a stream plus its Content-Type.
// The caller MUST Close the returned ReadCloser. Returns os.ErrNotExist
// when USDB responds 404 (song has no cover); other non-200 statuses
// surface as a wrapped error.
func (c *Client) FetchCover(ctx context.Context, songID int) (io.ReadCloser, string, error) {
	coverURL := fmt.Sprintf("%sdata/cover/%d.jpg", c.baseURL, songID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating cover request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("cover request: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, "", os.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("cover returned HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return resp.Body, contentType, nil
}

// DownloadCover saves the cover image for a song to <destDir>/cover.jpg.
func (c *Client) DownloadCover(songID int, destDir string) error {
	body, _, err := c.FetchCover(context.Background(), songID)
	if err != nil {
		return err
	}
	defer body.Close()

	destPath := filepath.Join(destDir, "cover.jpg")
	file, err := os.Create(destPath) //nolint:gosec // path constructed from song metadata
	if err != nil {
		return fmt.Errorf("creating cover file: %w", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, body); err != nil {
		return fmt.Errorf("writing cover file: %w", err)
	}
	return nil
}
