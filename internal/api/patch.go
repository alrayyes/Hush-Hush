package api

import (
	"errors"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

var (
	errPatchNothing = errors.New("at least one of description, slug or tags is required")
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

// buildObjectPatch checks a patch request against slug, the name it applies
// to, and turns it into the store's form. The error is the 400 message a
// caller sees, over HTTP or as a tool result.
func buildObjectPatch(req PatchObjectRequest, slug string) (store.ObjectPatch, error) {
	if req.Description == nil && req.Slug == nil && req.Tags == nil {
		return store.ObjectPatch{}, errPatchNothing
	}

	// A slug equal to the current one is no rename, and a name stored before
	// the rule may not follow it.
	if req.Slug != nil && *req.Slug != slug && !validSlug(*req.Slug) {
		return store.ObjectPatch{}, errInvalidSlug
	}

	patch := store.ObjectPatch{Description: req.Description, Slug: req.Slug}

	if req.Tags != nil {
		tags, err := normaliseTags(*req.Tags)
		if err != nil {
			return store.ObjectPatch{}, err
		}

		patch.Tags = &tags
	}

	return patch, nil
}

// readPatchRequest decodes a patch body and checks it. When it isn't
// acceptable it has already answered the caller, and returns false.
func readPatchRequest(w http.ResponseWriter, r *http.Request) (store.ObjectPatch, bool) {
	var req PatchObjectRequest
	if !decodeRequest(w, r, &req) {
		return store.ObjectPatch{}, false
	}

	patch, err := buildObjectPatch(req, r.PathValue("slug"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())

		return store.ObjectPatch{}, false
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
