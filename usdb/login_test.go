package usdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewClient_DoesNotHitNetwork verifies that constructing a client is a
// pure-local operation: no HTTP request happens until Login is called.
// This is what lets cmd/serve.go bind the listener before authenticating.
func TestNewClient_DoesNotHitNetwork(t *testing.T) {
	t.Parallel()
	var hit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		hit.Add(1)
	}))
	defer srv.Close()

	c, err := NewClient("u", "p")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.baseURL = srv.URL + "/"

	if c.Ready() {
		t.Error("Ready() should be false on a freshly constructed client")
	}
	if got := hit.Load(); got != 0 {
		t.Errorf("expected 0 HTTP requests after NewClient, got %d", got)
	}
}

func TestLogin_SuccessSetsReady(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>welcome</html>"))
	}))
	defer srv.Close()

	c, err := NewClient("u", "p")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL + "/"

	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !c.Ready() {
		t.Error("Ready() should be true after a successful Login")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Login or Password invalid"))
	}))
	defer srv.Close()

	c, _ := NewClient("u", "p")
	c.baseURL = srv.URL + "/"

	if err := c.Login(context.Background()); err == nil {
		t.Fatal("expected Login to fail on invalid-credentials response")
	}
	if c.Ready() {
		t.Error("Ready() should remain false after a failed Login")
	}
}

func TestLogin_MissingCredentials(t *testing.T) {
	t.Parallel()
	c, _ := NewClient("", "")

	err := c.Login(context.Background())
	if err == nil {
		t.Fatal("expected Login with empty credentials to error")
	}
	if c.Ready() {
		t.Error("Ready() should be false when credentials are missing")
	}
}

func TestLoginAsync_NoCredentialsReturnsImmediately(t *testing.T) {
	t.Parallel()
	var hit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		hit.Add(1)
	}))
	defer srv.Close()

	c, _ := NewClient("", "")
	c.baseURL = srv.URL + "/"

	done := make(chan error, 1)
	go func() { done <- c.LoginAsync(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("LoginAsync with no creds: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("LoginAsync should return immediately when credentials are missing")
	}
	if c.Ready() {
		t.Error("Ready() should remain false")
	}
	if got := hit.Load(); got != 0 {
		t.Errorf("expected 0 HTTP requests, got %d", got)
	}
}

func TestLoginAsync_RetriesUntilSuccess(t *testing.T) {
	t.Parallel()
	var hit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := hit.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("welcome"))
	}))
	defer srv.Close()

	c, _ := NewClient("u", "p")
	c.baseURL = srv.URL + "/"

	fast := []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	if err := c.loginAsync(context.Background(), fast); err != nil {
		t.Fatalf("loginAsync: %v", err)
	}
	if !c.Ready() {
		t.Error("Ready() should be true after eventual success")
	}
	if got := hit.Load(); got != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", got)
	}
}

func TestLoginAsync_CancelStopsRetry(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := NewClient("u", "p")
	c.baseURL = srv.URL + "/"

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- c.loginAsync(ctx, []time.Duration{50 * time.Millisecond, 50 * time.Millisecond})
	}()

	// Let one attempt land, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected context error, got nil")
		}
	case <-time.After(time.Second):
		t.Fatal("loginAsync did not return after context cancel")
	}
	if c.Ready() {
		t.Error("Ready() should remain false after a canceled retry loop")
	}
}
