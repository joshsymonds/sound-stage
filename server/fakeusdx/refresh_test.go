package fakeusdx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

// writeSong writes a valid USDX .txt file with the given metadata to a
// temp directory and returns the absolute path.
func writeSong(t *testing.T, dir, filename, artist, title string, duet bool) string {
	t.Helper()
	var b strings.Builder
	if artist != "" {
		b.WriteString("#ARTIST:" + artist + "\n")
	}
	if title != "" {
		b.WriteString("#TITLE:" + title + "\n")
	}
	b.WriteString("#BPM:200\n")
	if duet {
		b.WriteString("#DUETSINGERP1:Singer1\n")
		b.WriteString("#DUETSINGERP2:Singer2\n")
	}
	b.WriteString(": 0 4 60 Hello\n")
	b.WriteString("E\n")

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// postRefresh serves POST /refresh with the given body.
func postRefresh(t *testing.T, fake *fakeusdx.Fake, body []byte) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	return rec, rec.Body.Bytes()
}

func postRefreshJSON(t *testing.T, fake *fakeusdx.Fake, body any) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return postRefresh(t, fake, raw)
}

func assertAddedFalseError(t *testing.T, raw []byte, want string) {
	t.Helper()
	var got struct {
		Added *bool  `json:"added"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("body not JSON: %v, body=%q", err, raw)
	}
	if got.Added == nil || *got.Added {
		t.Errorf("added = %v, want false in body %q", got.Added, raw)
	}
	if got.Error != want {
		t.Errorf("error = %q, want %q", got.Error, want)
	}
}

// --- Body validation ---

func TestRefresh_MalformedJSON(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postRefresh(t, fake, []byte("not json"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "malformed json")
}

func TestRefresh_BodyIsArray(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postRefresh(t, fake, []byte(`[1,2,3]`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "body must be an object")
}

func TestRefresh_MissingPath(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postRefreshJSON(t, fake, map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "path required (string)")
}

func TestRefresh_PathNumber(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postRefreshJSON(t, fake, map[string]any{"path": 123})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "path required (string)")
}

func TestRefresh_PathEmpty(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postRefreshJSON(t, fake, map[string]any{"path": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertErrorBody(t, body, "path required (string)")
}

func TestRefresh_BodyTooLarge(t *testing.T) {
	fake := fakeusdx.New()
	big := strings.Repeat("x", 65*1024)
	raw := []byte(`{"path":"` + big + `"}`)
	rec, body := postRefresh(t, fake, raw)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
	assertErrorBody(t, body, "body too large")
}

// --- Path validation ---

func TestRefresh_PathRelative(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postRefreshJSON(t, fake, map[string]any{"path": "relative/path.txt"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertAddedFalseError(t, body, "path must be absolute")
}

func TestRefresh_WrongSuffix(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postRefreshJSON(t, fake, map[string]any{"path": "/tmp/song.xyz"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertAddedFalseError(t, body, "path must end in .txt")
}

func TestRefresh_MixedCaseSuffixAccepted(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	path := writeSong(t, dir, "song.TXT", "ABBA", "Dancing Queen", false)

	rec, _ := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (mixed-case suffix must be accepted)", rec.Code)
	}
}

func TestRefresh_PathNotFound(t *testing.T) {
	fake := fakeusdx.New()
	rec, body := postRefreshJSON(t, fake, map[string]any{"path": "/tmp/does-not-exist-xyz.txt"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	assertAddedFalseError(t, body, "path not found")
}

// --- Parsing ---

func TestRefresh_ValidSong(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	path := writeSong(t, dir, "song.txt", "ABBA", "Dancing Queen", false)

	rec, body := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, body)
	}

	var got struct {
		Added bool   `json:"added"`
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.Added {
		t.Errorf("added = false, want true")
	}
	wantID := stableid.Compute("ABBA", "Dancing Queen", false)
	if got.ID != wantID {
		t.Errorf("id = %q, want %q", got.ID, wantID)
	}
	if got.Title != "Dancing Queen" {
		t.Errorf("title = %q, want %q", got.Title, "Dancing Queen")
	}
}

func TestRefresh_MissingArtist(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	path := writeSong(t, dir, "song.txt", "", "Dancing Queen", false)

	rec, body := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertAddedFalseError(t, body, "parse failed")
}

func TestRefresh_MissingTitle(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	path := writeSong(t, dir, "song.txt", "ABBA", "", false)

	rec, body := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertAddedFalseError(t, body, "parse failed")
}

func TestRefresh_EmptyArtistValue(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	// Write a file where #ARTIST is present but value is empty.
	content := "#ARTIST:\n#TITLE:Dancing Queen\n#BPM:200\n: 0 4 60 X\nE\n"
	path := filepath.Join(dir, "song.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rec, body := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	assertAddedFalseError(t, body, "parse failed")
}

func TestRefresh_DuetViaDuetSingerHeader(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	path := writeSong(t, dir, "song.txt", "ABBA", "Dancing Queen", true)

	rec, body := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}

	var got struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &got)
	wantID := stableid.Compute("ABBA", "Dancing Queen", true)
	if got.ID != wantID {
		t.Errorf("id = %q, want %q (must reflect duet=true)", got.ID, wantID)
	}

	// /songs should reflect duet=true.
	rec2, body2 := doRequest(t, fake, http.MethodGet, "/songs")
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /songs: %d", rec2.Code)
	}
	var songs []map[string]any
	_ = json.Unmarshal(body2, &songs)
	if len(songs) != 1 {
		t.Fatalf("got %d songs, want 1", len(songs))
	}
	if duet, _ := songs[0]["duet"].(bool); !duet {
		t.Errorf("duet = false, want true in %v", songs[0])
	}
}

func TestRefresh_DuetViaP1P2Lines(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	content := "#ARTIST:Artist\n#TITLE:Title\n#BPM:200\nP1\n: 0 4 60 X\nP2\n: 4 4 60 Y\nE\n"
	path := filepath.Join(dir, "song.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rec, body := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}

	var got struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &got)
	wantID := stableid.Compute("Artist", "Title", true)
	if got.ID != wantID {
		t.Errorf("id = %q, want %q (must reflect duet=true from P1/P2 lines)", got.ID, wantID)
	}
}

func TestRefresh_WhitespaceAroundValue(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	// Trailing space / tab on header values must be stripped.
	content := "#ARTIST: ABBA \t\n#TITLE:\tDancing Queen  \n#BPM:200\n: 0 4 60 X\nE\n"
	path := filepath.Join(dir, "song.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rec, body := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, body)
	}

	var got struct {
		Title string `json:"title"`
		ID    string `json:"id"`
	}
	_ = json.Unmarshal(body, &got)
	if got.Title != "Dancing Queen" {
		t.Errorf("title = %q, want %q (whitespace must be stripped)", got.Title, "Dancing Queen")
	}
	wantID := stableid.Compute("ABBA", "Dancing Queen", false)
	if got.ID != wantID {
		t.Errorf("id = %q, want %q (whitespace stripping matters for hash)", got.ID, wantID)
	}
}

// --- Screen state ---

func TestRefresh_ScreenSing_Returns409(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	path := writeSong(t, dir, "song.txt", "ABBA", "Dancing Queen", false)
	if err := fake.SetScreen(fakeusdx.ScreenSing); err != nil {
		t.Fatalf("SetScreen: %v", err)
	}

	rec, body := postRefreshJSON(t, fake, map[string]any{"path": path})
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	assertAddedFalseError(t, body, "song in progress")

	// Library must be unchanged.
	_, songsBody := doRequest(t, fake, http.MethodGet, "/songs")
	if string(songsBody) != "[]" {
		t.Errorf("library = %q, want %q (must be unchanged on 409)", songsBody, "[]")
	}
}

// --- Library mutation ---

func TestRefresh_AddsSongToFreshLibrary(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	path := writeSong(t, dir, "song.txt", "ABBA", "Dancing Queen", false)

	if rec, _ := postRefreshJSON(t, fake, map[string]any{"path": path}); rec.Code != http.StatusOK {
		t.Fatalf("refresh: %d", rec.Code)
	}

	_, songsBody := doRequest(t, fake, http.MethodGet, "/songs")
	var songs []map[string]any
	if err := json.Unmarshal(songsBody, &songs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(songs) != 1 {
		t.Fatalf("got %d songs, want 1", len(songs))
	}
	if songs[0]["title"] != "Dancing Queen" {
		t.Errorf("title = %v", songs[0]["title"])
	}
}

func TestRefresh_ReplaceInPlaceSamePath(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	path := filepath.Join(dir, "song.txt")

	// First write
	if err := os.WriteFile(path, []byte("#ARTIST:A\n#TITLE:First\n#BPM:200\n: 0 4 60 X\nE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rec, _ := postRefreshJSON(t, fake, map[string]any{"path": path}); rec.Code != http.StatusOK {
		t.Fatalf("first refresh: %d", rec.Code)
	}

	// Overwrite with new metadata
	if err := os.WriteFile(path, []byte("#ARTIST:B\n#TITLE:Second\n#BPM:200\n: 0 4 60 X\nE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rec, _ := postRefreshJSON(t, fake, map[string]any{"path": path}); rec.Code != http.StatusOK {
		t.Fatalf("second refresh: %d", rec.Code)
	}

	_, songsBody := doRequest(t, fake, http.MethodGet, "/songs")
	var songs []map[string]any
	_ = json.Unmarshal(songsBody, &songs)
	if len(songs) != 1 {
		t.Fatalf("got %d songs, want 1 (same path must replace in place)", len(songs))
	}
	if songs[0]["title"] != "Second" {
		t.Errorf("title = %v, want Second", songs[0]["title"])
	}
}

func TestRefresh_CollisionRemovesOlder(t *testing.T) {
	fake := fakeusdx.New()
	dir := t.TempDir()
	// Two different paths, same metadata → same stable ID → collision.
	path1 := writeSong(t, dir, "copy1.txt", "ABBA", "Dancing Queen", false)
	path2 := writeSong(t, dir, "copy2.txt", "ABBA", "Dancing Queen", false)

	if rec, _ := postRefreshJSON(t, fake, map[string]any{"path": path1}); rec.Code != http.StatusOK {
		t.Fatalf("first refresh: %d", rec.Code)
	}
	if rec, _ := postRefreshJSON(t, fake, map[string]any{"path": path2}); rec.Code != http.StatusOK {
		t.Fatalf("second refresh: %d", rec.Code)
	}

	_, songsBody := doRequest(t, fake, http.MethodGet, "/songs")
	var songs []map[string]any
	_ = json.Unmarshal(songsBody, &songs)
	if len(songs) != 1 {
		t.Fatalf("got %d songs, want 1 (collision should remove older entry)", len(songs))
	}
	// The surviving entry should be the one from path2. We can't observe the
	// path in the /songs response (it's intentionally hidden), but the id
	// should equal the collision ID.
	wantID := stableid.Compute("ABBA", "Dancing Queen", false)
	if songs[0]["id"] != wantID {
		t.Errorf("id = %v, want %q", songs[0]["id"], wantID)
	}
}

func TestRefresh_OrderPreservation(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{
		{Title: "A-title", Artist: "A-artist", Duet: false},
		{Title: "B-title", Artist: "B-artist", Duet: false},
		{Title: "C-title", Artist: "C-artist", Duet: false},
	})

	dir := t.TempDir()
	path := writeSong(t, dir, "d.txt", "D-artist", "D-title", false)
	if rec, _ := postRefreshJSON(t, fake, map[string]any{"path": path}); rec.Code != http.StatusOK {
		t.Fatalf("refresh: %d", rec.Code)
	}

	_, songsBody := doRequest(t, fake, http.MethodGet, "/songs")
	var songs []map[string]any
	_ = json.Unmarshal(songsBody, &songs)
	if len(songs) != 4 {
		t.Fatalf("got %d songs, want 4", len(songs))
	}
	wantTitles := []string{"A-title", "B-title", "C-title", "D-title"}
	for i, wantTitle := range wantTitles {
		if songs[i]["title"] != wantTitle {
			t.Errorf("songs[%d].title = %v, want %q", i, songs[i]["title"], wantTitle)
		}
	}
}

// --- Method check ---

func TestRefresh_GetReturns405(t *testing.T) {
	fake := fakeusdx.New()
	req := httptest.NewRequest(http.MethodGet, "/refresh", nil)
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}
