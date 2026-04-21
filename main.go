// Package main is the entry point for the sound-stage CLI.
package main

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/joshsymonds/sound-stage/cmd"
)

func main() {
	// Strip the embed root prefix so the SPA serves at / rather than
	// /web/build/. The embed directive guarantees the subtree exists; an
	// error here means the build is malformed.
	sub, err := fs.Sub(staticFS, "web/build")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: fs.Sub on embedded staticFS: %v\n", err)
		os.Exit(1)
	}
	cmd.SetStaticFS(sub)

	if execErr := cmd.Execute(); execErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", execErr)
		os.Exit(1)
	}
}
