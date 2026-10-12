package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// An internal tool must not show up in search results, so every response
// says so (alrayyes/Hush-Hush#834), whatever the handler underneath does.
func TestNoIndexSetsRobotsHeaderOnEveryResponse(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"ok":       func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hi")) },
		"error":    func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", http.StatusUnauthorized) },
		"redirect": func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/x", http.StatusFound) },
		"empty":    func(http.ResponseWriter, *http.Request) {},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			hushhush.NoIndex(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			require.Equal(t, "noindex, nofollow", rec.Header().Get("X-Robots-Tag"))
		})
	}
}
