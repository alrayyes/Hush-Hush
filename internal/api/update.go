package api

import (
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// UpdateObjectRequest is the PUT /objects/{slug} body. Matches
// components.schemas.UpdateObjectRequest in api/openapi.yaml. UsedBy is a
// pointer so an absent field decodes to nil - "leave used_by as it is" -
// distinct from an explicit empty array, which clears it
// (alrayyes/hush-hush#299).
type UpdateObjectRequest struct {
	Value  []byte    `json:"value"`
	UsedBy *[]string `json:"used_by,omitempty" maxItems:"100" maxLength:"128"`
	// Tags follows UsedBy's rule: absent leaves them alone, an empty array
	// clears them (alrayyes/hush-hush#500).
	Tags *[]string `json:"tags,omitempty"`
	// KeepReadableCopy is the same per-request owner-recipient opt-in
	// CreateObjectRequest carries - see its own doc comment. An update
	// reseals the whole value from scratch, so this is evaluated fresh
	// each time, independent of whatever an earlier create or update on
	// this same object requested.
	KeepReadableCopy bool `json:"keep_readable_copy,omitempty"`
}

// readUpdateRequest decodes an update body and checks it. When it isn't
// acceptable it has already answered the caller, and returns false.
func readUpdateRequest(w http.ResponseWriter, r *http.Request) (UpdateObjectRequest, bool) {
	var req UpdateObjectRequest
	if !decodeRequest(w, r, &req) {
		return req, false
	}

	req.UsedBy = uniqueConsumersPtr(req.UsedBy)

	if len(req.Value) == 0 {
		writeError(w, r, http.StatusBadRequest, "value is required")

		return req, false
	}

	if len(req.Value) > MaxValueBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, errValueTooLarge.Error())

		return req, false
	}

	if err := validateAgeCiphertext(req.Value); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, err.Error())

		return req, false
	}

	return req, true
}

// handleUpdateObject replaces an object's sealed value, leaving its slug and
// description metadata unchanged. used_by is left unchanged too unless the
// request includes it, in which case it fully replaces the object's
// recorded consumers. The response carries the same shape create does,
// since both hand back the object's current metadata.
func handleUpdateObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, ok := readUpdateRequest(w, r)
		if !ok {
			return
		}

		opts, err := updateTagOptions(req.Tags)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())

			return
		}

		slug := r.PathValue("slug")

		err = s.UpdateObject(r.Context(), slug, req.Value, req.UsedBy, opts...)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "unknown object")

			return
		default:
			writeInternalError(w, r, err)

			return
		}

		obj, err := s.GetObject(r.Context(), slug)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		actorType, actorID := actorFrom(r)
		if err := s.RecordAuditLog(r.Context(), slug, store.AuditActionUpdate, callerFrom(r), sourceIPFrom(r), actorType, actorID); err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, ObjectMetadata{
			Slug: obj.Slug, UsedBy: obj.UsedBy, Tags: tagsOrEmpty(obj.Tags), Description: obj.Description,
			KeepReadableCopy: req.KeepReadableCopy,
		})
	}
}
