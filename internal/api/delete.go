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
		id := r.PathValue("id")

		err := s.DeleteObject(r.Context(), id)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "unknown object")

			return
		default:
			writeInternalError(w, r, err)

			return
		}

		if err := s.RecordAuditLog(r.Context(), id, store.AuditActionDelete, callerFrom(r)); err != nil {
			writeInternalError(w, r, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
