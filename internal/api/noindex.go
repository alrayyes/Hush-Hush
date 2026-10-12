package api

import "net/http"

// NoIndex marks every response noindex, nofollow so a search engine that
// reaches the server leaves it out of its results. It wraps the whole mux
// from the outside, like AccessLog, so JSON, static assets and errors all
// carry it. robots.txt stays permissive on purpose: a crawler that is
// blocked from fetching a page never sees the header on it.
func NoIndex(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		next.ServeHTTP(w, r)
	})
}
