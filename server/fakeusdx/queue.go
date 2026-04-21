package fakeusdx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxQueueBodySize = 64 * 1024

type queueRequest struct {
	songID    string
	requester string
	players   int
}

// parseQueueBody reads and validates a /queue request body. On error it
// writes the appropriate response and returns false; caller must return.
func parseQueueBody(w http.ResponseWriter, r *http.Request) (queueRequest, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxQueueBodySize))
	if err != nil {
		var mbErr *http.MaxBytesError
		if errors.As(err, &mbErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "body too large")
			return queueRequest{}, false
		}
		writeError(w, http.StatusBadRequest, "malformed json")
		return queueRequest{}, false
	}

	var raw any
	if jsonErr := json.Unmarshal(body, &raw); jsonErr != nil {
		writeError(w, http.StatusBadRequest, "malformed json")
		return queueRequest{}, false
	}
	obj, isObject := raw.(map[string]any)
	if !isObject {
		writeError(w, http.StatusBadRequest, "body must be an object")
		return queueRequest{}, false
	}

	if _, has := obj["singer"]; has {
		writeError(w, http.StatusBadRequest, "legacy fields removed; use requester")
		return queueRequest{}, false
	}
	if _, has := obj["singers"]; has {
		writeError(w, http.StatusBadRequest, "legacy fields removed; use requester")
		return queueRequest{}, false
	}

	songID, ok := obj["songId"].(string)
	if !ok || songID == "" {
		writeError(w, http.StatusBadRequest, "songId required (string)")
		return queueRequest{}, false
	}

	requester, ok := obj["requester"].(string)
	if !ok || requester == "" {
		writeError(w, http.StatusBadRequest, "requester required")
		return queueRequest{}, false
	}

	players := 1
	if playersRaw, has := obj["players"]; has {
		playersFloat, isFloat := playersRaw.(float64)
		if !isFloat || (playersFloat != 1 && playersFloat != 2) {
			writeError(w, http.StatusBadRequest, "players must be 1 or 2")
			return queueRequest{}, false
		}
		players = int(playersFloat)
	}

	return queueRequest{songID: songID, requester: requester, players: players}, true
}

func (f *Fake) handleQueue(w http.ResponseWriter, r *http.Request) {
	req, ok := parseQueueBody(w, r)
	if !ok {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.screen == ScreenSing {
		writeError(w, http.StatusConflict, "song in progress")
		return
	}
	if !f.songLoadedLocked(req.songID) {
		writeError(w, http.StatusNotFound, "unknown songId")
		return
	}

	// Session lock: first /queue on ScreenMain sets the player count. On any
	// other screen, an active session is already in effect; honor its lock.
	if f.screen == ScreenMain && f.sessionPlayers == 0 {
		f.sessionPlayers = req.players
	}

	f.slot = &QueuedSlot{
		SongID:    req.songID,
		Requester: req.requester,
		Players:   f.sessionPlayers,
	}

	// ScreenScore auto-advances to ScreenNextUp (push handoff).
	if f.screen == ScreenScore {
		f.screen = ScreenNextUp
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "playing"})
}

// songLoadedLocked reports whether songID is present in the library.
// Caller must hold f.mu.
func (f *Fake) songLoadedLocked(songID string) bool {
	for _, s := range f.songs {
		if s.ID == songID {
			return true
		}
	}
	return false
}
