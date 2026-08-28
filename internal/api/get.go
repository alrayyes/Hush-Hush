package api

import (
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// handleGetObject returns an object's stored ciphertext exactly as sealed.
// Unauthenticated by design - v1's confidentiality boundary is entirely
// "who holds a matching private key" (api/openapi.yaml, design.md).
func handleGetObject(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		obj, err := s.GetObject(r.Context(), r.PathValue("id"))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "unknown object")

			return
		}

		if err != nil {
			writeInternalError(w, err)

			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(obj.Value)
	}
}
