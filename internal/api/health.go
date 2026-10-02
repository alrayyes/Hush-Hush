package api

import (
	"context"
	"log/slog"
	"net/http"
)

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

// Ready is the body /readyz answers with. Matches
// components.schemas.Ready in api/openapi.yaml. Deliberately just a status:
// the endpoint is unauthenticated, so a failure never says why.
type Ready struct {
	Status string `json:"status"`
}

// readinessChecker is the one thing /readyz needs from the store.
type readinessChecker interface {
	Ready(ctx context.Context) error
}

// handleReady answers 200 when the store can serve a request and 503 when
// it can't. /healthz stays unconditional for anything that only wants to
// know the process is up; this is what the container's own health check
// asks.
func handleReady(s readinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.Ready(r.Context()); err != nil {
			slog.WarnContext(r.Context(), "not ready", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, Ready{Status: "unavailable"})

			return
		}

		writeJSON(w, http.StatusOK, Ready{Status: "ok"})
	}
}
