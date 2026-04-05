package usdb

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return string(data)
}

func TestParseSearchResults(t *testing.T) {
	t.Parallel()
	html := readFixture(t, "search_results.html")

	songs := parseSearchResults(html)

	if len(songs) < 3 {
		t.Fatalf("expected at least 3 songs, got %d", len(songs))
	}

	// First song: Albert Hammond - It Never Rains In Southern California
	first := songs[0]
	if first.ID != 57 {
		t.Errorf("first song ID = %d, want 57", first.ID)
	}
	if first.Artist != "Albert Hammond" {
		t.Errorf("first song Artist = %q, want %q", first.Artist, "Albert Hammond")
	}
	if first.Title != "It Never Rains In Southern California" {
		t.Errorf("first song Title = %q, want %q", first.Title, "It Never Rains In Southern California")
	}
	if first.Language != "English" {
		t.Errorf("first song Language = %q, want %q", first.Language, "English")
	}

	// Second song: Alex Britti - Prendere o lasciare
	second := songs[1]
	if second.ID != 59 {
		t.Errorf("second song ID = %d, want 59", second.ID)
	}
	if second.Language != "Italian, English" {
		t.Errorf("second song Language = %q, want %q", second.Language, "Italian, English")
	}
}

func TestExtractTextarea(t *testing.T) {
	t.Parallel()
	html := readFixture(t, "gettxt_response.html")

	txt, err := extractTextarea(html)
	if err != nil {
		t.Fatalf("extractTextarea: %v", err)
	}

	if txt == "" {
		t.Fatal("extractTextarea returned empty string")
	}

	// Should contain the UltraStar headers
	headers, _ := parseTxt(txt)

	assertHeader(t, headers, "TITLE", "Bohemian Rhapsody")
	assertHeader(t, headers, "ARTIST", "Queen")
	assertHeader(t, headers, "BPM", "280")
}

func TestExtractTextarea_NoTextarea(t *testing.T) {
	t.Parallel()
	_, err := extractTextarea("<html><body>no textarea here</body></html>")
	if err == nil {
		t.Error("expected error for missing textarea, got nil")
	}
}

func TestParseDetailPage_EmbeddedVideo(t *testing.T) {
	t.Parallel()
	html := readFixture(t, "detail_embedded_video.html")

	details := parseDetailPage(html, 26152)

	if details.Artist != "Revolverheld" {
		t.Errorf("Artist = %q, want %q", details.Artist, "Revolverheld")
	}
	if details.Title != "Ich lass für dich das Licht an" {
		t.Errorf("Title = %q, want %q", details.Title, "Ich lass für dich das Licht an")
	}
	if !details.HasCover {
		t.Error("HasCover = false, want true")
	}
	if len(details.YouTubeIDs) == 0 {
		t.Fatal("expected at least one YouTube ID")
	}
	if details.YouTubeIDs[0] != "Vf0MC3CFihY" {
		t.Errorf("YouTubeIDs[0] = %q, want %q", details.YouTubeIDs[0], "Vf0MC3CFihY")
	}
}

func TestParseDetailPage_UnembeddedVideo(t *testing.T) {
	t.Parallel()
	html := readFixture(t, "detail_unembedded_video.html")

	details := parseDetailPage(html, 16575)

	if details.Artist != "Donots" {
		t.Errorf("Artist = %q, want %q", details.Artist, "Donots")
	}
	if len(details.YouTubeIDs) == 0 {
		t.Fatal("expected at least one YouTube ID from plain text URL")
	}
	if details.YouTubeIDs[0] != "WIAvMiUcCgw" {
		t.Errorf("YouTubeIDs[0] = %q, want %q", details.YouTubeIDs[0], "WIAvMiUcCgw")
	}
}

func TestParseDetailPage_NoComments(t *testing.T) {
	t.Parallel()
	html := readFixture(t, "detail_no_comments.html")

	details := parseDetailPage(html, 26244)

	if details.Artist != "The Used" {
		t.Errorf("Artist = %q, want %q", details.Artist, "The Used")
	}
	if details.Title != "River Stay" {
		t.Errorf("Title = %q, want %q", details.Title, "River Stay")
	}
	if details.HasCover {
		t.Error("HasCover = true, want false (no cover for this song)")
	}
	if len(details.YouTubeIDs) != 0 {
		t.Errorf("expected no YouTube IDs, got %v", details.YouTubeIDs)
	}
}

