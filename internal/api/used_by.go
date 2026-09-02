package api

import (
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// UsedBy is the used-by-query response. Matches components.schemas.UsedBy
// in api/openapi.yaml, which marks used_by required - unlike
// ObjectMetadata's own optional used_by, this is the field the whole
// response exists for, so it's always present, empty array included.
type UsedBy struct {
	UsedBy []string `json:"used_by"`
}

// handleGetObjectUsedBy returns an object's recorded used_by lineage -
// unauthenticated, same as the get endpoint it shares a store call with.
func handleGetObjectUsedBy(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		obj, err := s.GetObject(r.Context(), r.PathValue("id"))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "unknown object")

			return
		}

		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		usedBy := obj.UsedBy
		if usedBy == nil {
			usedBy = []string{}
		}

		writeJSON(w, http.StatusOK, UsedBy{UsedBy: usedBy})
	}
}
