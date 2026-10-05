package api

import (
	"net/http"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
)

// handleListObjects returns every stored object's metadata (slug, used_by,
// description - never the sealed value, and never the internal id), optionally
// narrowed to objects whose used_by lineage includes a given consumer.
// Gated by a write token, unlike every other read path: listing needs no
// slug the caller already holds, so it's a new capability rather than
// something a slug-scoped read already implicitly grants.
func handleListObjects(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := store.ObjectFilter{UsedBy: r.URL.Query().Get("used_by")}

		tags := r.URL.Query()["tag"]
		if len(tags) > maxTagsPerObject {
			writeError(w, r, http.StatusBadRequest, errTooManyTagFilters.Error())

			return
		}

		for _, tag := range tags {
			tag, err := normaliseTag(tag)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, err.Error())

				return
			}

			filter.Tags = append(filter.Tags, tag)
		}

		objs, err := s.ListObjects(r.Context(), filter)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		metadata := make([]ObjectMetadata, len(objs))
		for i, obj := range objs {
			metadata[i] = ObjectMetadata{
				Slug: obj.Slug, UsedBy: obj.UsedBy, Tags: tagsOrEmpty(obj.Tags), Description: obj.Description,
				CreatedAt: timeOrNil(obj.CreatedAt), UpdatedAt: timeOrNil(obj.UpdatedAt),
				CreatedBy: actorOrNil(obj.CreatedBy), UpdatedBy: actorOrNil(obj.UpdatedBy),
			}
		}

		writeJSON(w, http.StatusOK, metadata)
	}
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
}

func actorOrNil(a store.Actor) *Actor {
	if a == (store.Actor{}) {
		return nil
	}

	return &Actor{Type: a.Type, ID: a.ID}
}
