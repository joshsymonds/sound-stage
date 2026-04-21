package main

import "embed"

// staticFS holds the compiled SvelteKit SPA. The all: prefix is required —
// SvelteKit emits hashed assets under _app/, and go:embed's default rules
// exclude underscore-prefixed paths.
//
// web/build is gitignored (built artifact) but a sentinel `.keep` is
// committed so the embed directive always finds the directory. Production
// builds run `npm run build` first to populate it; dev workflows do the
// same via the Justfile recipes.
//
//go:embed all:web/build
var staticFS embed.FS
