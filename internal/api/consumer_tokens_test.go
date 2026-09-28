package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

func TestCreateConsumerTokenReturnsItsRawValueOnce(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.CreateConsumerTokenRequest{Consumer: "homelab", Description: "deploy read", TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/consumer-tokens", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created hushhush.ConsumerTokenWithValue
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Value)
	require.Equal(t, "homelab", created.Consumer)
	require.False(t, created.Revoked)

	listReq := httptest.NewRequest(http.MethodGet, "/consumer-tokens", nil)
	listReq.AddCookie(sessionCookie)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	require.NotContains(t, listRec.Body.String(), created.Value)
}

func TestCreateConsumerTokenRejectsNonPositiveTTL(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.CreateConsumerTokenRequest{Consumer: "homelab", Description: "no ttl", TTLSeconds: 0})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/consumer-tokens", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestCreateConsumerTokenRejectsMissingConsumer(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.CreateConsumerTokenRequest{Description: "no consumer", TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/consumer-tokens", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestCreateConsumerTokenWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	body, err := json.Marshal(hushhush.CreateConsumerTokenRequest{Consumer: "homelab", Description: "no csrf", TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/consumer-tokens", body, sessionCookie, "", false)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// A write bearer token holder can mint and manage consumer read tokens
// over HTTP without a cookie session - alrayyes/hush-hush#467. A write
// token already reads/writes any object; authorizing it to also issue a
// consumer token, itself scoped to reading a single consumer's objects,
// is a narrowing delegation, not an escalation.

func TestCreateConsumerTokenWithBearerTokenAndNoSessionSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	body, err := json.Marshal(hushhush.CreateConsumerTokenRequest{Consumer: "homelab", Description: "minted by bearer", TTLSeconds: 3600})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/consumer-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created hushhush.ConsumerTokenWithValue
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Value)
	require.Equal(t, "homelab", created.Consumer)
}

func TestListConsumerTokensWithBearerTokenAndNoSessionSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	_, _, err := s.CreateConsumerToken(t.Context(), "homelab", "listed by bearer", 3600e9)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/consumer-tokens", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var tokens []hushhush.ConsumerTokenMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokens))
	require.Len(t, tokens, 1)
}

func TestRevokeConsumerTokenWithBearerTokenAndNoSessionSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "revoked by bearer", 3600e9)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/consumer-tokens/"+ct.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRotateConsumerTokenWithBearerTokenAndNoSessionSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "rotated by bearer", 3600e9)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.RotateConsumerTokenRequest{TTLSeconds: 7200})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/consumer-tokens/"+ct.ID+"/rotate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestPurgeConsumerTokenWithBearerTokenAndNoSessionSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "purged by bearer", 3600e9)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	req := httptest.NewRequest(http.MethodDelete, "/consumer-tokens/"+ct.ID+"/purge", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestListConsumerTokensWithoutASessionIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumer-tokens", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRevokeConsumerTokenInvalidatesItAndStaysListed(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	ct, token, err := s.CreateConsumerToken(t.Context(), "homelab", "to revoke", 3600e9)
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodDelete, "/consumer-tokens/"+ct.ID, nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	_, _, valid, err := s.AuthenticateConsumerToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.True(t, tokens[0].Revoked)
}

func TestRevokeUnknownConsumerTokenSucceedsWithoutError(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodDelete, "/consumer-tokens/does-not-exist", nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRotateConsumerTokenReturnsANewRawValueAndKeepsIdentity(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	ct, oldToken, err := s.CreateConsumerToken(t.Context(), "homelab", "to rotate", 3600e9)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.RotateConsumerTokenRequest{TTLSeconds: 7200})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/consumer-tokens/"+ct.ID+"/rotate", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var rotated hushhush.ConsumerTokenWithValue
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rotated))
	require.NotEmpty(t, rotated.Value)
	require.NotEqual(t, oldToken, rotated.Value)
	require.Equal(t, ct.ID, rotated.ID)
	require.Equal(t, "homelab", rotated.Consumer)

	_, _, oldValid, err := s.AuthenticateConsumerToken(t.Context(), oldToken)
	require.NoError(t, err)
	require.False(t, oldValid)

	_, _, newValid, err := s.AuthenticateConsumerToken(t.Context(), rotated.Value)
	require.NoError(t, err)
	require.True(t, newValid)
}

func TestRotateConsumerTokenRejectsNonPositiveTTL(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "to rotate", 3600e9)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.RotateConsumerTokenRequest{TTLSeconds: 0})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/consumer-tokens/"+ct.ID+"/rotate", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRotateUnknownConsumerTokenIsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.RotateConsumerTokenRequest{TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/consumer-tokens/does-not-exist/rotate", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRotateRevokedConsumerTokenIsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "to revoke then rotate", 3600e9)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	body, err := json.Marshal(hushhush.RotateConsumerTokenRequest{TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/consumer-tokens/"+ct.ID+"/rotate", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRotateConsumerTokenWithoutASessionIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/consumer-tokens/does-not-exist/rotate", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPurgeConsumerTokenRemovesARevokedToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "to purge", 3600e9)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	req := tokenRequest(t, http.MethodDelete, "/consumer-tokens/"+ct.ID+"/purge", nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestPurgeConsumerTokenRejectsAnActiveToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "still active", 3600e9)
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodDelete, "/consumer-tokens/"+ct.ID+"/purge", nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)

	tokens, err := s.ListConsumerTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
}

func TestPurgeUnknownConsumerTokenIsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodDelete, "/consumer-tokens/does-not-exist/purge", nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPurgeConsumerTokenWithoutASessionIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodDelete, "/consumer-tokens/does-not-exist/purge", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
