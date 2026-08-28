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

func TestGetObjectUsedByReturnsRecordedConsumers(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "mattermost_deploy_webhook", []byte("v"), []string{"homelab/vps-docker"}))

	req := httptest.NewRequest(http.MethodGet, "/objects/mattermost_deploy_webhook/used-by", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body hushhush.UsedBy
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, []string{"homelab/vps-docker"}, body.UsedBy)
}

func TestGetObjectUsedByUnknownIDReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/objects/nope/used-by", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
