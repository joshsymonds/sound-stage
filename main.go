// Package main is the entry point for the sound-stage CLI.
package main

import (
	"fmt"
	"os"

	"github.com/joshsymonds/sound-stage/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
