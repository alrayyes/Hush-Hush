package api_test

import (
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
