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

	return updateRequestWithUsedBy(t, id, value, nil, token)
}

func updateRequestWithUsedBy(t *testing.T, id string, value []byte, usedBy *[]string, token string) *http.Request {
	t.Helper()

	body, err := json.Marshal(hushhush.UpdateObjectRequest{Value: value, UsedBy: usedBy})
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
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), []string{"homelab/vps-docker"}, "", ""))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "mattermost_deploy_webhook", []byte("new"), issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, "mattermost_deploy_webhook", meta.Slug)
	require.Equal(t, []string{"homelab/vps-docker"}, meta.UsedBy)

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []byte("new"), obj.Value)
	require.Equal(t, []string{"homelab/vps-docker"}, obj.UsedBy)
}

func TestUpdateObjectPreservesDescription(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), nil, "prod deploy webhook", ""))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "mattermost_deploy_webhook", []byte("new"), issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, "prod deploy webhook", meta.Description)
}

func TestUpdateObjectReplacesUsedByWhenGiven(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), []string{"homelab/vps-docker"}, "", ""))

	usedBy := []string{"ci", "homelab/nas"}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequestWithUsedBy(t, "mattermost_deploy_webhook", []byte("new"), &usedBy, issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, []string{"ci", "homelab/nas"}, meta.UsedBy)

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []string{"ci", "homelab/nas"}, obj.UsedBy)
}

func TestUpdateObjectCanClearUsedByWithEmptyArray(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), []string{"homelab/vps-docker"}, "", ""))

	empty := []string{}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequestWithUsedBy(t, "mattermost_deploy_webhook", []byte("new"), &empty, issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code)

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Empty(t, obj.UsedBy)
}

// TestUpdateObjectOwnerOptsInToKeepAReadableCopy and
// TestUpdateObjectOwnerRecipientIsNotTheDefault cover specs/secret-
// objects/spec.md's "Opt-in owner-recipient inclusion at create time"
// requirement on the update path too, per tasks.md's 5.1 ("the create
// (and update) request shape") - see create_test.go's own pair of tests
// for why this is a request/response shape check, not proof of what the
// sealed value's recipients actually are (that's the e2e test's job).
func TestUpdateObjectOwnerOptsInToKeepAReadableCopy(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), nil, "", ""))

	body, err := json.Marshal(hushhush.UpdateObjectRequest{Value: []byte("new"), KeepReadableCopy: true})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/objects/mattermost_deploy_webhook", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.True(t, meta.KeepReadableCopy)
}

func TestUpdateObjectOwnerRecipientIsNotTheDefault(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), nil, "", ""))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "mattermost_deploy_webhook", []byte("new"), issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.False(t, meta.KeepReadableCopy)
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
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil, "", ""))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "x", []byte("new"), ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUpdateObjectWithWrongBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil, "", ""))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequest(t, "x", []byte("new"), "wrong-token"))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
