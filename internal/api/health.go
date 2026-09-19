package api

import "net/http"

// Health is the body /healthz answers with. Matches
// components.schemas.Health in api/openapi.yaml.
type Health struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// handleHealth ignores the request and answers 200 unconditionally — see the
// operation description in api/openapi.yaml for why. version is the running
// binary's own version (cmd/hush-hush's goreleaser-stamped var, "dev" for a
// plain go build) - the web UI's footer links it to the changelog page
// rather than hardcoding a version that would drift from what's running.
func handleHealth(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, Health{Status: "ok", Version: version})
	}
}
