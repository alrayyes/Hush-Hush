package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// UpdateObjectRequest is the PUT /objects/{id} body. Matches
// components.schemas.UpdateObjectRequest in api/openapi.yaml.
type UpdateObjectRequest struct {
	Value []byte `json:"value"`
}

// handleUpdateObject replaces an object's sealed value, leaving its id,
// used_by, and description metadata unchanged - the response carries the
// same shape create does, since both hand back the object's current
// metadata.
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

		err := s.UpdateObject(r.Context(), id, req.Value)
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

		if err := s.RecordAuditLog(r.Context(), id, store.AuditActionUpdate, callerFrom(r), sourceIPFrom(r)); err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, ObjectMetadata{ID: obj.ID, UsedBy: obj.UsedBy, Description: obj.Description})
	}
}
