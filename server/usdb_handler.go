package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/joshsymonds/sound-stage/usdb"
)

// USDBSearcher abstracts the USDB search capability for testing. Ready
// reports whether the underlying client has logged in; while false the
// handler short-circuits with HTTP 503 so the client knows to retry.
// Search takes the request context so an abandoned search releases the
// upstream goroutine instead of running to USDB's 30s httpTimeout.
type USDBSearcher interface {
	Ready() bool
	Search(ctx context.Context, params usdb.SearchParams) ([]usdb.Song, error)
}

// usdbNotReadyRetryAfter is the Retry-After value sent on 503 responses
// from USDB-gated endpoints. 5 seconds is short enough that the user's
// next interaction will land roughly when login completes.
const usdbNotReadyRetryAfter = "5"

// writeUSDBNotReady writes a 503 with a Retry-After header. Used by all
// USDB-backed handlers to signal "login still in flight".
func writeUSDBNotReady(w http.ResponseWriter) {
	w.Header().Set("Retry-After", usdbNotReadyRetryAfter)
	http.Error(w, "USDB not ready", http.StatusServiceUnavailable)
}

// USDBSearchHandler returns search results from USDB.
func USDBSearchHandler(searcher USDBSearcher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !searcher.Ready() {
			writeUSDBNotReady(w)
			return
		}
		query := r.URL.Query()
		artist := query.Get("artist")
		title := query.Get("title")
		edition := query.Get("edition")

		if artist == "" && title == "" && edition == "" {
			http.Error(w, "at least one search parameter required (artist, title, edition)", http.StatusBadRequest)
			return
		}

		results, err := searcher.Search(r.Context(), usdb.SearchParams{
			Artist:  artist,
			Title:   title,
			Edition: edition,
			Limit:   50, //nolint:mnd // reasonable default for search results
		})
		if err != nil {
			http.Error(w, "USDB search failed", http.StatusBadGateway)
			return
		}
		// Coerce nil → [] so the wire format is always an array; the web
		// client casts the response to USDBResult[] and would crash on null.
		if results == nil {
			results = []usdb.Song{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results) //nolint:gosec // best-effort response encoding
	})
}
