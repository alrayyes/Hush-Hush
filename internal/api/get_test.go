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

func TestGetObjectReturnsStoredCiphertextExactly(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sealed := []byte("sealed-ciphertext")
	require.NoError(t, s.CreateObject(context.Background(), "mattermost_deploy_webhook", sealed, nil, ""))

	req := httptest.NewRequest(http.MethodGet, "/objects/mattermost_deploy_webhook", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, sealed, rec.Body.Bytes())
}

func TestGetObjectUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/objects/nope", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)

	var body hushhush.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Error)
}

func TestGetObjectRequiresNoBearerToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "no_auth_needed", []byte("v"), nil, ""))

	req := httptest.NewRequest(http.MethodGet, "/objects/no_auth_needed", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
