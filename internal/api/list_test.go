package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

func TestListObjectsReturnsEveryObjectSortedByID(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "zeta", []byte("v"), nil, ""))
	require.NoError(t, s.CreateObject(context.Background(), "alpha", []byte("v"), []string{"homelab/vps-docker"}, "prod deploy webhook"))

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body []hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 2)
	require.Equal(t, "alpha", body[0].ID)
	require.Equal(t, []string{"homelab/vps-docker"}, body[0].UsedBy)
	require.Equal(t, "prod deploy webhook", body[0].Description)
	require.Equal(t, "zeta", body[1].ID)
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
	require.NoError(t, s.CreateObject(context.Background(), "shared_by_two", []byte("v"), []string{"homelab/vps-docker", "homelab/mattermost"}, ""))
	require.NoError(t, s.CreateObject(context.Background(), "unrelated", []byte("v"), []string{"homelab/mattermost"}, ""))

	req := httptest.NewRequest(http.MethodGet, "/objects?used_by=homelab/vps-docker", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body []hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 1)
	require.Equal(t, "shared_by_two", body[0].ID)
}

func TestListObjectsWithoutBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
