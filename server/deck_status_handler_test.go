package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server"
)

// fakeReporter implements the unexported deckStatusReporter interface
// structurally — Go's type system is happy as long as method shapes match.
type fakeReporter struct {
	ok  bool
	age time.Duration
}

func (f fakeReporter) Status() (bool, time.Duration) {
	return f.ok, f.age
}

func TestDeckStatusHandler(t *testing.T) {
	t.Parallel()

	t.Run("nil reporter reports offline + null lastSeen", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		server.DeckStatusHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/deck-status", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["online"] != false {
			t.Errorf("online = %v, want false", resp["online"])
		}
		if resp["lastSeenSecondsAgo"] != nil {
			t.Errorf("lastSeenSecondsAgo = %v, want nil", resp["lastSeenSecondsAgo"])
		}
	})

	t.Run("recent successful probe reports online", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		server.DeckStatusHandler(fakeReporter{ok: true, age: time.Second}).
			ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/deck-status", nil))
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["online"] != true {
			t.Errorf("online = %v, want true", resp["online"])
		}
		if secs, ok := resp["lastSeenSecondsAgo"].(float64); !ok || secs <= 0 {
			t.Errorf("lastSeenSecondsAgo = %v, want positive number", resp["lastSeenSecondsAgo"])
		}
	})

	t.Run("recent failed probe reports offline", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		server.DeckStatusHandler(fakeReporter{ok: false, age: time.Second}).
			ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/deck-status", nil))
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["online"] != false {
			t.Errorf("online = %v, want false", resp["online"])
		}
		if resp["lastSeenSecondsAgo"] == nil {
			t.Errorf("lastSeenSecondsAgo unexpectedly null")
		}
	})

	t.Run("stale successful probe reports offline", func(t *testing.T) {
		t.Parallel()
		// Last probe was 30s ago — the threshold is 10s, so report offline
		// even though the most recent attempt succeeded.
		rec := httptest.NewRecorder()
		server.DeckStatusHandler(fakeReporter{ok: true, age: 30 * time.Second}).
			ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/deck-status", nil))
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["online"] != false {
			t.Errorf("online = %v, want false (stale)", resp["online"])
		}
	})
}
