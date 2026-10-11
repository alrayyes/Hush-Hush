package api

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/alrayyes/hush-hush/internal/store"
)

// slugPattern is components.schemas.ObjectSlug's pattern in api/openapi.yaml.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

var (
	errPatchNothing = errors.New("at least one of description, slug or tags is required")
	errInvalidSlug  = errors.New("slug must start with a-z or 0-9 and use only a-z, 0-9, '_' and '-'")
	errSlugTaken    = errors.New("an object with that slug already exists")
)

// PatchObjectRequest is the PATCH /objects/{slug} body. Matches
// components.schemas.PatchObjectRequest in api/openapi.yaml. Every field is
// a pointer so an absent one decodes to nil, "leave it", distinct from an
// empty description or tag list, which clears it.
type PatchObjectRequest struct {
	Description *string   `json:"description,omitempty" maxLength:"1000"`
	Slug        *string   `json:"slug,omitempty" maxLength:"128"`
	Tags        *[]string `json:"tags,omitempty"`
}

// readPatchRequest decodes a patch body and checks it. When it isn't
// acceptable it has already answered the caller, and returns false.
func readPatchRequest(w http.ResponseWriter, r *http.Request) (store.ObjectPatch, bool) {
	var req PatchObjectRequest
	if !decodeRequest(w, r, &req) {
		return store.ObjectPatch{}, false
	}

	if req.Description == nil && req.Slug == nil && req.Tags == nil {
		writeError(w, r, http.StatusBadRequest, errPatchNothing.Error())

		return store.ObjectPatch{}, false
	}

	if req.Slug != nil && !slugPattern.MatchString(*req.Slug) {
		writeError(w, r, http.StatusBadRequest, errInvalidSlug.Error())

		return store.ObjectPatch{}, false
	}

	patch := store.ObjectPatch{Description: req.Description, Slug: req.Slug}

	if req.Tags != nil {
		tags, err := normaliseTags(*req.Tags)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())

			return store.ObjectPatch{}, false
		}

		patch.Tags = &tags
	}

	return patch, true
}

// handlePatchObject edits an object's description, tags and slug without
// re-sealing its value, so fixing a typo or reorganising names never needs the
// plaintext again. A rename moves every variant of the name; the optional id
// query parameter picks which variant description and tags apply to, and which
// one the response describes, exactly as it does for PUT. The response is the
// same metadata shape create and update hand back.
func handlePatchObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		patch, ok := readPatchRequest(w, r)
		if !ok {
			return
		}

		selector, ok := selectorFrom(w, r)
		if !ok {
			return
		}

		id, slug, err := s.PatchObject(r.Context(), r.PathValue("slug"), patch, selector...)
		if errors.Is(err, store.ErrAlreadyExists) {
			writeError(w, r, http.StatusConflict, errSlugTaken.Error())

			return
		}

		if objectLookupFailed(w, r, err) {
			return
		}

		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		obj, err := s.GetObject(r.Context(), slug, store.WithID(id))
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		actorType, actorID := actorFrom(r)
		if err := s.RecordAuditLog(r.Context(), slug, store.AuditActionUpdate, callerFrom(r), sourceIPFrom(r), actorType, actorID, store.AuditVariant(id)); err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, ObjectMetadata{
			ID: obj.ID, Slug: obj.Slug, UsedBy: obj.UsedBy, Tags: tagsOrEmpty(obj.Tags), Description: obj.Description,
		})
	}
}
