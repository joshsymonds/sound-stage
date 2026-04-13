package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
)

func writeSongTxt(t *testing.T, dir, artist, title string) {
	t.Helper()
	songDir := filepath.Join(dir, artist+" - "+title)
	if err := os.MkdirAll(songDir, 0o750); err != nil {
		t.Fatal(err)
	}
	txt := "#TITLE:" + title + "\n#ARTIST:" + artist + "\n#YEAR:2024\n#EDITION:Test Edition\n#MP3:audio.webm\n: 0 5 10 Hello\nE\n"
	if err := os.WriteFile(filepath.Join(songDir, "song.txt"), []byte(txt), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSongsHandler(t *testing.T) {
	t.Parallel()

	t.Run("returns empty array for empty directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		handler := server.SongsHandler(dir)
		req := httptest.NewRequest(http.MethodGet, "/api/songs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json, got %s", ct)
		}

		var songs []server.Song
		if err := json.Unmarshal(rec.Body.Bytes(), &songs); err != nil {
			t.Fatal(err)
		}
		if len(songs) != 0 {
			t.Fatalf("expected 0 songs, got %d", len(songs))
		}
	})

	t.Run("returns songs from library", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeSongTxt(t, dir, "Queen", "Bohemian Rhapsody")
		writeSongTxt(t, dir, "ABBA", "Dancing Queen")

		handler := server.SongsHandler(dir)
		req := httptest.NewRequest(http.MethodGet, "/api/songs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var songs []server.Song
		if err := json.Unmarshal(rec.Body.Bytes(), &songs); err != nil {
			t.Fatal(err)
		}
		if len(songs) != 2 {
			t.Fatalf("expected 2 songs, got %d", len(songs))
		}

		// Verify song fields.
		found := map[string]bool{}
		for _, s := range songs {
			found[s.Title] = true
			if s.Artist == "" {
				t.Errorf("song %q has empty artist", s.Title)
			}
			if s.Year != 2024 {
				t.Errorf("song %q: expected year 2024, got %d", s.Title, s.Year)
			}
			if s.Edition != "Test Edition" {
				t.Errorf("song %q: expected edition 'Test Edition', got %q", s.Title, s.Edition)
			}
		}
		if !found["Bohemian Rhapsody"] || !found["Dancing Queen"] {
			t.Errorf("missing expected songs: %v", found)
		}
	})

	t.Run("skips directories without song.txt", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeSongTxt(t, dir, "Queen", "Bohemian Rhapsody")
		// Create a directory without song.txt.
		if err := os.MkdirAll(filepath.Join(dir, "empty-dir"), 0o750); err != nil {
			t.Fatal(err)
		}

		handler := server.SongsHandler(dir)
		req := httptest.NewRequest(http.MethodGet, "/api/songs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var songs []server.Song
		if err := json.Unmarshal(rec.Body.Bytes(), &songs); err != nil {
			t.Fatal(err)
		}
		if len(songs) != 1 {
			t.Fatalf("expected 1 song, got %d", len(songs))
		}
	})
}
