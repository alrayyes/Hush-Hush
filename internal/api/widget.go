package api

import "net/http"

// Widget is the example resource this scaffold ships. Matches
// components.schemas.Widget in api/openapi.yaml.
type Widget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Error is the body every documented error response carries. Matches
// components.schemas.Error in api/openapi.yaml.
type Error struct {
	Error string `json:"error"`
}

func handleGetWidget(widgets map[string]Widget) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		widget, ok := widgets[r.PathValue("id")]
		if !ok {
			writeJSON(w, http.StatusNotFound, Error{Error: "unknown widget"})
			return
		}
		writeJSON(w, http.StatusOK, widget)
	}
}
