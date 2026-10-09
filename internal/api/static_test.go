package api_test

import (
	"compress/gzip"
	"io"
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

// alrayyes/hush-hush#272: /audit-log is both a page route and a real API
// endpoint. A hard navigation (refresh, bookmark, Playwright's page.goto)
// sets Sec-Fetch-Dest: document, a header the SPA's own fetch() call
// never sends - that's the signal the fix relies on to tell the two
// apart without moving the published API path (docs/adr/0015).
func TestHardNavigationToAuditLogServesTheSPA(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/audit-log", nil)
	req.Header.Set("Sec-Fetch-Dest", "document")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, testIndexHTML, rec.Body.String())
}

// alrayyes/hush-hush#295: /consumers is the same class of collision as
// #272's /audit-log - a SvelteKit page route and a real API endpoint
// sharing one path. Same fix, same signal.
func TestHardNavigationToConsumersServesTheSPA(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.Header.Set("Sec-Fetch-Dest", "document")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, testIndexHTML, rec.Body.String())
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
		{http.MethodGet, "/consumers"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.NotEqual(t, testIndexHTML, rec.Body.String(), tc.path)
	}
}

// alrayyes/hush-hush#721: content-hashed assets are cached for a year, HTML is
// revalidated, and text is compressed for clients that ask.

func TestHashedAssetIsCachedForeverAndImmutable(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/chunks/app.Ab12Cd34.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestHTMLIsRevalidatedNotCached(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	for _, path := range []string{"/", "/login", "/index.html"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"), path)
	}
}

func TestUnhashedStaticFileIsRevalidated(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/_app/version.txt", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
}

func TestTextAssetIsGzippedWhenTheClientAsks(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/chunks/app.Ab12Cd34.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

func TestGzippedBodyDecodesToTheOriginal(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/chunks/app.Ab12Cd34.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	zr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)

	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	require.Equal(t, testHashedScript, string(body))
}

func TestAssetIsNotCompressedWithoutAcceptEncoding(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/chunks/app.Ab12Cd34.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Empty(t, rec.Header().Get("Content-Encoding"))
}

func TestCompressedResponseVariesOnAcceptEncoding(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/chunks/app.Ab12Cd34.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Contains(t, rec.Header().Values("Vary"), "Accept-Encoding")
}
