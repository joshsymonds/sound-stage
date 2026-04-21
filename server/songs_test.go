package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/server/stableid"
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
		handler := server.SongsHandler(server.NewLibraryCache(), dir)
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

		handler := server.SongsHandler(server.NewLibraryCache(), dir)
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

		handler := server.SongsHandler(server.NewLibraryCache(), dir)
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

	t.Run("IDs are stableid hashes of (artist, title, duet)", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeSongTxt(t, dir, "ABBA", "Dancing Queen")

		handler := server.SongsHandler(server.NewLibraryCache(), dir)
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
		want := stableid.Compute("ABBA", "Dancing Queen", false)
		if songs[0].ID != want {
			t.Errorf("id = %q, want %q (stableid.Compute parity)", songs[0].ID, want)
		}
		if songs[0].Duet {
			t.Errorf("duet = true, want false")
		}
	})

	t.Run("collision keeps first, discards subsequent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Two distinct directories with identical artist/title → identical stableid.
		dirA := filepath.Join(dir, "A-ABBA - Dancing Queen")
		dirB := filepath.Join(dir, "B-ABBA - Dancing Queen")
		for _, songDir := range []string{dirA, dirB} {
			if err := os.MkdirAll(songDir, 0o750); err != nil {
				t.Fatal(err)
			}
			txt := "#TITLE:Dancing Queen\n#ARTIST:ABBA\n#MP3:audio.webm\n: 0 5 10 Hi\nE\n"
			if err := os.WriteFile(filepath.Join(songDir, "song.txt"), []byte(txt), 0o600); err != nil {
				t.Fatal(err)
			}
		}

		handler := server.SongsHandler(server.NewLibraryCache(), dir)
		req := httptest.NewRequest(http.MethodGet, "/api/songs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var songs []server.Song
		if err := json.Unmarshal(rec.Body.Bytes(), &songs); err != nil {
			t.Fatal(err)
		}
		if len(songs) != 1 {
			t.Errorf("expected 1 song after collision, got %d", len(songs))
		}
	})

	t.Run("LibraryCache avoids rescanning the same directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeSongTxt(t, dir, "Queen", "Bohemian Rhapsody")

		cache := server.NewLibraryCache()

		// First call scans; second should return the cached slice without
		// noticing a new file added to disk.
		songs1, err := cache.Get(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(songs1) != 1 {
			t.Fatalf("got %d, want 1", len(songs1))
		}

		writeSongTxt(t, dir, "ABBA", "Dancing Queen")

		songs2, _ := cache.Get(dir)
		if len(songs2) != 1 {
			t.Errorf("cache should still report 1 before Invalidate; got %d", len(songs2))
		}

		cache.Invalidate()
		songs3, _ := cache.Get(dir)
		if len(songs3) != 2 {
			t.Errorf("after Invalidate, expected 2, got %d", len(songs3))
		}
	})

	t.Run("detects duet via P1/P2 markers", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		songDir := filepath.Join(dir, "Kenny & Dolly - Islands")
		if err := os.MkdirAll(songDir, 0o750); err != nil {
			t.Fatal(err)
		}
		txt := "#TITLE:Islands\n#ARTIST:Kenny & Dolly\n#MP3:audio.webm\nP1\n: 0 4 60 X\nP2\n: 4 4 60 Y\nE\n"
		if err := os.WriteFile(filepath.Join(songDir, "song.txt"), []byte(txt), 0o600); err != nil {
			t.Fatal(err)
		}

		handler := server.SongsHandler(server.NewLibraryCache(), dir)
		req := httptest.NewRequest(http.MethodGet, "/api/songs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var songs []server.Song
		if err := json.Unmarshal(rec.Body.Bytes(), &songs); err != nil {
			t.Fatal(err)
		}
		if len(songs) != 1 || !songs[0].Duet {
			t.Errorf("expected one duet song, got %+v", songs)
		}
	})
}
