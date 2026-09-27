package api

import (
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// handleDeleteObject permanently removes an object. A subsequent get for
// the same slug returns 404.
func handleDeleteObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")

		err := s.DeleteObject(r.Context(), slug)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "unknown object")

			return
		default:
			writeInternalError(w, r, err)

			return
		}

		actorType, actorID := actorFrom(r)
		if err := s.RecordAuditLog(r.Context(), slug, store.AuditActionDelete, callerFrom(r), sourceIPFrom(r), actorType, actorID); err != nil {
			writeInternalError(w, r, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
