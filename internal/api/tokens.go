package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
)

// errNonPositiveTTL is returned when a token creation request's TTL is
// missing or not greater than zero - tokens/spec.md's "Missing or zero
// TTL is rejected" scenario.
var errNonPositiveTTL = errors.New("ttl_seconds must be a positive integer")

// The token lifetime policy (alrayyes/hush-hush#537): a token lasts 90 days
// unless the request says otherwise, and never more than a year. Retention
// is a rule the API owns, not the UI, so the CLI and SDKs get the same
// answer. api/openapi.yaml states both as `default` and `maximum` on
// ttl_seconds; keep the two in step (a test pins them together).
//
// The maximum applies when a token is minted or rotated. A token issued
// before it existed keeps working until it expires.
//
// Exported so the local `token` subcommands, which mint straight into the
// store, hold the same limit as the HTTP routes.
const (
	DefaultTokenTTL = 90 * 24 * time.Hour
	MaxTokenTTL     = 365 * 24 * time.Hour

	defaultTokenTTLSeconds = int64(DefaultTokenTTL / time.Second)
	maxTokenTTLSeconds     = int64(MaxTokenTTL / time.Second)
)

// resolveTTL turns a request's optional ttl_seconds into a duration. An
// absent value is the default; zero or less is a 400 (the same rule as
// before the default existed, and tokens/spec.md's "Missing or zero TTL is
// rejected" scenario for an explicit zero); above the maximum is a 422 that
// names the limit. status is 0 when the TTL is usable.
func resolveTTL(raw *int64) (ttl time.Duration, status int, message string) {
	switch {
	case raw == nil:
		return time.Duration(defaultTokenTTLSeconds) * time.Second, 0, ""
	case *raw <= 0:
		return 0, http.StatusBadRequest, errNonPositiveTTL.Error()
	case *raw > maxTokenTTLSeconds:
		return 0, http.StatusUnprocessableEntity,
			fmt.Sprintf("ttl_seconds must be at most %d (365 days)", maxTokenTTLSeconds)
	default:
		return time.Duration(*raw) * time.Second, 0, ""
	}
}

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
	// Status and AllowedActions are computed by the server from its own
	// clock, so a client never decides what a token is or may do from its
	// own. Always sent by this server.
	Status         string   `json:"status,omitempty"`
	AllowedActions []string `json:"allowed_actions,omitempty"`
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
	status := store.StatusOfToken(t.Revoked, t.ExpiresAt, time.Now().UTC())

	return TokenMetadata{
		ID:          t.ID,
		Description: t.Description,
		Owner:       t.Owner,
		CreatedAt:   t.CreatedAt,
		ExpiresAt:   t.ExpiresAt,
		Revoked:     t.Revoked,
		LastUsedAt:  t.LastUsedAt,

		Status:         string(status),
		AllowedActions: status.AllowedActions(),
	}
}

// createTokenBody and createConsumerTokenBody are what the two create
// handlers decode into: the request types above with ttl_seconds as a pointer,
// so an absent value (the default) is told apart from an explicit zero (a
// 400). They carry the field limits for those schemas.
type createTokenBody struct {
	Description string `json:"description" maxLength:"200"`
	TTLSeconds  *int64 `json:"ttl_seconds"`
}

type createConsumerTokenBody struct {
	Consumer    string `json:"consumer" maxLength:"128"`
	Description string `json:"description" maxLength:"200"`
	TTLSeconds  *int64 `json:"ttl_seconds"`
}

// handleCreateToken issues a new write bearer token, owned by the admin
// account, returning its raw value exactly once.
func handleCreateToken(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createTokenBody
		if !decodeRequest(w, r, &req) {
			return
		}

		ttl, status, message := resolveTTL(req.TTLSeconds)
		if status != 0 {
			writeError(w, r, status, message)

			return
		}

		wt, value, err := s.CreateWriteToken(r.Context(), req.Description, ttl, adminOwner)
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

		writePage(w, r, out)
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
			TTLSeconds *int64 `json:"ttl_seconds"`
		}
		if !decodeRequest(w, r, &req) {
			return
		}

		ttl, status, message := resolveTTL(req.TTLSeconds)
		if status != 0 {
			writeError(w, r, status, message)

			return
		}

		v, value, err := rotate(r.Context(), id, ttl)
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
	// Status and AllowedActions: see TokenMetadata.
	Status         string   `json:"status,omitempty"`
	AllowedActions []string `json:"allowed_actions,omitempty"`
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
	status := store.StatusOfToken(t.Revoked, t.ExpiresAt, time.Now().UTC())

	return ConsumerTokenMetadata{
		ID:          t.ID,
		Consumer:    t.Consumer,
		Description: t.Description,
		CreatedAt:   t.CreatedAt,
		ExpiresAt:   t.ExpiresAt,
		Revoked:     t.Revoked,
		LastUsedAt:  t.LastUsedAt,

		Status:         string(status),
		AllowedActions: status.AllowedActions(),
	}
}

// handleCreateConsumerToken issues a new consumer read token, scoped to
// the given consumer, returning its raw value exactly once.
func handleCreateConsumerToken(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createConsumerTokenBody
		if !decodeRequest(w, r, &req) {
			return
		}

		if req.Consumer == "" {
			writeError(w, r, http.StatusBadRequest, "consumer is required")

			return
		}

		ttl, status, message := resolveTTL(req.TTLSeconds)
		if status != 0 {
			writeError(w, r, status, message)

			return
		}

		ct, value, err := s.CreateConsumerToken(r.Context(), req.Consumer, req.Description, ttl)
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

		writePage(w, r, out)
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

// handlePurge is handlePurgeToken and handlePurgeConsumerToken's shared
// shape: an unknown id is 404, a still-active token is 409 (a purge is
// only ever allowed once a token is already dead - alrayyes/hush-hush#439),
// otherwise the row is gone and the response is 204.
func handlePurge(purge func(ctx context.Context, id string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		switch err := purge(r.Context(), id); {
		case err == nil:
		case errors.Is(err, store.ErrTokenNotFound):
			writeError(w, r, http.StatusNotFound, "unknown token")

			return
		case errors.Is(err, store.ErrTokenStillActive):
			writeError(w, r, http.StatusConflict, "token must be revoked or expired before it can be purged")

			return
		default:
			writeInternalError(w, r, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// handlePurgeToken permanently removes the write token issued under id,
// once it's already revoked or expired.
func handlePurgeToken(s objectStore) http.HandlerFunc {
	return handlePurge(s.PurgeWriteToken)
}

// handlePurgeConsumerToken permanently removes the consumer token issued
// under id, once it's already revoked or expired.
func handlePurgeConsumerToken(s objectStore) http.HandlerFunc {
	return handlePurge(s.PurgeConsumerToken)
}
