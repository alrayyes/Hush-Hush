package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestListConsumersReturnsEachDistinctNameOnce(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "a", []byte("v"), []string{"homelab/vps-docker", "homelab/mattermost"}, ""))
	require.NoError(t, s.CreateObject(context.Background(), "b", []byte("v"), []string{"homelab/mattermost"}, ""))

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, []string{"homelab/mattermost", "homelab/vps-docker"}, body)
}

func TestListConsumersReturnsEmptyArrayWhenNoneExist(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[]`, rec.Body.String())
}

func TestListConsumersWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListConsumersWithASessionSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func seedConsumersFixture(t *testing.T, s *store.Store) {
	t.Helper()

	require.NoError(t, s.CreateObject(context.Background(), "a", []byte("v"), []string{"homelab/vps-docker", "homelab/mattermost"}, ""))
	require.NoError(t, s.CreateObject(context.Background(), "b", []byte("v"), []string{"homelab/mattermost"}, ""))
	require.NoError(t, s.CreateObject(context.Background(), "c", []byte("v"), []string{"work/ci-runner"}, ""))
}

func TestListConsumersWithQFilterReturnsAPaginatedResponse(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers?q=homelab", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumersPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 2, body.Total)
	require.Equal(t, []hushhush.ConsumerEntry{
		{Name: "homelab/mattermost", SecretCount: 2},
		{Name: "homelab/vps-docker", SecretCount: 1},
	}, body.Consumers)
}

func TestListConsumersWithPageAndPageSizeReturnsOnePage(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers?page=2&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumersPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 3, body.Total)
	require.Equal(t, []hushhush.ConsumerEntry{{Name: "work/ci-runner", SecretCount: 1}}, body.Consumers)
}

func TestListConsumersNoParametersDefaultsToTheUnpaginatedListing(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, []string{"homelab/mattermost", "homelab/vps-docker", "work/ci-runner"}, body)
}

func TestListConsumersWithInvalidPageIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers?page=0", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListConsumersWithInvalidPageSizeIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers?page_size=101", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func renameConsumerRequest(t *testing.T, name, newName, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPatch, "/consumers/"+name,
		bytes.NewReader([]byte(`{"name":"`+newName+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestRenameConsumerUpdatesEveryObjectAndReturnsTheNewEntry(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renameConsumerRequest(t, "homelab/vps-docker", "homelab/vps-docker-2", issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, hushhush.ConsumerEntry{Name: "homelab/vps-docker-2", SecretCount: 1}, body)

	obj, err := s.GetObject(context.Background(), "a")
	require.NoError(t, err)
	require.Contains(t, obj.UsedBy, "homelab/vps-docker-2")
	require.NotContains(t, obj.UsedBy, "homelab/vps-docker")
}

func TestRenameConsumerWithASlashInThePathSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	// The consumer name itself contains a "/" - the route has to treat
	// everything after /consumers/ as the name rather than a path
	// segment boundary (api/openapi.yaml's consumerName parameter).
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renameConsumerRequest(t, "homelab/mattermost", "homelab/mattermost-2", issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestRenameConsumerUnknownNameReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renameConsumerRequest(t, "nonexistent", "new", issueToken(t, s)))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRenameConsumerWithoutNameIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodPatch, "/consumers/homelab/vps-docker", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRenameConsumerWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renameConsumerRequest(t, "homelab/vps-docker", "new", ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionRenamesConsumerWithItsCSRFToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := renameConsumerRequest(t, "homelab/vps-docker", "new", "")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestSessionRenamesConsumerWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)

	req := renameConsumerRequest(t, "homelab/vps-docker", "new", "")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func deleteConsumerRequest(t *testing.T, name, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, "/consumers/"+name, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestDeleteConsumerRemovesItFromEveryObject(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteConsumerRequest(t, "homelab/mattermost", issueToken(t, s)))

	require.Equal(t, http.StatusNoContent, rec.Code)

	consumers, err := s.ListConsumers(context.Background())
	require.NoError(t, err)
	require.NotContains(t, consumers, "homelab/mattermost")

	obj, err := s.GetObject(context.Background(), "a")
	require.NoError(t, err)
	require.NotContains(t, obj.UsedBy, "homelab/mattermost")
}

func TestDeleteConsumerUnknownNameReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteConsumerRequest(t, "nonexistent", issueToken(t, s)))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteConsumerWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteConsumerRequest(t, "homelab/mattermost", ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionDeletesConsumerWithItsCSRFToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := deleteConsumerRequest(t, "homelab/mattermost", "")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

func TestSessionDeletesConsumerWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)

	req := deleteConsumerRequest(t, "homelab/mattermost", "")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func addConsumerRequest(t *testing.T, name, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/consumers",
		bytes.NewReader([]byte(`{"name":"`+name+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestAddConsumerCreatesEntryWithZeroSecrets(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, addConsumerRequest(t, "homelab/new-device", issueToken(t, s)))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var body hushhush.ConsumerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, hushhush.ConsumerEntry{Name: "homelab/new-device", SecretCount: 0}, body)

	consumers, err := s.ListConsumers(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/new-device"}, consumers)
}

func TestAddConsumerDuplicateNameReturnsConflict(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, addConsumerRequest(t, "homelab/mattermost", issueToken(t, s)))

	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestAddConsumerWithoutNameIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/consumers", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAddConsumerWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, addConsumerRequest(t, "homelab/new-device", ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionAddsConsumerWithItsCSRFToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := addConsumerRequest(t, "homelab/new-device", "")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestSessionAddsConsumerWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	req := addConsumerRequest(t, "homelab/new-device", "")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