func TestExtractYouTubeID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard watch", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"short URL", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embed URL", "http://www.youtube.com/v/dQw4w9WgXcQ&rel=1", "dQw4w9WgXcQ"},
		{"nocookie embed", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?rel=0", "dQw4w9WgXcQ"},
		{"with feature param", "http://www.youtube.com/watch?feature=player_detailpage&v=WIAvMiUcCgw", "WIAvMiUcCgw"},
		{"mobile", "https://m.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"no match", "https://example.com/video", ""},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractYouTubeID(tc.url)
			if got != tc.want {
				t.Errorf("extractYouTubeID(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestParseTxt(t *testing.T) {
	t.Parallel()
	raw := "#TITLE:Test Song\n#ARTIST:Test Artist\n#BPM:120\n: 0 5 10 Hello\nE"

	headers, body := parseTxt(raw)

	assertHeader(t, headers, "TITLE", "Test Song")
	assertHeader(t, headers, "ARTIST", "Test Artist")
	assertHeader(t, headers, "BPM", "120")

	if body != ": 0 5 10 Hello\nE" {
		t.Errorf("body = %q, want %q", body, ": 0 5 10 Hello\nE")
	}
}

func TestSetHeader(t *testing.T) {
	t.Parallel()
	headers := []header{
		{key: "TITLE", value: "Old"},
		{key: "ARTIST", value: "Someone"},
	}

	// Update existing.
	headers = setHeader(headers, "TITLE", "New")
	assertHeader(t, headers, "TITLE", "New")

	// Add new.
	headers = setHeader(headers, "VIDEO", "video.webm")
	assertHeader(t, headers, "VIDEO", "video.webm")
}

func TestPrepareSong(t *testing.T) {
	t.Parallel()
	rawTxt := "#TITLE:Test\n#ARTIST:Artist\n#MP3:old.mp3\n#BPM:120\n: 0 5 10 Hello\nE"
	details := &SongDetails{
		Artist:     "Artist",
		Title:      "Test",
		HasCover:   true,
		YouTubeIDs: []string{"dQw4w9WgXcQ"},
	}

	dir := t.TempDir()
	songDir := filepath.Join(dir, "Artist - Test")

	song, err := PrepareSong(rawTxt, details, songDir)
	if err != nil {
		t.Fatalf("PrepareSong: %v", err)
	}

	if song.YouTubeURL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Errorf("YouTubeURL = %q", song.YouTubeURL)
	}
	if song.AudioFile != "audio.webm" {
		t.Errorf("AudioFile = %q", song.AudioFile)
	}
	if song.VideoFile != "video.webm" {
		t.Errorf("VideoFile = %q", song.VideoFile)
	}

	// Verify written txt file.
	data, err := os.ReadFile(song.TxtPath)
	if err != nil {
		t.Fatalf("reading song.txt: %v", err)
	}
	content := string(data)

	// Should have corrected headers.
	headers, _ := parseTxt(content)
	assertHeader(t, headers, "MP3", "audio.webm")
	assertHeader(t, headers, "VIDEO", "video.webm")
	assertHeader(t, headers, "COVER", "cover.jpg")
}

func TestPrepareSong_NoYouTube(t *testing.T) {
	t.Parallel()
	rawTxt := "#TITLE:Test\n#ARTIST:Artist\n#BPM:120\n: 0 5 10 Hello\nE"
	details := &SongDetails{Artist: "Artist", Title: "Test"}

	dir := t.TempDir()
	songDir := filepath.Join(dir, "Artist - Test")

	song, err := PrepareSong(rawTxt, details, songDir)
	if err != nil {
		t.Fatalf("PrepareSong: %v", err)
	}
	if song.YouTubeURL != "" {
		t.Errorf("YouTubeURL = %q, want empty", song.YouTubeURL)
	}
}

func assertHeader(t *testing.T, headers []header, key, want string) {
	t.Helper()
	got := headerValue(headers, key)
	if got != want {
		t.Errorf("header %s = %q, want %q", key, got, want)
	}
}
