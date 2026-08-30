package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func deleteRequest(t *testing.T, id, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, "/objects/"+id, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestDeleteObjectRemovesItAndSubsequentGetReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "mattermost_deploy_webhook", []byte("v"), nil))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteRequest(t, "mattermost_deploy_webhook", issueToken(t, s)))

	require.Equal(t, http.StatusNoContent, rec.Code)

	_, err := s.GetObject(context.Background(), "mattermost_deploy_webhook")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestDeleteObjectUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteRequest(t, "nope", issueToken(t, s)))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteObjectWithoutBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteRequest(t, "x", ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDeleteObjectWithWrongBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteRequest(t, "x", "wrong-token"))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
