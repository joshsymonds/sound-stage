package server

import (
	"path/filepath"
	"strings"
)

// DeckPath translates a server-local library path into the Deck's view of
// the same file. The library is a single NFS export mounted at libraryDir on
// this host and at deckLibraryDir on the Deck, so translation is a prefix
// swap. Returns ok=false when no mapping is configured or serverPath lies
// outside libraryDir — callers fall back to the untranslated path.
func DeckPath(serverPath, libraryDir, deckLibraryDir string) (string, bool) {
	if libraryDir == "" || deckLibraryDir == "" {
		return "", false
	}
	rel, err := filepath.Rel(libraryDir, serverPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(deckLibraryDir, rel), true
}
