package api

import (
	"context"
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
	LastUsedAt  string `json:"last_used_at,omitempty"`
}

// TokenWithValue is the POST /tokens response - the only response that
// ever carries a token's raw value. Matches
// components.schemas.TokenWithValue.
type TokenWithValue struct {
	TokenMetadata
	Value string `json:"value"`
}

// RotateTokenRequest is the POST /tokens/{id}/rotate body. Matches
// components.schemas.RotateTokenRequest in api/openapi.yaml.
type RotateTokenRequest struct {
	TTLSeconds int64 `json:"ttl_seconds"`
}

func tokenMetadataFromStore(t store.WriteToken) TokenMetadata {
	return TokenMetadata{
		ID:          t.ID,
		Description: t.Description,
		Owner:       t.Owner,
		CreatedAt:   t.CreatedAt,
		ExpiresAt:   t.ExpiresAt,
		Revoked:     t.Revoked,
		LastUsedAt:  t.LastUsedAt,
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

// handleRotate is handleRotateToken and handleRotateConsumerToken's
// shared shape: decode a {ttl_seconds} body (RotateTokenRequest and
// RotateConsumerTokenRequest are identical on the wire, so one anonymous
// struct decodes either), reject a non-positive TTL, call rotate, map
// store.ErrTokenNotFound to 404, and build the 200 response from
// toResponse - an id that's unknown, already revoked, or already
// expired is an error either way: a rotate response promises the caller
// a working new secret, and there's no valid token to hand one to.
func handleRotate[T any](rotate func(ctx context.Context, id string, ttl time.Duration) (T, string, error), toResponse func(T, string) any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			TTLSeconds int64 `json:"ttl_seconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		if req.TTLSeconds <= 0 {
			writeError(w, r, http.StatusBadRequest, errNonPositiveTTL.Error())

			return
		}

		v, value, err := rotate(r.Context(), id, time.Duration(req.TTLSeconds)*time.Second)
		if errors.Is(err, store.ErrTokenNotFound) {
			writeError(w, r, http.StatusNotFound, "unknown token")

			return
		}
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, toResponse(v, value))
	}
}

// handleRotateToken replaces the secret and expiry of the token issued
// under id, returning its new raw value exactly once.
func handleRotateToken(s objectStore) http.HandlerFunc {
	return handleRotate(s.RotateWriteToken, func(wt store.WriteToken, value string) any {
		return TokenWithValue{TokenMetadata: tokenMetadataFromStore(wt), Value: value}
	})
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

// CreateConsumerTokenRequest is the POST /tokens/consumer body. Matches
// components.schemas.CreateConsumerTokenRequest in api/openapi.yaml.
type CreateConsumerTokenRequest struct {
	Consumer    string `json:"consumer"`
	Description string `json:"description"`
	TTLSeconds  int64  `json:"ttl_seconds"`
}

// ConsumerTokenMetadata is a consumer token's HTTP-facing shape without
// its raw value. Matches components.schemas.ConsumerTokenMetadata.
type ConsumerTokenMetadata struct {
	ID          string `json:"id"`
	Consumer    string `json:"consumer"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
	Revoked     bool   `json:"revoked"`
	LastUsedAt  string `json:"last_used_at,omitempty"`
}

// ConsumerTokenWithValue is the POST /tokens/consumer response - the only
// response that ever carries a consumer token's raw value. Matches
// components.schemas.ConsumerTokenWithValue.
type ConsumerTokenWithValue struct {
	ConsumerTokenMetadata
	Value string `json:"value"`
}

// RotateConsumerTokenRequest is the POST /tokens/consumer/{id}/rotate
// body. Matches components.schemas.RotateConsumerTokenRequest.
type RotateConsumerTokenRequest struct {
	TTLSeconds int64 `json:"ttl_seconds"`
}

func consumerTokenMetadataFromStore(t store.ConsumerToken) ConsumerTokenMetadata {
	return ConsumerTokenMetadata{
		ID:          t.ID,
		Consumer:    t.Consumer,
		Description: t.Description,
		CreatedAt:   t.CreatedAt,
		ExpiresAt:   t.ExpiresAt,
		Revoked:     t.Revoked,
		LastUsedAt:  t.LastUsedAt,
	}
}

// handleCreateConsumerToken issues a new consumer read token, scoped to
// the given consumer, returning its raw value exactly once.
func handleCreateConsumerToken(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateConsumerTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		if req.Consumer == "" {
			writeError(w, r, http.StatusBadRequest, "consumer is required")

			return
		}

		if req.TTLSeconds <= 0 {
			writeError(w, r, http.StatusBadRequest, errNonPositiveTTL.Error())

			return
		}

		ct, value, err := s.CreateConsumerToken(r.Context(), req.Consumer, req.Description, time.Duration(req.TTLSeconds)*time.Second)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusCreated, ConsumerTokenWithValue{ConsumerTokenMetadata: consumerTokenMetadataFromStore(ct), Value: value})
	}
}

// handleListConsumerTokens returns every issued consumer token's
// metadata - never a raw value.
func handleListConsumerTokens(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokens, err := s.ListConsumerTokens(r.Context())
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		out := make([]ConsumerTokenMetadata, 0, len(tokens))
		for _, t := range tokens {
			out = append(out, consumerTokenMetadataFromStore(t))
		}

		writeJSON(w, http.StatusOK, out)
	}
}

// handleRotateConsumerToken replaces the secret and expiry of the
// consumer token issued under id, returning its new raw value exactly
// once - same semantics as handleRotateToken, including rejecting an id
// that's unknown, already revoked, or already expired.
func handleRotateConsumerToken(s objectStore) http.HandlerFunc {
	return handleRotate(s.RotateConsumerToken, func(ct store.ConsumerToken, value string) any {
		return ConsumerTokenWithValue{ConsumerTokenMetadata: consumerTokenMetadataFromStore(ct), Value: value}
	})
}

// handleRevokeConsumerToken invalidates the consumer token issued under
// id - same not-an-error semantics as handleRevokeToken for an id that's
// already expired, already revoked, or doesn't exist.
func handleRevokeConsumerToken(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if err := s.RevokeConsumerToken(r.Context(), id); err != nil && !errors.Is(err, store.ErrTokenNotFound) {
			writeInternalError(w, r, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
