package server

import (
	"encoding/json"
	"net/http"
	"strconv"
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

// QueueRemoveHandler removes the entry at the given 1-indexed position when
// the requesting guest matches the entry's owner.
//
// Threat model: Wi-Fi-gated trust. The `guest` query param is the only
// identity check — a guest who types someone else's name into NameEntry can
// remove their songs. This is acceptable for the LAN/house-party scope; if
// the threat model ever broadens, this is the place to add real auth.
func QueueRemoveHandler(queue *Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		position, err := strconv.Atoi(r.PathValue("position"))
		if err != nil || position < 1 {
			http.Error(w, "valid position is required", http.StatusBadRequest)
			return
		}
		guest := r.URL.Query().Get("guest")
		if guest == "" {
			http.Error(w, "guest is required", http.StatusBadRequest)
			return
		}

		entries := queue.List()
		if position > len(entries) {
			http.Error(w, "position out of range", http.StatusNotFound)
			return
		}
		if entries[position-1].Guest != guest {
			http.Error(w, "not your song", http.StatusForbidden)
			return
		}
		// Race: between List and Remove, the queue could shift (e.g.,
		// QueueDriver pops). Treat a remove miss as 404 — from the user's
		// vantage point, the entry they tried to delete is no longer there.
		if !queue.Remove(position) {
			http.Error(w, "position out of range", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
