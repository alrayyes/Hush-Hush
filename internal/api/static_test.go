package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// design.md's "Routing boundary" decision: known API routes are handled
// exactly as before, and everything else falls back to the embedded
// SPA's index.html so SvelteKit's own client-side router can take over.

func TestUnmatchedPathServesIndexHTML(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	for _, path := range []string{"/", "/settings", "/login", "/audit-log-page"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, path)
		require.Equal(t, testIndexHTML, rec.Body.String(), path)
	}
}

func TestRealStaticAssetIsServedAsItself(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/_app/version.txt", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "test", rec.Body.String())
	require.NotEqual(t, testIndexHTML, rec.Body.String())
}

func TestKnownAPIRoutesAreUnaffectedByTheStaticFallback(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/objects"},
		{http.MethodGet, "/audit-log"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.NotEqual(t, testIndexHTML, rec.Body.String(), tc.path)
	}
}
