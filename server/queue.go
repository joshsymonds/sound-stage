package server

import (
	"slices"
	"sync"
)

// QueueEntry represents a song in the queue with its position and guest.
type QueueEntry struct {
	Position int    `json:"position"`
	Song     Song   `json:"song"`
	Guest    string `json:"guest"`
	IsNext   bool   `json:"isNext"`
}

type guestEntry struct {
	song  Song
	guest string
}

// Queue manages a round-robin fair song queue across guests.
// Safe for concurrent access.
type Queue struct {
	mu         sync.Mutex
	guestOrder []string                // order in which guests first appeared — monotonic, never shrinks
	guestSongs map[string][]guestEntry // per-guest FIFO sub-queues; empty sub-queues are removed
}

// NewQueue creates a new empty queue.
func NewQueue() *Queue {
	return &Queue{
		guestSongs: make(map[string][]guestEntry),
	}
}

// Add appends a song to a guest's sub-queue.
func (q *Queue) Add(song Song, guest string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.ensureGuestOrderLocked(guest)
	q.guestSongs[guest] = append(q.guestSongs[guest], guestEntry{song: song, guest: guest})
}

// ReAdd prepends a song to a guest's sub-queue — used by the queue driver
// when a stage attempt fails transiently (409/5xx) and the song needs to be
// retried before other entries from the same guest move ahead of it.
// The guest's round-robin position is preserved: even if their sub-queue had
// emptied between Next and ReAdd, their original turn order is unchanged.
func (q *Queue) ReAdd(song Song, guest string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.ensureGuestOrderLocked(guest)
	entry := guestEntry{song: song, guest: guest}
	q.guestSongs[guest] = append([]guestEntry{entry}, q.guestSongs[guest]...)
}

// ensureGuestOrderLocked adds guest to guestOrder if not already present.
// Caller must hold q.mu. guestOrder is monotonic — an entry that appears
// here is never removed, so a guest whose sub-queue empties and later
// re-fills (via ReAdd or a fresh Add) retains their original turn order.
func (q *Queue) ensureGuestOrderLocked(guest string) {
	if slices.Contains(q.guestOrder, guest) {
		return
	}
	q.guestOrder = append(q.guestOrder, guest)
}

// List returns the round-robin interleaved queue with positions and isNext.
func (q *Queue) List() []QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.listLocked()
}

func (q *Queue) listLocked() []QueueEntry {
	// Build interleaved list: take one song from each guest per round.
	// Guests with empty sub-queues contribute nothing; they stay in
	// guestOrder so their position is preserved if they re-queue later.
	var entries []QueueEntry
	round := 0

	for {
		addedThisRound := false
		for _, guest := range q.guestOrder {
			songs := q.guestSongs[guest]
			if round < len(songs) {
				entries = append(entries, QueueEntry{
					Song:  songs[round].song,
					Guest: songs[round].guest,
				})
				addedThisRound = true
			}
		}
		if !addedThisRound {
			break
		}
		round++
	}

	// Set positions (1-indexed) and isNext flag.
	for i := range entries {
		entries[i].Position = i + 1
		entries[i].IsNext = i == 0
	}

	return entries
}

// Next pops and returns the first entry in round-robin order, or nil if empty.
func (q *Queue) Next() *QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()

	entries := q.listLocked()
	if len(entries) == 0 {
		return nil
	}

	first := entries[0]

	// Remove the first song from this guest's sub-queue. Leave guestOrder
	// alone so the guest keeps their position if they re-queue.
	songs := q.guestSongs[first.Guest]
	if len(songs) > 1 {
		q.guestSongs[first.Guest] = songs[1:]
	} else {
		delete(q.guestSongs, first.Guest)
	}

	return &first
}

// Remove removes the entry at the given 1-indexed position. Returns true if removed.
func (q *Queue) Remove(position int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	entries := q.listLocked()
	if position < 1 || position > len(entries) {
		return false
	}

	target := entries[position-1]

	// Find and remove this song from the guest's sub-queue.
	songs := q.guestSongs[target.Guest]
	idx := -1
	for i, s := range songs {
		if s.song.ID == target.Song.ID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return false
	}

	q.guestSongs[target.Guest] = append(songs[:idx], songs[idx+1:]...)

	if len(q.guestSongs[target.Guest]) == 0 {
		delete(q.guestSongs, target.Guest)
	}

	return true
}
