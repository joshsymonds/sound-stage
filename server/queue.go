package server

import "sync"

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
	guestOrder []string                // order in which guests first appeared
	guestSongs map[string][]guestEntry // per-guest FIFO sub-queues
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

	// Track guest order by first appearance.
	if _, exists := q.guestSongs[guest]; !exists {
		q.guestOrder = append(q.guestOrder, guest)
	}

	q.guestSongs[guest] = append(q.guestSongs[guest], guestEntry{song: song, guest: guest})
}

// List returns the round-robin interleaved queue with positions and isNext.
func (q *Queue) List() []QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.listLocked()
}

func (q *Queue) listLocked() []QueueEntry {
	// Build interleaved list: take one song from each guest per round.
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

	// Remove the first song from this guest's sub-queue.
	songs := q.guestSongs[first.Guest]
	q.guestSongs[first.Guest] = songs[1:]

	// Clean up empty guest entries.
	if len(q.guestSongs[first.Guest]) == 0 {
		delete(q.guestSongs, first.Guest)
		q.guestOrder = removeFromSlice(q.guestOrder, first.Guest)
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
		q.guestOrder = removeFromSlice(q.guestOrder, target.Guest)
	}

	return true
}

func removeFromSlice(slice []string, val string) []string {
	for i, v := range slice {
		if v == val {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
