package api

import (
	"net/http"
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
func handleGetObjectUsedBy(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opts, ok := selectorFrom(w, r)
		if !ok {
			return
		}

		obj, err := s.GetObject(r.Context(), r.PathValue("slug"), opts...)
		if objectLookupFailed(w, r, err) {
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

// uniqueConsumers drops a consumer named twice, keeping the first of each in
// order. The store keys a used_by row on object and consumer, so a repeat would
// be a constraint failure, and a repeat means nothing a caller would want
// refused (alrayyes/hush-hush#646). A nil list stays nil: absent and empty are
// different on update.
func uniqueConsumers(consumers []string) []string {
	if consumers == nil {
		return nil
	}

	seen := make(map[string]struct{}, len(consumers))
	out := make([]string, 0, len(consumers))

	for _, c := range consumers {
		if _, dup := seen[c]; dup {
			continue
		}

		seen[c] = struct{}{}
		out = append(out, c)
	}

	return out
}

// uniqueConsumersPtr is uniqueConsumers for an update's optional list, where
// nil means "leave it alone".
func uniqueConsumersPtr(consumers *[]string) *[]string {
	if consumers == nil {
		return nil
	}

	unique := uniqueConsumers(*consumers)

	return &unique
}
