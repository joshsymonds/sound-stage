package server

import (
	"encoding/json"
	"net/http"
)

const maxRequestBody = 1 << 20 // 1 MB

type queueAddRequest struct {
	SongID  string `json:"songId"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Duet    bool   `json:"duet"`
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
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
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
			Duet:    req.Duet,
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
