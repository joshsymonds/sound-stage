package archive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsDownloaded_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if IsDownloaded(dir, 12345) {
		t.Error("expected false for non-existent archive")
	}
}

func TestMarkAndCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := MarkDownloaded(dir, 100); err != nil {
		t.Fatalf("MarkDownloaded(100): %v", err)
	}
	if err := MarkDownloaded(dir, 200); err != nil {
		t.Fatalf("MarkDownloaded(200): %v", err)
	}

	if !IsDownloaded(dir, 100) {
		t.Error("expected 100 to be downloaded")
	}
	if !IsDownloaded(dir, 200) {
		t.Error("expected 200 to be downloaded")
	}
	if IsDownloaded(dir, 300) {
		t.Error("expected 300 to NOT be downloaded")
	}
}

func TestMarkDownloaded_FileContents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := MarkDownloaded(dir, 42); err != nil {
		t.Fatalf("MarkDownloaded: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".downloaded.txt"))
	if err != nil {
		t.Fatalf("reading archive: %v", err)
	}

	if string(data) != "42\n" {
		t.Errorf("archive contents = %q, want %q", string(data), "42\n")
	}
}

func TestMarkDownloaded_Append(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := MarkDownloaded(dir, 1); err != nil {
		t.Fatalf("MarkDownloaded(1): %v", err)
	}
	if err := MarkDownloaded(dir, 2); err != nil {
		t.Fatalf("MarkDownloaded(2): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".downloaded.txt"))
	if err != nil {
		t.Fatalf("reading archive: %v", err)
	}

	if string(data) != "1\n2\n" {
		t.Errorf("archive contents = %q, want %q", string(data), "1\n2\n")
	}
}

func TestLoadDownloaded_NoArchive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ids, err := LoadDownloaded(dir)
	if err != nil {
		t.Fatalf("LoadDownloaded: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty set, got %d entries", len(ids))
	}
}

func TestLoadDownloaded_WithEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, id := range []int{10, 20, 30} {
		if err := MarkDownloaded(dir, id); err != nil {
			t.Fatalf("MarkDownloaded(%d): %v", id, err)
		}
	}

	ids, err := LoadDownloaded(dir)
	if err != nil {
		t.Fatalf("LoadDownloaded: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(ids))
	}
	for _, id := range []int{10, 20, 30} {
		if _, ok := ids[id]; !ok {
			t.Errorf("expected %d in set", id)
		}
	}
}
