package fakeusdx

import "errors"

// ErrWrongScreen is returned by PressSing/PressEnter/PressEsc when invoked
// from a screen where the action doesn't apply.
var ErrWrongScreen = errors.New("wrong screen for this action")

// ErrNoSlot is returned by PressEnter when invoked on ScreenNextUp without
// a populated slot.
var ErrNoSlot = errors.New("no staged song in slot")

const defaultPlaybackDuration = 180.0

// PressSing simulates the Deck user pressing the Sing button on ScreenMain.
// With a populated slot: transition to ScreenNextUp. Without a slot: no-op
// (real USDX falls through to song selection, which the fake doesn't model).
// On any other screen: ErrWrongScreen, state unchanged.
func (f *Fake) PressSing() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.screen != ScreenMain {
		return ErrWrongScreen
	}
	if f.slot == nil {
		return nil
	}
	f.screen = ScreenNextUp
	return nil
}

// PressEnter simulates Enter on ScreenNextUp: consumes the slot, starts the
// staged song at elapsed=0 with the fake's default duration, and transitions
// to ScreenSing. Paused clears (new song starts unpaused). Session player
// count is re-established from the slot.
func (f *Fake) PressEnter() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.screen != ScreenNextUp {
		return ErrWrongScreen
	}
	if f.slot == nil {
		return ErrNoSlot
	}

	entry, ok := f.lookupSongLocked(f.slot.SongID)
	if !ok {
		return ErrUnknownSongID
	}

	f.playing = &playingState{
		entry:    entry,
		elapsed:  0,
		duration: f.effectiveDurationLocked(),
	}
	f.paused = false
	f.sessionPlayers = f.slot.Players
	f.slot = nil
	f.screen = ScreenSing
	return nil
}

// PressEsc simulates Esc/Backspace on ScreenNextUp: returns to ScreenMain
// preserving the slot so the user can retry. Ending on ScreenMain also ends
// the session (per API.md "session ends when the user returns to ScreenMain").
func (f *Fake) PressEsc() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.screen != ScreenNextUp {
		return ErrWrongScreen
	}
	f.screen = ScreenMain
	f.sessionPlayers = 0
	return nil
}

// AdvanceElapsed adds `seconds` to the currently-playing song's elapsed time.
// No-op if not playing or paused. If the new elapsed value meets or exceeds
// duration, the song completes: playing clears, paused clears, screen goes
// to ScreenScore. Returns the new elapsed value (or 0 if not playing).
func (f *Fake) AdvanceElapsed(seconds float64) float64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.playing == nil {
		return 0
	}
	if f.paused {
		return f.playing.elapsed
	}

	f.playing.elapsed += seconds
	if f.playing.elapsed >= f.playing.duration {
		f.playing = nil
		f.paused = false
		f.screen = ScreenScore
		return 0
	}
	return f.playing.elapsed
}

// SetDefaultDuration sets the duration applied to songs promoted via
// PressEnter. Defaults to 180s. Useful in tests to trigger AdvanceElapsed
// terminal behavior without real-time waits.
func (f *Fake) SetDefaultDuration(seconds float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.defaultDuration = seconds
}

// lookupSongLocked returns the songEntry for the given ID and whether it
// was found. Caller must hold f.mu.
func (f *Fake) lookupSongLocked(songID string) (songEntry, bool) {
	for _, s := range f.songs {
		if s.ID == songID {
			return s, true
		}
	}
	return songEntry{}, false
}

// effectiveDurationLocked returns the default duration, substituting the
// hardcoded 180s if the setter hasn't been called. Caller must hold f.mu.
func (f *Fake) effectiveDurationLocked() float64 {
	if f.defaultDuration <= 0 {
		return defaultPlaybackDuration
	}
	return f.defaultDuration
}
