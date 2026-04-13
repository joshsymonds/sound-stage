package server

import (
	"encoding/json"
	"net/http"
)

type queueAddRequest struct {
	SongID  int    `json:"songId"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Edition string `json:"edition"`
	Year    int    `json:"year"`
	Guest   string `json:"guest"`
}

// QueueListHandler returns the current queue as JSON.
func QueueListHandler(queue *Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entries := queue.List()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries) //nolint:gosec // best-effort response encoding
	})
}

// QueueAddHandler adds a song to the queue for a guest.
func QueueAddHandler(queue *Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req queueAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Guest == "" {
			http.Error(w, "guest name is required", http.StatusBadRequest)
			return
		}

		if req.Title == "" {
			http.Error(w, "song title is required", http.StatusBadRequest)
			return
		}

		addSong := Song{
			ID:      req.SongID,
			Title:   req.Title,
			Artist:  req.Artist,
			Edition: req.Edition,
			Year:    req.Year,
		}

		queue.Add(addSong, req.Guest)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		resp := map[string]string{"status": "queued"}
		json.NewEncoder(w).Encode(resp) //nolint:gosec // best-effort response encoding
	})
}

// QueueSkipHandler pops the next song from the queue and returns it.
func QueueSkipHandler(queue *Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entry := queue.Next()
		if entry == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entry) //nolint:gosec // best-effort response encoding
	})
}
