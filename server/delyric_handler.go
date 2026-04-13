package server

import (
	"context"
	"io"
	"net/http"
	"time"
)

const delyricTimeout = 10 * time.Minute

// DelyricProxyHandler proxies requests to the delyric worker at the given base URL.
// Returns 503 if the worker is unreachable.
func DelyricProxyHandler(delyricURL, method, endpoint string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delyricURL == "" {
			http.Error(w, "delyric worker not configured", http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), delyricTimeout)
		defer cancel()

		targetURL := delyricURL + endpoint
		proxyReq, err := http.NewRequestWithContext(ctx, method, targetURL, r.Body)
		if err != nil {
			http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
			return
		}

		if r.Header.Get("Content-Type") != "" {
			proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		}

		resp, err := http.DefaultClient.Do(proxyReq)
		if err != nil {
			http.Error(w, "delyric worker unreachable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(resp.Body, maxProxyResponse))
	})
}
