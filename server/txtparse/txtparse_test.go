package txtparse_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joshsymonds/sound-stage/server/txtparse"
)

func writeTestFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "song.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestParse_BasicSong(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "#TITLE:Dancing Queen\n#ARTIST:ABBA\n#BPM:200\n: 0 4 60 Hello\nE\n")

	song, err := txtparse.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if song.Artist != "ABBA" || song.Title != "Dancing Queen" || song.Duet {
		t.Errorf("got %+v", song)
	}
}

func TestParse_MissingArtist(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "#TITLE:X\n#BPM:200\n: 0 4 60 Hi\nE\n")
	if _, err := txtparse.Parse(path); err == nil {
		t.Error("Parse: err = nil, want error")
	}
}

func TestParse_MissingTitle(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "#ARTIST:X\n#BPM:200\n: 0 4 60 Hi\nE\n")
	if _, err := txtparse.Parse(path); err == nil {
		t.Error("Parse: err = nil, want error")
	}
}

func TestParse_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "#ARTIST:\n#TITLE:X\n#BPM:200\n: 0 4 60 Hi\nE\n")
	if _, err := txtparse.Parse(path); err == nil {
		t.Error("Parse: err = nil, want error on empty artist value")
	}
}

func TestParse_DuetViaDuetSingerHeader(t *testing.T) {
	dir := t.TempDir()
	content := "#TITLE:T\n#ARTIST:A\n#DUETSINGERP1:Singer1\n" +
		"#DUETSINGERP2:Singer2\n#BPM:200\n: 0 4 60 Hi\nE\n"
	path := writeTestFile(t, dir, content)
	song, err := txtparse.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !song.Duet {
		t.Errorf("Duet = false, want true")
	}
}

func TestParse_DuetViaP1P2Lines(t *testing.T) {
	dir := t.TempDir()
	content := "#ARTIST:A\n#TITLE:T\n#BPM:200\nP1\n: 0 4 60 X\nP2\n: 4 4 60 Y\nE\n"
	path := writeTestFile(t, dir, content)
	song, err := txtparse.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !song.Duet {
		t.Errorf("Duet = false, want true")
	}
}

func TestParse_WhitespaceStripped(t *testing.T) {
	dir := t.TempDir()
	content := "#ARTIST: ABBA \t\n#TITLE:\tDancing Queen  \n#BPM:200\n: 0 4 60 Hi\nE\n"
	path := writeTestFile(t, dir, content)
	song, err := txtparse.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if song.Artist != "ABBA" || song.Title != "Dancing Queen" {
		t.Errorf("got %+v, want ABBA/Dancing Queen (trimmed)", song)
	}
}

func TestParse_NotFound(t *testing.T) {
	if _, err := txtparse.Parse("/tmp/does-not-exist-xyz.txt"); err == nil {
		t.Error("Parse: err = nil, want error")
	}
}
