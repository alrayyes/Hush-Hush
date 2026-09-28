package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// issueConsumerToken mints a consumer read token scoped to consumer,
// valid against s.
func issueConsumerToken(t *testing.T, s *store.Store, consumer string) string {
	t.Helper()

	_, token, err := s.CreateConsumerToken(t.Context(), consumer, "test", time.Hour)
	require.NoError(t, err)

	return token
}

func getObjectRequest(slug, token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/objects/"+slug, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestGetObjectWithWriteTokenReturnsStoredCiphertextExactly(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sealed := []byte("sealed-ciphertext")
	require.NoError(t, s.CreateObject(context.Background(), "mattermost_deploy_webhook", sealed, nil, "", ""))

	req := getObjectRequest("mattermost_deploy_webhook", issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, sealed, rec.Body.Bytes())
}

func TestGetObjectWithSessionReturnsStoredCiphertextExactly(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sealed := []byte("sealed-ciphertext")
	require.NoError(t, s.CreateObject(context.Background(), "mattermost_deploy_webhook", sealed, nil, "", ""))
	sessionCookie := seedSession(t, s)

	req := httptest.NewRequest(http.MethodGet, "/objects/mattermost_deploy_webhook", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, sealed, rec.Body.Bytes())
}

func TestGetObjectWithConsumerTokenInScopeReturnsStoredCiphertext(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sealed := []byte("sealed-ciphertext")
	require.NoError(t, s.CreateObject(context.Background(), "mattermost_deploy_webhook", sealed, []string{"homelab"}, "", ""))

	req := getObjectRequest("mattermost_deploy_webhook", issueConsumerToken(t, s, "homelab"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, sealed, rec.Body.Bytes())
}

func TestGetObjectWithConsumerTokenOutOfScopeIsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "mattermost_deploy_webhook", []byte("v"), []string{"homelab"}, "", ""))

	req := getObjectRequest("mattermost_deploy_webhook", issueConsumerToken(t, s, "vps-docker"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetObjectUnknownIDWithValidCredentialReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := getObjectRequest("nope", issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)

	var body hushhush.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Error)
}

func TestGetObjectWithNoCredentialIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "no_credential", []byte("v"), nil, "", ""))

	req := getObjectRequest("no_credential", "")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetObjectWithUnknownTokenIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "unknown_token", []byte("v"), nil, "", ""))

	req := getObjectRequest("unknown_token", "never-issued")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetObjectWithRevokedConsumerTokenIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "revoked_token", []byte("v"), []string{"homelab"}, "", ""))

	ct, token, err := s.CreateConsumerToken(t.Context(), "homelab", "test", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(t.Context(), ct.ID))

	req := getObjectRequest("revoked_token", token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetObjectWithConsumerTokenRecordsConsumerTokenActorAttribution(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "attributed", []byte("v"), []string{"homelab"}, "", ""))

	req := getObjectRequest("attributed", issueConsumerToken(t, s, "homelab"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	entries, err := s.QueryAuditLog(t.Context(), store.AuditLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "consumer_token", entries[0].ActorType)
	require.NotEmpty(t, entries[0].ActorID)
}
