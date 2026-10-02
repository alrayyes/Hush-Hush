package api

import "net/http"

// Health is the body /healthz answers with. Matches
// components.schemas.Health in api/openapi.yaml.
type Health struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	// Environment is the operator's own label for this instance
	// (INSTANCE_LABEL), omitted when unset. Unauthenticated like the rest
	// of this body, so it's never a place for anything secret.
	Environment string `json:"environment,omitempty"`
}

// handleHealth ignores the request and answers 200 unconditionally — see the
// operation description in api/openapi.yaml for why. version is the running
// binary's own version (cmd/hush-hush's goreleaser-stamped var, "dev" for a
// plain go build) - the web UI's footer links it to the changelog page
// rather than hardcoding a version that would drift from what's running.
func handleHealth(version, instanceLabel string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, Health{Status: "ok", Version: version, Environment: instanceLabel})
	}
}
