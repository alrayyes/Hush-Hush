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

// handleHardNavRoute resolves a path's dual identity as both a SvelteKit
// page route and a real API endpoint (docs/adr/0015 keeps API paths
// unprefixed, so the two share one path rather than one moving under a
// new namespace). A hard navigation to it - a refresh, a bookmark, a
// Playwright page.goto - is a real HTTP request Go's mux would otherwise
// route straight to the JSON handler, ahead of the SPA fallback.
// /audit-log was the first case (alrayyes/hush-hush#272, docs/adr/0018);
// /consumers is the same collision, fixed the same way rather than a
// second bespoke dispatcher (alrayyes/hush-hush#295).
//
// The Fetch Metadata Sec-Fetch-Dest header
// (https://developer.mozilla.org/en-US/docs/Glossary/Fetch_metadata_request_header)
// is what tells the two apart: the browser sets it to "document" only
// for that kind of top-level navigation, never for the SPA's own
// fetch() call to the same path, and page script can't override it. No
// Sec-Fetch-Dest at all - curl, an SDK, hush-hush-cli - keeps today's
// JSON response, so the published API contract is unaffected.
func handleHardNavRoute(api, spa http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Sec-Fetch-Dest") == "document" {
			spa.ServeHTTP(w, r)

			return
		}

		api.ServeHTTP(w, r)
	}
}
