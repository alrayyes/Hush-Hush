package api

import (
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// handleListObjects returns every stored object's metadata (id, used_by,
// description - never the sealed value), optionally narrowed to objects
// whose used_by lineage includes a given consumer. Gated by a write token,
// unlike every other read path: listing needs no id the caller already
// holds, so it's a new capability rather than something an id-scoped read
// already implicitly grants.
func handleListObjects(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := store.ObjectFilter{UsedBy: r.URL.Query().Get("used_by")}

		objs, err := s.ListObjects(r.Context(), filter)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		metadata := make([]ObjectMetadata, len(objs))
		for i, obj := range objs {
			metadata[i] = ObjectMetadata{ID: obj.ID, UsedBy: obj.UsedBy, Description: obj.Description}
		}

		writeJSON(w, http.StatusOK, metadata)
	}
}
