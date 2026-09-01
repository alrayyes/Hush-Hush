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
type CreateObjectRequest struct {
	ID          string   `json:"id"`
	Value       []byte   `json:"value"`
	UsedBy      []string `json:"used_by,omitempty"`
	Description string   `json:"description,omitempty"`
}

// ObjectMetadata is what a successful create or update returns. Matches
// components.schemas.ObjectMetadata in api/openapi.yaml.
type ObjectMetadata struct {
	ID          string   `json:"id"`
	UsedBy      []string `json:"used_by,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Error is the body every documented error response carries. Matches
// components.schemas.Error in api/openapi.yaml.
type Error struct {
	Error string `json:"error"`
}

func handleCreateObject(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateObjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		if req.ID == "" || len(req.Value) == 0 {
			writeError(w, r, http.StatusBadRequest, "id and value are required")

			return
		}

		err := s.CreateObject(r.Context(), req.ID, req.Value, req.UsedBy, req.Description)
		switch {
		case err == nil:
			if err := s.RecordAuditLog(r.Context(), req.ID, store.AuditActionCreate, callerFrom(r), sourceIPFrom(r)); err != nil {
				writeInternalError(w, r, err)

				return
			}

			writeJSON(w, http.StatusCreated, ObjectMetadata{ID: req.ID, UsedBy: req.UsedBy, Description: req.Description})
		case errors.Is(err, store.ErrAlreadyExists):
			writeError(w, r, http.StatusConflict, "object already exists")
		default:
			writeInternalError(w, r, err)
		}
	}
}
