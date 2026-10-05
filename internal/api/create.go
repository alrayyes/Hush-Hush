package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
)

// CreateObjectRequest is the POST /objects body. Matches
// components.schemas.CreateObjectRequest in api/openapi.yaml - Value is
// []byte rather than string because encoding/json already encodes a []byte
// field as base64 on the wire, which is exactly the spec's format: byte.
//
// Slug (json:"slug") is what a caller chooses to address the created
// object by afterward - the internal id the store generates underneath it
// is never part of this request, and never returned as something
// addressable either (specs/secret-objects/spec.md's "Internal id
// decoupled from user-facing slug" requirement, alrayyes/Hush-Hush#383).
//
// KeepReadableCopy opts the creating session's own user into being an
// additional decrypt recipient, alongside whatever consumer recipients
// the client already resolved (specs/secret-objects/spec.md's "Opt-in
// owner-recipient inclusion at create time" requirement). It carries no
// server-side effect beyond being echoed back in the response below: the
// server never decrypts and never adds a sealing recipient itself - the
// client is the one that has to add the owner's public key to Value's
// recipients before sealing it, the same as it already does for every
// resolved consumer. Omitting it (the default) means the owner was not
// added as a recipient.
type CreateObjectRequest struct {
	Slug             string   `json:"slug" maxLength:"128"`
	Value            []byte   `json:"value"`
	UsedBy           []string `json:"used_by,omitempty" maxItems:"100" maxLength:"128"`
	Tags             []string `json:"tags,omitempty"`
	Description      string   `json:"description,omitempty" maxLength:"1000"`
	KeepReadableCopy bool     `json:"keep_readable_copy,omitempty"`
}

// ObjectMetadata is what a successful create, update, or list call
// returns. Matches components.schemas.ObjectMetadata in
// api/openapi.yaml. Slug is the only identifier here - the object's
// internal id is never returned as something a caller could address it
// by (specs/secret-objects/spec.md's "An object's internal id is never
// returned as an addressable value" scenario).
//
// KeepReadableCopy on a create or update response is exactly what that
// same request asked for, nothing derived from storage - the server
// doesn't persist this fact (there's nothing to persist: the sealing
// recipient list itself, which the server never sees the contents of, is
// the only observable effect), so a later GET/List call never carries
// this field at all, only the create/update call that set it.
type ObjectMetadata struct {
	Slug             string   `json:"slug"`
	UsedBy           []string `json:"used_by,omitempty"`
	Tags             []string `json:"tags"`
	Description      string   `json:"description,omitempty"`
	KeepReadableCopy bool     `json:"keep_readable_copy,omitempty"`
	// CreatedAt, UpdatedAt, CreatedBy and UpdatedBy are filled by the list
	// call only, and left out of a create or update response. An actor is
	// also left out when the audit log can't say who it was.
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	CreatedBy *Actor     `json:"created_by,omitempty"`
	UpdatedBy *Actor     `json:"updated_by,omitempty"`
}

// Actor is who performed an audited write. Type is the audit log's own
// actor type, "session" or "token". Matches components.schemas.Actor in
// api/openapi.yaml.
type Actor struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Error is the body every documented error response carries. Matches
// components.schemas.Error in api/openapi.yaml.
type Error struct {
	Error string `json:"error"`
}

// readCreateRequest decodes a create body and checks it. When it isn't
// acceptable it has already answered the caller, and returns false.
func readCreateRequest(w http.ResponseWriter, r *http.Request) (CreateObjectRequest, bool) {
	var req CreateObjectRequest
	if !decodeRequest(w, r, &req) {
		return req, false
	}

	req.UsedBy = uniqueConsumers(req.UsedBy)

	if req.Slug == "" || len(req.Value) == 0 {
		writeError(w, r, http.StatusBadRequest, "slug and value are required")

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

func handleCreateObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, ok := readCreateRequest(w, r)
		if !ok {
			return
		}

		tags, err := createTags(req.Tags)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())

			return
		}

		ownerID, err := s.CurrentUserID(r.Context())
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		err = s.CreateObject(r.Context(), req.Slug, req.Value, req.UsedBy, req.Description, ownerID, store.WithTags(tags))
		switch {
		case err == nil:
			actorType, actorID := actorFrom(r)
			if err := s.RecordAuditLog(r.Context(), req.Slug, store.AuditActionCreate, callerFrom(r), sourceIPFrom(r), actorType, actorID); err != nil {
				writeInternalError(w, r, err)

				return
			}

			writeJSON(w, http.StatusCreated, ObjectMetadata{
				Slug: req.Slug, UsedBy: req.UsedBy, Tags: tags, Description: req.Description,
				KeepReadableCopy: req.KeepReadableCopy,
			})
		case errors.Is(err, store.ErrAlreadyExists):
			writeError(w, r, http.StatusConflict, "object already exists")
		default:
			writeInternalError(w, r, err)
		}
	}
}
