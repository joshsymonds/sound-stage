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

// PlaybackProxyHandler creates a handler that proxies POST requests to the Deck's Pascal API.
func PlaybackProxyHandler(deckURL, endpoint string) http.Handler {
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

		resp, err := http.DefaultClient.Do(proxyReq)
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
func NowPlayingProxyHandler(deckURL string) http.Handler {
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

		resp, err := http.DefaultClient.Do(proxyReq)
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
