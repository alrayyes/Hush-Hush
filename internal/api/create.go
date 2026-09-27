package api

import (
	"encoding/json"
	"errors"
	"net/http"

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
type CreateObjectRequest struct {
	Slug        string   `json:"slug"`
	Value       []byte   `json:"value"`
	UsedBy      []string `json:"used_by,omitempty"`
	Description string   `json:"description,omitempty"`
}

// ObjectMetadata is what a successful create, update, or list call
// returns. Matches components.schemas.ObjectMetadata in
// api/openapi.yaml. Slug is the only identifier here - the object's
// internal id is never returned as something a caller could address it
// by (specs/secret-objects/spec.md's "An object's internal id is never
// returned as an addressable value" scenario).
type ObjectMetadata struct {
	Slug        string   `json:"slug"`
	UsedBy      []string `json:"used_by,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Error is the body every documented error response carries. Matches
// components.schemas.Error in api/openapi.yaml.
type Error struct {
	Error string `json:"error"`
}

func handleCreateObject(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateObjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		if req.Slug == "" || len(req.Value) == 0 {
			writeError(w, r, http.StatusBadRequest, "slug and value are required")

			return
		}

		err := s.CreateObject(r.Context(), req.Slug, req.Value, req.UsedBy, req.Description)
		switch {
		case err == nil:
			actorType, actorID := actorFrom(r)
			if err := s.RecordAuditLog(r.Context(), req.Slug, store.AuditActionCreate, callerFrom(r), sourceIPFrom(r), actorType, actorID); err != nil {
				writeInternalError(w, r, err)

				return
			}

			writeJSON(w, http.StatusCreated, ObjectMetadata{Slug: req.Slug, UsedBy: req.UsedBy, Description: req.Description})
		case errors.Is(err, store.ErrAlreadyExists):
			writeError(w, r, http.StatusConflict, "object already exists")
		default:
			writeInternalError(w, r, err)
		}
	}
}
