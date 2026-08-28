package api

import (
	"encoding/json"
	"net/http"
)

// NewMux wires the handlers registered against api/openapi.yaml. widgets is
// the whole of this scaffold's state — an in-memory map rather than a
// database, because there's nothing here worth persisting yet.
func NewMux(widgets map[string]Widget) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /widgets/{id}", handleGetWidget(widgets))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
