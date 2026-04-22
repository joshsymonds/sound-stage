package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const proxyTimeout = 5 * time.Second
const maxProxyResponse = 10 << 20 // 10 MB

// newDeckProxyClient builds an http.Client tuned for user-facing Deck-bound
// requests (pause/resume/now-playing proxies + notifyDeck). One per
// Handler instance — shares a connection pool across the three call sites
// without polluting http.DefaultClient if the Deck misbehaves. Distinct
// from QueueDriver's tighter probe timeout (1.5s).
func newDeckProxyClient() *http.Client {
	return &http.Client{
		Timeout: proxyTimeout + time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        deckMaxIdleConns,
			MaxIdleConnsPerHost: deckMaxIdleConnsPerHost,
			IdleConnTimeout:     deckIdleConnTimeout,
		},
	}
}

// PlaybackProxyHandler creates a handler that proxies POST requests to the
// Deck's Pascal API. The caller-provided client is reused for the proxy +
// notifyDeck pool sharing.
func PlaybackProxyHandler(client *http.Client, deckURL, endpoint string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deckURL == "" {
			http.Error(w, "deck not configured", http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), proxyTimeout)
		defer cancel()

		targetURL := deckURL + endpoint
		proxyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, nil)
		if err != nil {
			http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
			return
		}

		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, "deck unreachable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(resp.Body, maxProxyResponse))
	})
}

// NowPlayingProxyHandler proxies GET /now-playing from the Deck.
func NowPlayingProxyHandler(client *http.Client, deckURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deckURL == "" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, "null")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), proxyTimeout)
		defer cancel()

		targetURL := deckURL + "/now-playing"
		proxyReq, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, "null")
			return
		}

		resp, err := client.Do(proxyReq)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, "null")
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(resp.Body, maxProxyResponse))
	})
}
