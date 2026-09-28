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

// tokenRequest builds a request against sess's session, adding its CSRF
// header only when withCSRF is true - a test exercising the missing-CSRF
// rejection path needs to omit it.
func tokenRequest(t *testing.T, method, path string, body []byte, sess *http.Cookie, csrfToken string, withCSRF bool) *http.Request {
	t.Helper()

	var r *bytes.Reader
	if body != nil {
		r = bytes.NewReader(body)
	} else {
		r = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(sess)
	if withCSRF {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	return req
}

func TestCreateTokenReturnsItsRawValueOnce(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.CreateTokenRequest{Description: "web UI test token", TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/tokens", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created hushhush.TokenWithValue
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Value)
	require.Equal(t, "admin", created.Owner)
	require.False(t, created.Revoked)

	listReq := httptest.NewRequest(http.MethodGet, "/tokens", nil)
	listReq.AddCookie(sessionCookie)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	require.NotContains(t, listRec.Body.String(), created.Value)
}

func TestCreateTokenRejectsNonPositiveTTL(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.CreateTokenRequest{Description: "no ttl", TTLSeconds: 0})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/tokens", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestCreateTokenWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	body, err := json.Marshal(hushhush.CreateTokenRequest{Description: "no csrf", TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/tokens", body, sessionCookie, "", false)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListTokensShowsHTTPAndCLICreatedTokensWithOwnership(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	_, _, err = s.CreateWriteToken(t.Context(), "cli token", 3600e9, "")
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.CreateTokenRequest{Description: "http token", TTLSeconds: 3600})
	require.NoError(t, err)
	createReq := tokenRequest(t, http.MethodPost, "/tokens", body, sessionCookie, sess.CSRFToken, true)
	mux.ServeHTTP(httptest.NewRecorder(), createReq)

	req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var tokens []hushhush.TokenMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokens))
	require.Len(t, tokens, 2)

	byDescription := make(map[string]hushhush.TokenMetadata, len(tokens))
	for _, tok := range tokens {
		byDescription[tok.Description] = tok
	}
	require.Empty(t, byDescription["cli token"].Owner)
	require.Equal(t, "admin", byDescription["http token"].Owner)
}

func TestRevokeTokenInvalidatesItAndStaysListed(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	wt, token, err := s.CreateWriteToken(t.Context(), "to revoke", 3600e9, "admin")
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodDelete, "/tokens/"+wt.ID, nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	valid, err := s.ValidateWriteToken(t.Context(), token)
	require.NoError(t, err)
	require.False(t, valid)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.True(t, tokens[0].Revoked)
	require.Equal(t, "to revoke", tokens[0].Description)
}

func TestRevokeUnknownTokenSucceedsWithoutError(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodDelete, "/tokens/does-not-exist", nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRotateTokenReturnsANewRawValueAndKeepsIdentity(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	wt, oldToken, err := s.CreateWriteToken(t.Context(), "to rotate", 3600e9, "admin")
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.RotateTokenRequest{TTLSeconds: 7200})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/tokens/"+wt.ID+"/rotate", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var rotated hushhush.TokenWithValue
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rotated))
	require.NotEmpty(t, rotated.Value)
	require.NotEqual(t, oldToken, rotated.Value)
	require.Equal(t, wt.ID, rotated.ID)
	require.Equal(t, "to rotate", rotated.Description)
	require.Equal(t, "admin", rotated.Owner)

	oldValid, err := s.ValidateWriteToken(t.Context(), oldToken)
	require.NoError(t, err)
	require.False(t, oldValid)

	newValid, err := s.ValidateWriteToken(t.Context(), rotated.Value)
	require.NoError(t, err)
	require.True(t, newValid)
}

func TestRotateTokenRejectsNonPositiveTTL(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	wt, _, err := s.CreateWriteToken(t.Context(), "to rotate", 3600e9, "admin")
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.RotateTokenRequest{TTLSeconds: 0})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/tokens/"+wt.ID+"/rotate", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRotateUnknownTokenIsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.RotateTokenRequest{TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/tokens/does-not-exist/rotate", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRotateRevokedTokenIsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	wt, _, err := s.CreateWriteToken(t.Context(), "to revoke then rotate", 3600e9, "admin")
	require.NoError(t, err)
	require.NoError(t, s.RevokeWriteToken(t.Context(), wt.ID))

	body, err := json.Marshal(hushhush.RotateTokenRequest{TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/tokens/"+wt.ID+"/rotate", body, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRotateTokenWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	wt, _, err := s.CreateWriteToken(t.Context(), "to rotate", 3600e9, "admin")
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.RotateTokenRequest{TTLSeconds: 3600})
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, "/tokens/"+wt.ID+"/rotate", body, sessionCookie, sess.CSRFToken, false)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRotateTokenWithoutASessionIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/tokens/does-not-exist/rotate", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListTokensWithoutASessionIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPurgeTokenRemovesARevokedToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	wt, _, err := s.CreateWriteToken(t.Context(), "to purge", 3600e9, "admin")
	require.NoError(t, err)
	require.NoError(t, s.RevokeWriteToken(t.Context(), wt.ID))

	req := tokenRequest(t, http.MethodDelete, "/tokens/"+wt.ID+"/purge", nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestPurgeTokenRejectsAnActiveToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	wt, _, err := s.CreateWriteToken(t.Context(), "still active", 3600e9, "admin")
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodDelete, "/tokens/"+wt.ID+"/purge", nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
}

func TestPurgeUnknownTokenIsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodDelete, "/tokens/does-not-exist/purge", nil, sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPurgeTokenWithoutASessionIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodDelete, "/tokens/does-not-exist/purge", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
