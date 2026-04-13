package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joshsymonds/sound-stage/usdb"
)

func TestSanitizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "forward slash", input: "AC/DC", want: "AC-DC"},
		{name: "backslash", input: "path\\file", want: "path-file"},
		{name: "colon", input: "Artist: Title", want: "Artist- Title"},
		{name: "asterisk", input: "Star*Wars", want: "StarWars"},
		{name: "question mark", input: "What?", want: "What"},
		{name: "double quotes", input: `Say "Hello"`, want: "Say Hello"},
		{name: "angle brackets", input: "<tag>", want: "tag"},
		{name: "pipe", input: "A|B", want: "AB"},
		{name: "combined special chars", input: `A/B\C:D*E?F"G<H>I|J`, want: "A-B-C-DEFGHIJ"},
		{name: "clean string", input: "Queen - Bohemian Rhapsody", want: "Queen - Bohemian Rhapsody"},
		{name: "leading trailing spaces", input: "  hello  ", want: "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := usdb.SanitizePath(tt.input)
			if got != tt.want {
				t.Errorf("SanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCollectIDs_ArgsOnly(t *testing.T) {
	t.Parallel()

	ids, err := collectIDs([]string{"100", "200", "300"}, "")
	if err != nil {
		t.Fatalf("collectIDs: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != "100" || ids[1] != "200" || ids[2] != "300" {
		t.Errorf("unexpected IDs: %v", ids)
	}
}

func TestCollectIDs_FromFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	idFile := filepath.Join(dir, "ids.txt")
	if err := os.WriteFile(idFile, []byte("10\n20\n30\n"), 0o600); err != nil {
		t.Fatalf("writing id file: %v", err)
	}

	ids, err := collectIDs(nil, idFile)
	if err != nil {
		t.Fatalf("collectIDs: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != "10" || ids[1] != "20" || ids[2] != "30" {
		t.Errorf("unexpected IDs: %v", ids)
	}
}

func TestCollectIDs_CombinedArgsAndFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	idFile := filepath.Join(dir, "ids.txt")
	if err := os.WriteFile(idFile, []byte("20\n30\n"), 0o600); err != nil {
		t.Fatalf("writing id file: %v", err)
	}

	ids, err := collectIDs([]string{"10"}, idFile)
	if err != nil {
		t.Fatalf("collectIDs: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != "10" || ids[1] != "20" || ids[2] != "30" {
		t.Errorf("unexpected IDs: %v", ids)
	}
}

func TestCollectIDs_CommentsAndBlanks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	idFile := filepath.Join(dir, "ids.txt")
	content := "# Queen songs\n100\n\n# Daft Punk\n200\n\n"
	if err := os.WriteFile(idFile, []byte(content), 0o600); err != nil {
		t.Fatalf("writing id file: %v", err)
	}

	ids, err := collectIDs(nil, idFile)
	if err != nil {
		t.Fatalf("collectIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
	if ids[0] != "100" || ids[1] != "200" {
		t.Errorf("unexpected IDs: %v", ids)
	}
}

func TestCollectIDs_NonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := collectIDs(nil, "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestValidateProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		proxy   string
		wantErr bool
	}{
		{name: "empty", proxy: "", wantErr: false},
		{name: "socks5", proxy: "socks5://10.64.0.1:1080", wantErr: false},
		{name: "socks4", proxy: "socks4://10.64.0.1:1080", wantErr: false},
		{name: "http", proxy: "http://proxy.example.com:8080", wantErr: false},
		{name: "https", proxy: "https://proxy.example.com:8080", wantErr: false},
		{name: "no scheme", proxy: "10.64.0.1:1080", wantErr: true},
		{name: "ftp scheme", proxy: "ftp://proxy.example.com", wantErr: true},
		{name: "bare host", proxy: "proxy.example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateProxy(tt.proxy)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProxy(%q) error = %v, wantErr %v", tt.proxy, err, tt.wantErr)
			}
		})
	}
}
