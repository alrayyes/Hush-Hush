package main

import "embed"

// webBuild holds the SvelteKit SPA's built output, embedded into the
// binary so the whole service ships as one file - design.md's "Build
// embedding" decision. It lives under this package's own directory,
// not a repo-root web/ one, because go:embed can only reach a
// subdirectory of the file that declares it (no ../ escapes) - the
// frontend project is scaffolded here for exactly that reason
// (alrayyes/hush-hush#205, correcting design.md's original wording).
//
// all: is load-bearing: SvelteKit's own build output includes an
// _app/ directory, and go:embed excludes _-prefixed files and
// directories unless the pattern says otherwise.
//
// web/build/index.html is a placeholder until alrayyes/hush-hush#206
// scaffolds the real SvelteKit project here.
//
//go:embed all:web/build
var webBuild embed.FS
