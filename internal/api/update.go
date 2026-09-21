package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// UpdateObjectRequest is the PUT /objects/{id} body. Matches
// components.schemas.UpdateObjectRequest in api/openapi.yaml. UsedBy is a
// pointer so an absent field decodes to nil - "leave used_by as it is" -
// distinct from an explicit empty array, which clears it
// (alrayyes/hush-hush#299).
type UpdateObjectRequest struct {
	Value  []byte    `json:"value"`
	UsedBy *[]string `json:"used_by,omitempty"`
}

// handleUpdateObject replaces an object's sealed value, leaving its id and
// description metadata unchanged. used_by is left unchanged too unless the
// request includes it, in which case it fully replaces the object's
// recorded consumers. The response carries the same shape create does,
// since both hand back the object's current metadata.
func handleUpdateObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req UpdateObjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		if len(req.Value) == 0 {
			writeError(w, r, http.StatusBadRequest, "value is required")

			return
		}

		id := r.PathValue("id")

		err := s.UpdateObject(r.Context(), id, req.Value, req.UsedBy)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "unknown object")

			return
		default:
			writeInternalError(w, r, err)

			return
		}

		obj, err := s.GetObject(r.Context(), id)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		actorType, actorID := actorFrom(r)
		if err := s.RecordAuditLog(r.Context(), id, store.AuditActionUpdate, callerFrom(r), sourceIPFrom(r), actorType, actorID); err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, ObjectMetadata{ID: obj.ID, UsedBy: obj.UsedBy, Description: obj.Description})
	}
}
