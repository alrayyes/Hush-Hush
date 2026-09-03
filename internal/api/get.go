package api

import (
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// handleGetObject returns an object's stored ciphertext exactly as sealed.
// Unauthenticated by design - v1's confidentiality boundary is entirely
// "who holds a matching private key" (api/openapi.yaml, design.md).
func handleGetObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		obj, err := s.GetObject(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "unknown object")

			return
		}

		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		if err := s.RecordAuditLog(r.Context(), id, store.AuditActionRead, callerFrom(r), sourceIPFrom(r)); err != nil {
			writeInternalError(w, r, err)

			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(obj.Value)
	}
}
