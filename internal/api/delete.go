package api

import (
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// handleDeleteObject permanently removes an object. A subsequent get for
// the same slug returns 404.
func handleDeleteObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")

		opts, ok := selectorFrom(w, r)
		if !ok {
			return
		}

		err := s.DeleteObject(r.Context(), slug, opts...)
		if objectLookupFailed(w, r, err) {
			return
		}

		if err != nil {
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
