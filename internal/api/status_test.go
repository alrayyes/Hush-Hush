package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

func TestAuthStatusReportsNoAdminAccountOnAFreshInstall(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var status hushhush.AuthStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	require.False(t, status.Bootstrapped)
}

func TestAuthStatusReportsAnAdminAccountOnceOneExists(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	registerCredential(t, mux, nil, "first")

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var status hushhush.AuthStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	require.True(t, status.Bootstrapped)
}

func TestAuthStatusSetsNoCookies(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Empty(t, rec.Result().Cookies())
}
