package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// /objects accepts a valid session as a credential equally valid to the
// write bearer token (openspec/changes/web-ui/design.md's "/objects
// accepts a valid session" decision) - the web UI holds a session, never
// a bearer token, and this is what lets its secrets overview work at
// all. These tests cover that gate directly; create_test.go and friends
// already cover the pre-existing bearer-token path.

func TestSessionListsObjectsWithoutABearerToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestBearerTokenAuthenticatedWriteRecordsLastUsedAt(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	wt, token, err := s.CreateWriteToken(t.Context(), "records usage", 3600e9, "")
	require.NoError(t, err)

	before, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, before, 1)
	require.Empty(t, before[0].LastUsedAt)

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	after, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.Equal(t, wt.ID, after[0].ID)
	require.NotEmpty(t, after[0].LastUsedAt)
}

func TestSessionAuthenticatedWriteDoesNotTouchAnyTokensLastUsedAt(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	_, _, err := s.CreateWriteToken(t.Context(), "untouched", 3600e9, "")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Empty(t, tokens[0].LastUsedAt)
}

func TestSessionCreatesAnObjectWithItsCSRFToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/objects",
		bytes.NewReader([]byte(`{"id":"session_created","value":"c2VhbGVkLWNpcGhlcnRleHQ="}`)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestSessionCreatesAnObjectWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	req := httptest.NewRequest(http.MethodPost, "/objects",
		bytes.NewReader([]byte(`{"id":"no_csrf","value":"c2VhbGVkLWNpcGhlcnRleHQ="}`)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestNeitherBearerTokenNorSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBearerTokenStillCreatesAnObjectWithNoSession(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "token_created", Value: []byte("sealed-ciphertext")}, issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}
