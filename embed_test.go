package main

import (
	"io/fs"
	"testing"
)

// TestEmbeddedPWAAssets guards against a future build that silently drops
// the PWA manifest or icons. Without this, a missing manifest would only
// surface as "the install prompt stopped appearing" in production.
func TestEmbeddedPWAAssets(t *testing.T) {
	t.Parallel()
	sub, err := fs.Sub(staticFS, "web/build")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	required := []string{
		"manifest.webmanifest",
		"icon-192.png",
		"icon-512.png",
		"icon-512-maskable.png",
		"apple-touch-icon.png",
	}
	for _, name := range required {
		if _, statErr := fs.Stat(sub, name); statErr != nil {
			t.Errorf("missing PWA asset %s in embedded FS: %v", name, statErr)
		}
	}
}
