package api

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"net/http"
	"regexp"
	"strings"

	"github.com/klauspost/compress/gzhttp"
)

// inlineScript finds a script element's attributes and body. The index.html
// it's run against is this project's own build output, a handful of
// well-formed tags, so a regular expression is enough and no HTML parser has
// to become a direct dependency.
var inlineScript = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)

// contentSecurityPolicy is the policy the web UI is served under
// (alrayyes/hush-hush#660). A script runs only if it comes from this origin or
// is one of the inline scripts in the build's own index.html, so a script an
// attacker injects into the page doesn't run and can't read a secret before
// it's sealed. Styles allow inline because SvelteKit and the component
// library set style attributes; a style can't run code. frame-ancestors is
// only honoured in a header, which is why the policy is sent from here and
// not as a meta tag.
func contentSecurityPolicy(build fs.FS) string {
	sources := []string{"'self'"}

	if index, err := fs.ReadFile(build, "index.html"); err == nil {
		for _, m := range inlineScript.FindAllSubmatch(index, -1) {
			if strings.Contains(strings.ToLower(string(m[1])), "src=") || len(m[2]) == 0 {
				continue
			}

			sum := sha256.Sum256(m[2])
			sources = append(sources, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		}
	}

	return strings.Join([]string{
		"default-src 'self'",
		"script-src " + strings.Join(sources, " "),
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data:",
		"font-src 'self'",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")
}

// handleStatic serves the embedded SPA build - any path that doesn't
// match a real file in build gets index.html instead, so SvelteKit's own
// client-side router can take over (adapter-static's "fallback:
// 'index.html'" pattern, design.md's "Build embedding" decision).
// Registered last, as the mux's fallback for anything no more specific
// API pattern already claimed - design.md's "Routing boundary" decision.
func handleStatic(build fs.FS) http.Handler {
	fileServer := http.FileServerFS(build)
	policy := contentSecurityPolicy(build)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)

		path := r.URL.Path[1:] // ServeMux guarantees a leading "/"
		if path == "" {
			path = "index.html"
		}

		w.Header().Set("Cache-Control", cacheControl(path))

		if info, err := fs.Stat(build, path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)

			return
		}

		// The SPA fallback is HTML whatever path asked for it.
		w.Header().Set("Cache-Control", cacheControlRevalidate)
		http.ServeFileFS(w, r, build, "index.html")
	})

	// Text is compressed here because this is the server that ships it
	// (alrayyes/hush-hush#721). gzhttp skips anything already encoded or too
	// small to be worth it, and adds Vary: Accept-Encoding.
	return gzhttp.GzipHandler(handler)
}

const (
	// immutablePrefix is where SvelteKit writes content-hashed files: the name
	// changes when the content does, so a copy never goes stale.
	immutablePrefix = "_app/immutable/"

	cacheControlImmutable  = "public, max-age=31536000, immutable"
	cacheControlRevalidate = "no-cache"
)

// cacheControl is the header for a static path: a year for a hashed file, and
// revalidation for everything else, HTML included, since an HTML URL can't be
// cache-busted.
func cacheControl(path string) string {
	if strings.HasPrefix(path, immutablePrefix) {
		return cacheControlImmutable
	}

	return cacheControlRevalidate
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
