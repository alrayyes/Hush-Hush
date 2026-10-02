package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

func TestHealthReportsTheRunningVersion(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var health hushhush.Health
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &health))
	require.Equal(t, "ok", health.Status)
	require.Equal(t, testVersion, health.Version)
}

func healthBody(t *testing.T, mux *http.ServeMux) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	return rec.Body.String()
}

func TestHealthReportsTheInstanceLabelWhenSet(t *testing.T) {
	t.Parallel()

	_, s := newTestMux(t)
	mux := hushhush.NewMux(s, testPublicURL, testWebBuild(), testVersion, hushhush.WithInstanceLabel("prod / homelab"))

	var health hushhush.Health
	require.NoError(t, json.Unmarshal([]byte(healthBody(t, mux)), &health))
	require.Equal(t, "prod / homelab", health.Environment)
}

func TestHealthOmitsTheEnvironmentKeyWhenNoLabelIsSet(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	body := healthBody(t, mux)
	require.NotContains(t, body, "environment")
}

func TestReadyzReportsOKWhenTheStoreIsUsable(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestReadyzReports503WhenTheStoreIsBrokenButHealthzStaysGreen(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.Close())

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"status":"unavailable"}`, rec.Body.String())

	health := httptest.NewRecorder()
	mux.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, health.Code)
}
