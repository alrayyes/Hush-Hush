package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
)

// errNonPositiveTTL is returned when a token creation request's TTL is
// missing or not greater than zero - tokens/spec.md's "Missing or zero
// TTL is rejected" scenario.
var errNonPositiveTTL = errors.New("ttl_seconds must be a positive integer")

// adminOwner is what a session-created token's owner is recorded as -
// literal, since there's only one admin account (design.md's "single
// admin account, multiple passkeys" decision) to attribute it to.
const adminOwner = "admin"

// CreateTokenRequest is the POST /tokens body. Matches
// components.schemas.CreateTokenRequest in api/openapi.yaml.
type CreateTokenRequest struct {
	Description string `json:"description"`
	TTLSeconds  int64  `json:"ttl_seconds"`
}

// TokenMetadata is a token's HTTP-facing shape without its raw value.
// Matches components.schemas.TokenMetadata.
type TokenMetadata struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Owner       string `json:"owner,omitempty"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
	Revoked     bool   `json:"revoked"`
}

// TokenWithValue is the POST /tokens response - the only response that
// ever carries a token's raw value. Matches
// components.schemas.TokenWithValue.
type TokenWithValue struct {
	TokenMetadata
	Value string `json:"value"`
}

func tokenMetadataFromStore(t store.WriteToken) TokenMetadata {
	return TokenMetadata{
		ID:          t.ID,
		Description: t.Description,
		Owner:       t.Owner,
		CreatedAt:   t.CreatedAt,
		ExpiresAt:   t.ExpiresAt,
		Revoked:     t.Revoked,
	}
}

// handleCreateToken issues a new write bearer token, owned by the admin
// account, returning its raw value exactly once.
func handleCreateToken(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		if req.TTLSeconds <= 0 {
			writeError(w, r, http.StatusBadRequest, errNonPositiveTTL.Error())

			return
		}

		wt, value, err := s.CreateWriteToken(r.Context(), req.Description, time.Duration(req.TTLSeconds)*time.Second, adminOwner)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusCreated, TokenWithValue{TokenMetadata: tokenMetadataFromStore(wt), Value: value})
	}
}

// handleListTokens returns every issued token's metadata - never a raw
// value, which by design no longer exists anywhere to return once a
// token is created.
func handleListTokens(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokens, err := s.ListWriteTokens(r.Context())
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		out := make([]TokenMetadata, 0, len(tokens))
		for _, t := range tokens {
			out = append(out, tokenMetadataFromStore(t))
		}

		writeJSON(w, http.StatusOK, out)
	}
}

// handleRevokeToken invalidates the token issued under id. Revoking an id
// that's already expired, already revoked, or doesn't exist isn't an
// error - tokens/spec.md's "Revoking an already-expired or unknown
// token" scenario.
func handleRevokeToken(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if err := s.RevokeWriteToken(r.Context(), id); err != nil && !errors.Is(err, store.ErrTokenNotFound) {
			writeInternalError(w, r, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
