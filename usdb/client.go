// Package usdb provides an authenticated client for the USDB song database.
package usdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const baseURL = "https://usdb.animux.de/"

// httpTimeout is the timeout for all USDB HTTP requests.
const httpTimeout = 30 * time.Second

// Client is a USDB HTTP client. Construction is cheap and offline; call
// Login (or LoginAsync) before invoking any method that hits USDB. Ready
// reports whether a login has succeeded.
type Client struct {
	http     *http.Client
	baseURL  string
	username string
	password string
	ready    atomic.Bool
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

// NewClient constructs a USDB client without contacting the network. The
// returned client reports Ready() == false until Login or LoginAsync
// succeeds. Empty credentials are allowed at construction time so callers
// can build a fully-wired server even when USDB is unconfigured; Login then
// returns a credentials error and Ready stays false.
func NewClient(username, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}
	return &Client{
		http:     &http.Client{Jar: jar, Timeout: httpTimeout},
		baseURL:  baseURL,
		username: username,
		password: password,
	}, nil
}

// Ready reports whether this client has a valid USDB session. Handlers that
// proxy to USDB use this to short-circuit with HTTP 503 before login lands.
func (c *Client) Ready() bool {
	return c.ready.Load()
}

// errMissingCredentials is returned by Login when credentials weren't
// configured. LoginAsync treats this as a permanent decision (do nothing,
// stay not-ready) rather than retrying.
var errMissingCredentials = errors.New("USDB credentials not configured")

// Login performs a single login attempt. On success, Ready() flips to true.
// On any failure, Ready stays false and the error is returned for the caller
// to log or retry.
func (c *Client) Login(ctx context.Context) error {
	if c.username == "" || c.password == "" {
		return errMissingCredentials
	}

	data := url.Values{
		"user":  {c.username},
		"pass":  {c.password},
		"login": {"Login"},
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading login response: %w", err)
	}
	if strings.Contains(string(body), "Login or Password invalid") {
		return errors.New("invalid USDB credentials")
	}

	c.ready.Store(true)
	return nil
}

// loginBackoffSchedule controls LoginAsync's wait between attempts. The
// final value is reused indefinitely so the loop never gives up while the
// process is alive — operators see slow log growth on permanent auth
// failure rather than a one-shot abandonment.
//
//nolint:gochecknoglobals // configuration constant; can't be const because slice.
var loginBackoffSchedule = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
	30 * time.Second,
}

// LoginAsync runs Login in a loop with exponential backoff until success or
// context cancellation. Returns nil on successful login (Ready() == true),
// ctx.Err() on cancellation. With no credentials configured, returns nil
// immediately and Ready() stays false — the caller's startup path doesn't
// loop for nothing.
func (c *Client) LoginAsync(ctx context.Context) error {
	return c.loginAsync(ctx, loginBackoffSchedule)
}

func (c *Client) loginAsync(ctx context.Context, backoff []time.Duration) error {
	for attempt := 0; ; attempt++ {
		err := c.Login(ctx)
		if err == nil {
			return nil
		}
		if errors.Is(err, errMissingCredentials) {
			// No credentials configured: nothing to retry.
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("usdb login: %w", err)
		}

		wait := backoff[len(backoff)-1]
		if attempt < len(backoff) {
			wait = backoff[attempt]
		}
		slog.Default().WarnContext(ctx, "usdb login failed; will retry",
			"attempt", attempt+1, "wait", wait, "error", err)

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("usdb login: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

// Search queries USDB for songs matching the given params. The context
// scopes the upstream HTTP call — handlers should pass r.Context() so an
// abandoned search releases the goroutine instead of running to the 30s
// httpTimeout.
func (c *Client) Search(ctx context.Context, params SearchParams) ([]Song, error) {
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
		ctx,
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
func (c *Client) GetSongTxt(ctx context.Context, songID int) (string, error) {
	return c.getSongTxt(ctx, songID, func(d time.Duration) {
		timer := time.NewTimer(d)
		<-timer.C
	})
}

// getSongTxt is the internal implementation that accepts a sleep function for testing.
func (c *Client) getSongTxt(ctx context.Context, songID int, sleepFn func(time.Duration)) (string, error) {
	txtURL := fmt.Sprintf("%s?link=gettxt&id=%d", c.baseURL, songID)

	for attempt := range maxTxtRetries {
		data := url.Values{"wd": {"1"}}
		req, err := http.NewRequestWithContext(
			ctx,
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
func (c *Client) GetSongDetails(ctx context.Context, songID int) (*SongDetails, error) {
	req, err := http.NewRequestWithContext(
		ctx,
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
