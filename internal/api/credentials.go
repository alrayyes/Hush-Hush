package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
)

// errLastCredential is returned when a delete would leave the admin
// account with no way to log in - auth/spec.md's "Refusing to delete the
// last credential" scenario.
var errLastCredential = errors.New("cannot remove the last remaining passkey")

// CredentialRenameRequest is the PATCH /credentials/{id} body. Matches
// components.schemas.CredentialRenameRequest in api/openapi.yaml.
type CredentialRenameRequest struct {
	Nickname string `json:"nickname"`
}

// handleListCredentials returns every registered credential's nickname
// and timestamps - never the public key material.
func handleListCredentials(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		creds, err := s.ListCredentials(r.Context())
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		out := make([]Credential, 0, len(creds))
		for _, c := range creds {
			out = append(out, credentialFromStore(c))
		}

		writeJSON(w, http.StatusOK, out)
	}
}

// handleRenameCredential updates a credential's nickname.
func handleRenameCredential(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CredentialRenameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		id := r.PathValue("id")

		if err := s.RenameCredential(r.Context(), id, req.Nickname); err != nil {
			if errors.Is(err, store.ErrCredentialNotFound) {
				writeError(w, r, http.StatusNotFound, "unknown credential")

				return
			}

			writeInternalError(w, r, err)

			return
		}

		creds, err := s.ListCredentials(r.Context())
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		for _, c := range creds {
			if c.ID == id {
				writeJSON(w, http.StatusOK, credentialFromStore(c))

				return
			}
		}

		writeError(w, r, http.StatusNotFound, "unknown credential")
	}
}

// handleDeleteCredential permanently removes a credential, refusing when
// it's the only one remaining.
func handleDeleteCredential(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if err := deleteCredentialUnlessLast(r, s, id); err != nil {
			switch {
			case errors.Is(err, store.ErrCredentialNotFound):
				writeError(w, r, http.StatusNotFound, "unknown credential")
			case errors.Is(err, errLastCredential):
				writeError(w, r, http.StatusConflict, errLastCredential.Error())
			default:
				writeInternalError(w, r, err)
			}

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// deleteCredentialUnlessLast checks the current credential count before
// deleting, so the admin account is never left with zero passkeys and no
// way to log back in.
func deleteCredentialUnlessLast(r *http.Request, s objectStore, id string) error {
	creds, err := s.ListCredentials(r.Context())
	if err != nil {
		return fmt.Errorf("list credentials: %w", err)
	}

	if len(creds) <= 1 {
		return errLastCredential
	}

	if err := s.DeleteCredential(r.Context(), id); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}

	return nil
}
