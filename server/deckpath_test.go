package server

import "testing"

func TestDeckPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		serverPath     string
		libraryDir     string
		deckLibraryDir string
		want           string
		wantOK         bool
	}{
		{
			name:           "prefix swap",
			serverPath:     "/mnt/music/sound-stage/ABBA - Dancing Queen/song.txt",
			libraryDir:     "/mnt/music/sound-stage",
			deckLibraryDir: "/var/mnt/music/sound-stage",
			want:           "/var/mnt/music/sound-stage/ABBA - Dancing Queen/song.txt",
			wantOK:         true,
		},
		{
			name:           "unset deck dir disables translation",
			serverPath:     "/mnt/music/sound-stage/ABBA - Dancing Queen/song.txt",
			libraryDir:     "/mnt/music/sound-stage",
			deckLibraryDir: "",
			wantOK:         false,
		},
		{
			name:           "unset library dir disables translation",
			serverPath:     "/mnt/music/sound-stage/ABBA - Dancing Queen/song.txt",
			libraryDir:     "",
			deckLibraryDir: "/var/mnt/music/sound-stage",
			wantOK:         false,
		},
		{
			name:           "path outside library root refuses",
			serverPath:     "/etc/passwd",
			libraryDir:     "/mnt/music/sound-stage",
			deckLibraryDir: "/var/mnt/music/sound-stage",
			wantOK:         false,
		},
		{
			name:           "sibling directory with shared prefix refuses",
			serverPath:     "/mnt/music/sound-stage-other/song.txt",
			libraryDir:     "/mnt/music/sound-stage",
			deckLibraryDir: "/var/mnt/music/sound-stage",
			wantOK:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := deckPath(tt.serverPath, tt.libraryDir, tt.deckLibraryDir)
			if ok != tt.wantOK {
				t.Fatalf("deckPath ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("deckPath = %q, want %q", got, tt.want)
			}
		})
	}
}
