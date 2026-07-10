package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeSong creates <libDir>/<dirName>/song.txt with content and returns its path.
func writeSong(t *testing.T, libDir, dirName, content string) string {
	t.Helper()
	songDir := filepath.Join(libDir, dirName)
	if err := os.MkdirAll(songDir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", songDir, err)
	}
	txtPath := filepath.Join(songDir, "song.txt")
	if err := os.WriteFile(txtPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", txtPath, err)
	}
	return txtPath
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestFixEntitiesSkipsNewlineProducingEntities(t *testing.T) {
	// &#10; decodes to a raw newline; rewriting it would split one header
	// into two lines and let USDB-sourced content fabricate headers
	// (e.g. #VIDEO). Such values must be left byte-identical.
	dir := t.TempDir()
	txt := "#ARTIST:A\n#TITLE:T\n#EDITION:Foo&#10;#VIDEO:evil.mp4\n#MP3:audio.webm\n: 0 5 10 Hi\nE\n"
	txtPath := writeSong(t, dir, "A - T", txt)

	var out strings.Builder
	result, err := fixEntities(t.Context(), fixEntitiesConfig{
		libraryDir: dir,
		apply:      true,
		out:        &out,
	})
	if err != nil {
		t.Fatalf("fixEntities: %v", err)
	}
	if result.filesRewritten != 0 {
		t.Errorf("filesRewritten = %d, want 0 (newline-producing entity must be skipped)", result.filesRewritten)
	}
	got := readFileString(t, txtPath)
	if got != txt {
		t.Errorf("song.txt modified:\n%q\nwant unchanged:\n%q", got, txt)
	}
}

func TestFixEntitiesTagValues(t *testing.T) {
	t.Parallel()
	libDir := t.TempDir()
	content := "#ARTIST:A &amp; B\n#TITLE:Song\n#MP3:audio.webm\n: 0 5 10 Hi\nP1\nE\n"
	txtPath := writeSong(t, libDir, "A & B - Song", content)

	var out bytes.Buffer
	result, err := fixEntities(t.Context(), fixEntitiesConfig{libraryDir: libDir, apply: true, out: &out})
	if err != nil {
		t.Fatalf("fixEntities: %v", err)
	}

	want := "#ARTIST:A & B\n#TITLE:Song\n#MP3:audio.webm\n: 0 5 10 Hi\nP1\nE\n"
	if got := readFileString(t, txtPath); got != want {
		t.Errorf("song.txt = %q, want %q", got, want)
	}
	if result.filesRewritten != 1 {
		t.Errorf("filesRewritten = %d, want 1", result.filesRewritten)
	}
	if result.dirsRenamed != 0 {
		t.Errorf("dirsRenamed = %d, want 0", result.dirsRenamed)
	}
	if !strings.Contains(out.String(), "#ARTIST:A &amp; B → #ARTIST:A & B") {
		t.Errorf("output missing tag change line, got:\n%s", out.String())
	}
}

func TestFixEntitiesLeavesKeysAndNoteLines(t *testing.T) {
	t.Parallel()
	libDir := t.TempDir()
	// Key with an entity but entity-free value stays byte-identical; a note
	// line carrying &amp; in lyrics is never touched; only the header VALUE
	// with an entity is rewritten.
	content := "#ARTIST:X\n#A&amp;B:plain\n#COMMENT:&quot;quoted&quot; &#39;n stuff\n: 0 5 10 You &amp; Me\nE\n"
	txtPath := writeSong(t, libDir, "X - Y", content)

	var out bytes.Buffer
	if _, err := fixEntities(t.Context(), fixEntitiesConfig{libraryDir: libDir, apply: true, out: &out}); err != nil {
		t.Fatalf("fixEntities: %v", err)
	}

	want := "#ARTIST:X\n#A&amp;B:plain\n#COMMENT:\"quoted\" 'n stuff\n: 0 5 10 You &amp; Me\nE\n"
	if got := readFileString(t, txtPath); got != want {
		t.Errorf("song.txt = %q, want %q", got, want)
	}
}

func TestFixEntitiesRenamesDir(t *testing.T) {
	t.Parallel()
	libDir := t.TempDir()
	content := "#ARTIST:A &amp; B\n#TITLE:Song\nE\n"
	writeSong(t, libDir, "A &amp; B - Song", content)

	var out bytes.Buffer
	result, err := fixEntities(t.Context(), fixEntitiesConfig{libraryDir: libDir, apply: true, out: &out})
	if err != nil {
		t.Fatalf("fixEntities: %v", err)
	}

	if got, want := listDir(t, libDir), []string{"A & B - Song"}; !equalStrings(got, want) {
		t.Errorf("library dirs = %v, want %v", got, want)
	}
	if result.dirsRenamed != 1 {
		t.Errorf("dirsRenamed = %d, want 1", result.dirsRenamed)
	}
	renamedTxt := filepath.Join(libDir, "A & B - Song", "song.txt")
	if got, want := readFileString(t, renamedTxt), "#ARTIST:A & B\n#TITLE:Song\nE\n"; got != want {
		t.Errorf("song.txt after rename = %q, want %q", got, want)
	}
}

func TestFixEntitiesRenameCollisionSkips(t *testing.T) {
	t.Parallel()
	libDir := t.TempDir()
	writeSong(t, libDir, "A &amp; B - Song", "#ARTIST:A &amp; B\n#TITLE:Song\nE\n")
	writeSong(t, libDir, "A & B - Song", "#ARTIST:A & B\n#TITLE:Song\nE\n")

	var out bytes.Buffer
	result, err := fixEntities(t.Context(), fixEntitiesConfig{libraryDir: libDir, apply: true, out: &out})
	if err != nil {
		t.Fatalf("fixEntities: %v", err)
	}

	if result.dirsRenamed != 0 {
		t.Errorf("dirsRenamed = %d, want 0", result.dirsRenamed)
	}
	// Source dir intact (txt still fixed), target dir untouched.
	wantDirs := []string{"A & B - Song", "A &amp; B - Song"}
	if got := listDir(t, libDir); !equalStrings(got, wantDirs) {
		t.Errorf("library dirs = %v, want %v", got, wantDirs)
	}
	srcTxt := filepath.Join(libDir, "A &amp; B - Song", "song.txt")
	if got, want := readFileString(t, srcTxt), "#ARTIST:A & B\n#TITLE:Song\nE\n"; got != want {
		t.Errorf("source song.txt = %q, want %q", got, want)
	}
	if !strings.Contains(strings.ToLower(out.String()), "skip") {
		t.Errorf("output missing skip warning, got:\n%s", out.String())
	}
}

func TestFixEntitiesIdempotent(t *testing.T) {
	t.Parallel()
	libDir := t.TempDir()
	writeSong(t, libDir, "A &amp; B - Song", "#ARTIST:A &amp; B\n#TITLE:Song\nE\n")

	var first bytes.Buffer
	if _, err := fixEntities(t.Context(), fixEntitiesConfig{libraryDir: libDir, apply: true, out: &first}); err != nil {
		t.Fatalf("first run: %v", err)
	}

	var second bytes.Buffer
	result, err := fixEntities(t.Context(), fixEntitiesConfig{libraryDir: libDir, apply: true, out: &second})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if result.filesRewritten != 0 || result.dirsRenamed != 0 {
		t.Errorf("second run = %+v, want zero changes", result)
	}
}

func TestFixEntitiesDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	libDir := t.TempDir()
	content := "#ARTIST:A &amp; B\n#TITLE:Song\nE\n"
	txtPath := writeSong(t, libDir, "A &amp; B - Song", content)
	dirsBefore := listDir(t, libDir)

	var out bytes.Buffer
	result, err := fixEntities(t.Context(), fixEntitiesConfig{libraryDir: libDir, apply: false, out: &out})
	if err != nil {
		t.Fatalf("fixEntities: %v", err)
	}

	if got := readFileString(t, txtPath); got != content {
		t.Errorf("dry run modified song.txt: %q, want %q", got, content)
	}
	if got := listDir(t, libDir); !equalStrings(got, dirsBefore) {
		t.Errorf("dry run modified dirs: %v, want %v", got, dirsBefore)
	}
	if result.filesRewritten != 1 || result.dirsRenamed != 1 {
		t.Errorf("dry run result = %+v, want 1 file / 1 dir reported", result)
	}
	if !strings.Contains(out.String(), "→") {
		t.Errorf("dry run output missing change lines, got:\n%s", out.String())
	}
}

func TestFixEntitiesNotifiesDeck(t *testing.T) {
	t.Parallel()
	libDir := t.TempDir()
	writeSong(t, libDir, "A &amp; B - Song", "#ARTIST:A &amp; B\n#TITLE:Song\nE\n")

	var gotPaths []string
	deck := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/refresh" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode /refresh body: %v", err)
		}
		gotPaths = append(gotPaths, payload.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer deck.Close()

	var out bytes.Buffer
	result, err := fixEntities(t.Context(), fixEntitiesConfig{
		libraryDir:     libDir,
		apply:          true,
		deckURL:        deck.URL,
		deckLibraryDir: "/deck/lib",
		out:            &out,
	})
	if err != nil {
		t.Fatalf("fixEntities: %v", err)
	}

	if result.refreshOK != 1 || result.refreshFailed != 0 {
		t.Errorf("refresh ok/failed = %d/%d, want 1/0", result.refreshOK, result.refreshFailed)
	}
	wantPath := "/deck/lib/A & B - Song/song.txt"
	if len(gotPaths) != 1 || gotPaths[0] != wantPath {
		t.Errorf("deck /refresh paths = %v, want [%s]", gotPaths, wantPath)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
