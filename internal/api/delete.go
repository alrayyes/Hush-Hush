package api

import (
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// handleDeleteObject permanently removes an object. A subsequent get for
// the same id returns 404.
func handleDeleteObject(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := s.DeleteObject(r.Context(), r.PathValue("id"))
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, store.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "unknown object")
		default:
			writeInternalError(w, r, err)
		}
	}
}
