package server

import (
	"encoding/json"
	"net/http"

	"github.com/joshsymonds/sound-stage/usdb"
)

// USDBSearcher abstracts the USDB search capability for testing.
type USDBSearcher interface {
	Search(params usdb.SearchParams) ([]usdb.Song, error)
}

// USDBSearchHandler returns search results from USDB.
func USDBSearchHandler(searcher USDBSearcher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		artist := query.Get("artist")
		title := query.Get("title")
		edition := query.Get("edition")

		if artist == "" && title == "" && edition == "" {
			http.Error(w, "at least one search parameter required (artist, title, edition)", http.StatusBadRequest)
			return
		}

		results, err := searcher.Search(usdb.SearchParams{
			Artist:  artist,
			Title:   title,
			Edition: edition,
			Limit:   50, //nolint:mnd // reasonable default for search results
		})
		if err != nil {
			http.Error(w, "USDB search failed", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results) //nolint:gosec // best-effort response encoding
	})
}
