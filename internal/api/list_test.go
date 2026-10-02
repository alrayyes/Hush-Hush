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

func TestListObjectsReturnsEveryObjectSortedByID(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "zeta", []byte("v"), nil, "", ""))
	require.NoError(t, s.CreateObject(context.Background(), "alpha", []byte("v"), []string{"homelab/vps-docker"}, "prod deploy webhook", ""))

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body []hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 2)
	require.Equal(t, "alpha", body[0].Slug)
	require.Equal(t, []string{"homelab/vps-docker"}, body[0].UsedBy)
	require.Equal(t, "prod deploy webhook", body[0].Description)
	require.Equal(t, "zeta", body[1].Slug)
}

func TestListObjectsReturnsEmptyArrayWhenNoneExist(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[]`, rec.Body.String())
}

func TestListObjectsFiltersByUsedBy(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "shared_by_two", []byte("v"), []string{"homelab/vps-docker", "homelab/mattermost"}, "", ""))
	require.NoError(t, s.CreateObject(context.Background(), "unrelated", []byte("v"), []string{"homelab/mattermost"}, "", ""))

	req := httptest.NewRequest(http.MethodGet, "/objects?used_by=homelab/vps-docker", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body []hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 1)
	require.Equal(t, "shared_by_two", body[0].Slug)
}

func TestListObjectsWithoutBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListObjectsCarriesWhoCreatedAndLastUpdatedEachObject(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), nil, "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "session", "user-1"))
	require.NoError(t, s.UpdateObject(ctx, "a", []byte("w"), nil))
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionUpdate, "", "203.0.113.2", "token", "tok-1"))

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body []hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 1)
	require.NotNil(t, body[0].CreatedAt)
	require.NotNil(t, body[0].UpdatedAt)
	require.Equal(t, &hushhush.Actor{Type: "session", ID: "user-1"}, body[0].CreatedBy)
	require.Equal(t, &hushhush.Actor{Type: "token", ID: "tok-1"}, body[0].UpdatedBy)
	require.Contains(t, rec.Body.String(), `"created_by":{"type":"session","id":"user-1"}`)
}

func TestListObjectsOmitsActorsForAnObjectWithNoAuditEntries(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "a", []byte("v"), nil, "", ""))

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.NotContains(t, rec.Body.String(), "created_by")
	require.Contains(t, rec.Body.String(), "created_at")
}
