package fakeusdx

import "net/http"

func (f *Fake) handlePause(w http.ResponseWriter) {
	f.mu.Lock()
	if f.playing == nil || f.paused {
		f.mu.Unlock()
		writeError(w, http.StatusConflict, "not playing")
		return
	}
	f.paused = true
	f.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (f *Fake) handleResume(w http.ResponseWriter) {
	f.mu.Lock()
	if !f.paused {
		f.mu.Unlock()
		writeError(w, http.StatusConflict, "nothing to resume")
		return
	}
	f.paused = false
	f.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

type debugStateResponse struct {
	Screen                string             `json:"screen"`
	IniPlayers            int                `json:"iniPlayers"`
	PlayersPlay           int                `json:"playersPlay"`
	AudioFinished         bool               `json:"audioFinished"`
	IniName               []string           `json:"iniName"`
	Player                []debugPlayerEntry `json:"player"`
	ScreenSingPlayerNames []string           `json:"screenSingPlayerNames"`
	CurrentSong           *debugCurrentSong  `json:"currentSong"`
	QueuedSong            *debugQueuedSong   `json:"queuedSong"`
}

type debugPlayerEntry struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

type debugCurrentSong struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

type debugQueuedSong struct {
	SongID    string `json:"songId"`
	Requester string `json:"requester"`
	Is2P      bool   `json:"is2P"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
}

func (f *Fake) handleDebugState(w http.ResponseWriter) {
	f.mu.Lock()
	defer f.mu.Unlock()

	resp := debugStateResponse{
		Screen:                string(f.screen),
		IniPlayers:            iniPlayersIndex(f.sessionPlayers),
		PlayersPlay:           playersPlay(f.sessionPlayers),
		AudioFinished:         f.screen == ScreenSing && f.playing == nil,
		IniName:               defaultIniNames(),
		ScreenSingPlayerNames: make([]string, 6),
	}

	// Requester becomes P1's name when a slot is populated; otherwise placeholder.
	p1Name := "Player1"
	if f.slot != nil && f.slot.Requester != "" {
		p1Name = f.slot.Requester
	}
	resp.Player = make([]debugPlayerEntry, resp.PlayersPlay)
	for i := range resp.Player {
		name := playerName(i, p1Name)
		resp.Player[i] = debugPlayerEntry{Name: name, Level: 1}
		resp.ScreenSingPlayerNames[i] = name
	}

	if f.playing != nil {
		resp.CurrentSong = &debugCurrentSong{
			ID:     f.playing.entry.ID,
			Title:  f.playing.entry.Title,
			Artist: f.playing.entry.Artist,
		}
	}
	if f.slot != nil {
		title, artist := f.resolveSongLocked(f.slot.SongID)
		resp.QueuedSong = &debugQueuedSong{
			SongID:    f.slot.SongID,
			Requester: f.slot.Requester,
			Is2P:      f.slot.Players == 2,
			Title:     title,
			Artist:    artist,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// resolveSongLocked looks up title/artist for the given song ID in the loaded
// library. Returns empty strings if not found. Caller must hold f.mu.
func (f *Fake) resolveSongLocked(songID string) (string, string) {
	for _, s := range f.songs {
		if s.ID == songID {
			return s.Title, s.Artist
		}
	}
	return "", ""
}

// iPlayersVals mirrors USDX's IPlayersVals constant array — the allowed
// player counts indexed by the config's player-count index. Effectively a
// constant (Go has no array constants).
//
//nolint:gochecknoglobals // immutable, mirrors USDX's IPlayersVals constant
var iPlayersVals = [...]int{1, 2, 3, 4, 6}

// iniPlayersIndex maps a player count to USDX's IPlayersVals index.
// Unknown values (including 0) fall back to 0.
func iniPlayersIndex(players int) int {
	for i, v := range iPlayersVals {
		if v == players {
			return i
		}
	}
	return 0
}

// playersPlay returns the effective play-count, minimum 1.
func playersPlay(sessionPlayers int) int {
	if sessionPlayers < 1 {
		return 1
	}
	return sessionPlayers
}

func defaultIniNames() []string {
	return []string{"Player1", "Player2", "Player3", "Player4", "Player5", "Player6"}
}

func playerName(index int, p1 string) string {
	if index == 0 {
		return p1
	}
	return defaultIniNames()[index]
}
