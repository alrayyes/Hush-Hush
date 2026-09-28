package api

import (
	"errors"
	"net/http"
	"slices"

	"github.com/alrayyes/hush-hush/internal/store"
)

// handleGetObject returns an object's stored ciphertext exactly as
// sealed, to a caller requireReadAccess has already authenticated as a
// write token, a session, or a consumer token
// (openspec/changes/consumer-read-tokens/proposal.md - this partially
// supersedes the earlier fully-unauthenticated design). A consumer
// token additionally needs the requested object's used_by to include its
// bound consumer, checked here rather than in requireReadAccess since
// that's the point this handler first knows which object was requested.
func handleGetObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")

		obj, err := s.GetObject(r.Context(), slug)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "unknown object")

			return
		}

		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		// A consumer token outside its scope reads as not-found, not
		// forbidden - indistinguishable from an unknown slug, so it can't
		// be used to probe which other slugs exist (ADR 7's enumeration
		// concern; specs/secret-objects/spec.md's "A consumer token
		// outside its scope reads as not found" requirement).
		if auth, ok := r.Context().Value(consumerTokenContextKey{}).(consumerTokenAuth); ok && !slices.Contains(obj.UsedBy, auth.consumer) {
			writeError(w, r, http.StatusNotFound, "unknown object")

			return
		}

		actorType, actorID := actorFrom(r)
		if err := s.RecordAuditLog(r.Context(), slug, store.AuditActionRead, callerFrom(r), sourceIPFrom(r), actorType, actorID); err != nil {
			writeInternalError(w, r, err)

			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(obj.Value)
	}
}
