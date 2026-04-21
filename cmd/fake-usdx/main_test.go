package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSeed_ReadsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.json")

	payload := []map[string]any{
		{"title": "Song A", "artist": "Artist A", "duet": false},
		{"title": "Song B", "artist": "Artist B", "duet": true},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	songs, err := loadSeed(path)
	if err != nil {
		t.Fatalf("loadSeed: %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("got %d songs, want 2", len(songs))
	}
	if songs[0].Title != "Song A" || songs[0].Artist != "Artist A" || songs[0].Duet {
		t.Errorf("songs[0] = %+v", songs[0])
	}
	if !songs[1].Duet {
		t.Errorf("songs[1].Duet = false, want true")
	}
}

func TestLoadSeed_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := loadSeed(path); err == nil {
		t.Error("loadSeed: err = nil, want error")
	}
}

func TestLoadSeed_NotFound(t *testing.T) {
	if _, err := loadSeed("/tmp/does-not-exist-xyz.json"); err == nil {
		t.Error("loadSeed: err = nil, want error")
	}
}

func TestDefaultSeed_HasThreeSongs(t *testing.T) {
	songs := defaultSeed()
	if len(songs) != 3 {
		t.Fatalf("got %d songs, want 3", len(songs))
	}
	for i, s := range songs {
		if s.Title == "" {
			t.Errorf("songs[%d].Title is empty", i)
		}
		if s.Artist == "" {
			t.Errorf("songs[%d].Artist is empty", i)
		}
	}
}
