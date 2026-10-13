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

		// Go's built-in MIME table has no entry for .webmanifest, and a host
		// without a system one serves it as text/plain, which a browser
		// won't install from.
		if strings.HasSuffix(path, ".webmanifest") {
			w.Header().Set("Content-Type", "application/manifest+json")
		}

		if info, err := fs.Stat(build, path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)

			return
		}

		// A missing build file is a 404, not the app: a script or stylesheet
		// that isn't there must fail, not parse as HTML.
		if strings.HasPrefix(path, "_app/") {
			http.NotFound(w, r)

			return
		}

		// The SPA fallback is HTML whatever path asked for it. A client route
		// is a 200. Anything else is a 404 carrying the same app, which
		// renders its own not-found page (alrayyes/hush-hush#789).
		w.Header().Set("Cache-Control", cacheControlRevalidate)

		if !isPageRoute(r.URL.Path) {
			serveIndexWithStatus(w, build, http.StatusNotFound)

			return
		}

		http.ServeFileFS(w, r, build, "index.html")
	})

	// Text is compressed here because this is the server that ships it
	// (alrayyes/hush-hush#721). gzhttp skips anything already encoded or too
	// small to be worth it, and adds Vary: Accept-Encoding.
	return gzhttp.GzipHandler(handler)
}

// pageRoutes are the web UI's client routes: one per +page.svelte under
// cmd/hush-hush/web/src/routes, {name} standing for a [name] segment.
// TestPageRoutesMatchTheWebUIRoutes keeps it in step with that tree, so a new
// page can't be left off and answer 404 (docs/adr/0036).
var pageRoutes = []string{
	"/",
	"/audit-log",
	"/changelog",
	"/consumers",
	"/disclaimer",
	"/license",
	"/login",
	"/privacy",
	"/secrets/{slug}",
	"/settings",
}

// isPageRoute reports whether path is a client route. A trailing slash is
// ignored, as SvelteKit does by default.
func isPageRoute(path string) bool {
	segments := strings.Split(strings.Trim(path, "/"), "/")

	for _, route := range pageRoutes {
		pattern := strings.Split(strings.Trim(route, "/"), "/")
		if len(pattern) != len(segments) {
			continue
		}

		matched := true

		for i, part := range pattern {
			if segments[i] == "" || (!strings.HasPrefix(part, "{") && part != segments[i]) {
				matched = false

				break
			}
		}

		if matched {
			return true
		}
	}

	return false
}

// serveIndexWithStatus writes the build's index.html under a status
// http.ServeFileFS wouldn't use.
func serveIndexWithStatus(w http.ResponseWriter, build fs.FS, status int) {
	body, err := fs.ReadFile(build, "index.html")
	if err != nil {
		http.NotFound(w, nil)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
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
