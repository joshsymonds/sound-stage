package server

import (
	"encoding/json"
	"errors"
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

// QueueRemoveByGuestHandler drops every queued entry belonging to the named
// guest. Wi-Fi-gated trust model: anyone can boot anyone (the user's stated
// "no admin, we're all friends" intent). Always 204 on a valid request,
// even when the guest had zero entries — idempotent from the caller's view.
func QueueRemoveByGuestHandler(queue *Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guest := r.URL.Query().Get("guest")
		if guest == "" {
			http.Error(w, "guest is required", http.StatusBadRequest)
			return
		}
		queue.RemoveByGuest(guest)
		w.WriteHeader(http.StatusNoContent)
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

		// Owner check happens INSIDE Queue.Remove under the queue lock —
		// no snapshot-then-mutate race window where a concurrent Next()
		// could shift positions between auth and removal.
		removeErr := queue.Remove(position, guest)
		switch {
		case errors.Is(removeErr, ErrNotYourSong):
			http.Error(w, "not your song", http.StatusForbidden)
			return
		case errors.Is(removeErr, ErrPositionOutOfRange):
			http.Error(w, "position out of range", http.StatusNotFound)
			return
		case removeErr != nil:
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
