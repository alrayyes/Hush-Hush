package api

import (
	"io/fs"
	"net/http"
)

// handleStatic serves the embedded SPA build - any path that doesn't
// match a real file in build gets index.html instead, so SvelteKit's own
// client-side router can take over (adapter-static's "fallback:
// 'index.html'" pattern, design.md's "Build embedding" decision).
// Registered last, as the mux's fallback for anything no more specific
// API pattern already claimed - design.md's "Routing boundary" decision.
func handleStatic(build fs.FS) http.Handler {
	fileServer := http.FileServerFS(build)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[1:] // ServeMux guarantees a leading "/"
		if path == "" {
			path = "index.html"
		}

		if info, err := fs.Stat(build, path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)

			return
		}

		http.ServeFileFS(w, r, build, "index.html")
	})
}
