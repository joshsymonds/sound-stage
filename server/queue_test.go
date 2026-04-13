package server_test

import (
	"testing"

	"github.com/joshsymonds/sound-stage/server"
)

func song(id int, title string) server.Song {
	return server.Song{ID: id, Title: title, Artist: "Test Artist"}
}

func TestQueue(t *testing.T) {
	t.Parallel()

	t.Run("empty queue returns empty list", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		entries := q.List()
		if len(entries) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("single guest FIFO", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(song(1, "Song A"), "Alice")
		q.Add(song(2, "Song B"), "Alice")
		q.Add(song(3, "Song C"), "Alice")

		entries := q.List()
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
		if entries[0].Song.Title != "Song A" {
			t.Errorf("expected Song A first, got %s", entries[0].Song.Title)
		}
		if entries[2].Song.Title != "Song C" {
			t.Errorf("expected Song C last, got %s", entries[2].Song.Title)
		}
	})

	t.Run("round-robin interleaves guests", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(song(1, "Alice-1"), "Alice")
		q.Add(song(2, "Alice-2"), "Alice")
		q.Add(song(3, "Alice-3"), "Alice")
		q.Add(song(4, "Bob-1"), "Bob")
		q.Add(song(5, "Bob-2"), "Bob")

		entries := q.List()
		// Expected: Alice-1, Bob-1, Alice-2, Bob-2, Alice-3
		expected := []string{"Alice-1", "Bob-1", "Alice-2", "Bob-2", "Alice-3"}
		if len(entries) != len(expected) {
			t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
		}
		for i, want := range expected {
			if entries[i].Song.Title != want {
				t.Errorf("position %d: expected %s, got %s", i+1, want, entries[i].Song.Title)
			}
		}
	})

	t.Run("round-robin with three guests", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(song(1, "A1"), "Alice")
		q.Add(song(2, "A2"), "Alice")
		q.Add(song(3, "B1"), "Bob")
		q.Add(song(4, "C1"), "Charlie")
		q.Add(song(5, "C2"), "Charlie")

		entries := q.List()
		// Round 1: A1, B1, C1 | Round 2: A2, C2
		expected := []string{"A1", "B1", "C1", "A2", "C2"}
		if len(entries) != len(expected) {
			t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
		}
		for i, want := range expected {
			if entries[i].Song.Title != want {
				t.Errorf("position %d: expected %s, got %s", i+1, want, entries[i].Song.Title)
			}
		}
	})

	t.Run("positions are 1-indexed", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(song(1, "First"), "Alice")
		q.Add(song(2, "Second"), "Bob")

		entries := q.List()
		if entries[0].Position != 1 {
			t.Errorf("first position should be 1, got %d", entries[0].Position)
		}
		if entries[1].Position != 2 {
			t.Errorf("second position should be 2, got %d", entries[1].Position)
		}
	})

	t.Run("first entry is marked isNext", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(song(1, "First"), "Alice")
		q.Add(song(2, "Second"), "Bob")

		entries := q.List()
		if !entries[0].IsNext {
			t.Error("first entry should be isNext")
		}
		if entries[1].IsNext {
			t.Error("second entry should not be isNext")
		}
	})

	t.Run("Next pops the first entry", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(song(1, "First"), "Alice")
		q.Add(song(2, "Second"), "Bob")

		entry := q.Next()
		if entry == nil {
			t.Fatal("expected entry, got nil")
		}
		if entry.Song.Title != "First" {
			t.Errorf("expected First, got %s", entry.Song.Title)
		}
		if entry.Guest != "Alice" {
			t.Errorf("expected Alice, got %s", entry.Guest)
		}

		remaining := q.List()
		if len(remaining) != 1 {
			t.Fatalf("expected 1 remaining, got %d", len(remaining))
		}
		if remaining[0].Song.Title != "Second" {
			t.Errorf("expected Second remaining, got %s", remaining[0].Song.Title)
		}
	})

	t.Run("Next on empty queue returns nil", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		if entry := q.Next(); entry != nil {
			t.Errorf("expected nil, got %+v", entry)
		}
	})

	t.Run("Remove by position", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(song(1, "A1"), "Alice")
		q.Add(song(2, "B1"), "Bob")
		q.Add(song(3, "A2"), "Alice")

		// Remove position 2 (B1 in round-robin: A1, B1, A2).
		ok := q.Remove(2)
		if !ok {
			t.Fatal("expected Remove to return true")
		}

		entries := q.List()
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		for _, e := range entries {
			if e.Song.Title == "B1" {
				t.Error("B1 should have been removed")
			}
		}
	})

	t.Run("Remove invalid position returns false", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		q.Add(song(1, "A1"), "Alice")
		if q.Remove(99) {
			t.Error("expected Remove(99) to return false")
		}
	})

	t.Run("guest order is preserved by insertion order", func(t *testing.T) {
		t.Parallel()
		q := server.NewQueue()
		// Charlie adds first, then Alice, then Bob.
		q.Add(song(1, "C1"), "Charlie")
		q.Add(song(2, "A1"), "Alice")
		q.Add(song(3, "B1"), "Bob")

		entries := q.List()
		// Round-robin order should respect first-seen: Charlie, Alice, Bob.
		expected := []string{"C1", "A1", "B1"}
		for i, want := range expected {
			if entries[i].Song.Title != want {
				t.Errorf("position %d: expected %s, got %s", i+1, want, entries[i].Song.Title)
			}
		}
	})
}
