package server

import (
	"encoding/json"
	"net/http"
	"time"
)

// deckStatusReporter is the minimal interface the handler needs from the
// queue driver. Lets tests inject a fake without standing up a real driver.
type deckStatusReporter interface {
	Status() (ok bool, age time.Duration)
}

// deckOnlineThreshold is the staleness limit beyond which the most recent
// successful probe no longer counts as "online." The driver ticks every 2 s,
// so 10 s == 5 missed ticks: decisive without flapping on transient hiccups.
const deckOnlineThreshold = 10 * time.Second

type deckStatusResponse struct {
	Online             bool     `json:"online"`
	LastSeenSecondsAgo *float64 `json:"lastSeenSecondsAgo"`
}

// DeckStatusHandler returns the Deck's reachability state. With a nil
// reporter (no DeckURL configured — dev mode without a Deck), reports
// online=false / lastSeenSecondsAgo=null so the web client can still
// render a meaningful banner.
func DeckStatusHandler(reporter deckStatusReporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := deckStatusResponse{Online: false}
		if reporter != nil {
			ok, age := reporter.Status()
			if age > 0 {
				secs := age.Seconds()
				resp.LastSeenSecondsAgo = &secs
				resp.Online = ok && age <= deckOnlineThreshold
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:gosec // best-effort response encoding
	})
}
