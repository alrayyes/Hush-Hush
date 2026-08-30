package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

func updateRequest(t *testing.T, id string, value []byte, token string) *http.Request {
	t.Helper()

	body, err := json.Marshal(hushhush.UpdateObjectRequest{Value: value})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/objects/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestUpdateObjectReplacesValuePreservingIDAndUsedBy(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), []string{"homelab/vps-docker"}))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "mattermost_deploy_webhook", []byte("new"), issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, "mattermost_deploy_webhook", meta.ID)
	require.Equal(t, []string{"homelab/vps-docker"}, meta.UsedBy)

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []byte("new"), obj.Value)
	require.Equal(t, []string{"homelab/vps-docker"}, obj.UsedBy)
}

func TestUpdateObjectUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "nope", []byte("v"), issueToken(t, s)))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateObjectWithoutBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "x", []byte("new"), ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUpdateObjectWithWrongBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "x", []byte("new"), "wrong-token"))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
