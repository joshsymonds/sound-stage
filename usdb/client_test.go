package usdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetSongTxt_RateLimitRetry(t *testing.T) {
	t.Parallel()

	rateLimitHTML := readFixture(t, "gettxt_ratelimit.html")
	successHTML := readFixture(t, "gettxt_response.html")

	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := requestCount.Add(1)
		if n == 1 {
			w.Write([]byte(rateLimitHTML))
		} else {
			w.Write([]byte(successHTML))
		}
	}))
	defer srv.Close()

	client := &Client{
		http:    srv.Client(),
		baseURL: srv.URL + "/",
	}

	var sleepCalled bool
	var sleepDuration time.Duration
	fakeSleep := func(d time.Duration) {
		sleepCalled = true
		sleepDuration = d
	}

	txt, err := client.getSongTxt(context.Background(), 12345, fakeSleep)
	if err != nil {
		t.Fatalf("getSongTxt: %v", err)
	}
	if txt == "" {
		t.Fatal("getSongTxt returned empty string")
	}

	if !sleepCalled {
		t.Error("expected sleep to be called for rate limit wait")
	}
	// Fixture has time = 24, plus 1s buffer = 25s.
	if sleepDuration != 25*time.Second {
		t.Errorf("sleep duration = %v, want 25s", sleepDuration)
	}

	if requestCount.Load() != 2 {
		t.Errorf("request count = %d, want 2", requestCount.Load())
	}

	// Verify the returned txt is valid UltraStar content.
	headers, _ := parseTxt(txt)
	assertHeader(t, headers, "ARTIST", "Queen")
}

func TestGetSongTxt_RateLimitExhausted(t *testing.T) {
	t.Parallel()

	rateLimitHTML := readFixture(t, "gettxt_ratelimit.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(rateLimitHTML))
	}))
	defer srv.Close()

	client := &Client{
		http:    srv.Client(),
		baseURL: srv.URL + "/",
	}
	fakeSleep := func(_ time.Duration) {}

	_, err := client.getSongTxt(context.Background(), 12345, fakeSleep)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
}
