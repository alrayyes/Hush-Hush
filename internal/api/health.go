package api

import "net/http"

// Health is the body /healthz answers with. Matches
// components.schemas.Health in api/openapi.yaml.
type Health struct {
	Status string `json:"status"`
}

// handleHealth ignores the request and answers 200 unconditionally — see the
// operation description in api/openapi.yaml for why.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, Health{Status: "ok"})
}
