package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// The list endpoints that return every row can be paged with limit and offset
// (alrayyes/hush-hush#649). Asking for nothing is unchanged, so no client
// breaks, and the total count is always in X-Total-Count.

type pagedEndpoint struct {
	name string
	path string
	// seed stores n rows and returns a function that sends a GET with a query.
	seed func(t *testing.T, n int) func(query string) *httptest.ResponseRecorder
}

func sessionGetter(t *testing.T, mux http.Handler, s *store.Store, path string) func(string) *httptest.ResponseRecorder {
	t.Helper()

	cookie := seedSession(t, s)

	return func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path+query, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		return rec
	}
}

func pagedEndpoints() []pagedEndpoint {
	return []pagedEndpoint{
		{"objects", "/objects", func(t *testing.T, n int) func(string) *httptest.ResponseRecorder {
			t.Helper()

			mux, s := newTestMux(t)
			token := issueToken(t, s)

			for i := range n {
				require.NoError(t, s.CreateObject(t.Context(), "obj"+strconv.Itoa(i), sealedFixture, nil, "", ""))
			}

			return func(query string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodGet, "/objects"+query, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)

				return rec
			}
		}},
		{"tokens", "/tokens", func(t *testing.T, n int) func(string) *httptest.ResponseRecorder {
			t.Helper()

			mux, s := newTestMux(t)

			for range n {
				_, _, err := s.CreateWriteToken(t.Context(), "t", time.Hour, "admin")
				require.NoError(t, err)
			}

			return sessionGetter(t, mux, s, "/tokens")
		}},
		{"consumer tokens", "/consumer-tokens", func(t *testing.T, n int) func(string) *httptest.ResponseRecorder {
			t.Helper()

			mux, s := newTestMux(t)

			for range n {
				_, _, err := s.CreateConsumerToken(t.Context(), "homelab", "t", time.Hour)
				require.NoError(t, err)
			}

			return sessionGetter(t, mux, s, "/consumer-tokens")
		}},
	}
}

func rows(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got []json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), rec.Body.String())

	return len(got)
}

func TestAListWithNoPagingParametersIsUnchanged(t *testing.T) {
	t.Parallel()

	for _, ep := range pagedEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			get := ep.seed(t, 3)
			rec := get("")

			require.Equal(t, 3, rows(t, rec))
			require.Equal(t, "3", rec.Header().Get("X-Total-Count"))
		})
	}
}

func TestAListReturnsAtMostLimitRowsAndTheTotal(t *testing.T) {
	t.Parallel()

	for _, ep := range pagedEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			get := ep.seed(t, 3)

			first := get("?limit=2")
			require.Equal(t, 2, rows(t, first))
			require.Equal(t, "3", first.Header().Get("X-Total-Count"))

			second := get("?limit=2&offset=2")
			require.Equal(t, 1, rows(t, second))
			require.Equal(t, "3", second.Header().Get("X-Total-Count"))

			past := get("?limit=2&offset=9")
			require.Equal(t, 0, rows(t, past))
			require.JSONEq(t, "[]", past.Body.String(), "an empty page is an empty array, not null")
		})
	}
}

func TestPagesDoNotOverlapOrSkip(t *testing.T) {
	t.Parallel()

	for _, ep := range pagedEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			get := ep.seed(t, 5)

			var all []map[string]any
			require.NoError(t, json.Unmarshal(get("").Body.Bytes(), &all))

			var paged []map[string]any

			for offset := 0; offset < 5; offset += 2 {
				var page []map[string]any
				require.NoError(t, json.Unmarshal(get("?limit=2&offset="+strconv.Itoa(offset)).Body.Bytes(), &page))

				paged = append(paged, page...)
			}

			require.Equal(t, all, paged)
		})
	}
}

func TestAnOffsetAloneReturnsTheRestUpToTheDefaultPageSize(t *testing.T) {
	t.Parallel()

	for _, ep := range pagedEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, 2, rows(t, ep.seed(t, 3)("?offset=1")))
		})
	}
}

func TestPagingParametersOutsideTheirRangeAreRefused(t *testing.T) {
	t.Parallel()

	for _, ep := range pagedEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			get := ep.seed(t, 1)

			for _, q := range []string{"?limit=0", "?limit=501", "?limit=x", "?offset=-1", "?offset=x"} {
				require.Equal(t, http.StatusBadRequest, get(q).Code, q)
			}

			require.Equal(t, http.StatusOK, get("?limit=500").Code)
		})
	}
}

func TestSpecDocumentsThePagingOnEveryPagedList(t *testing.T) {
	t.Parallel()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	for _, path := range []string{"/objects", "/tokens", "/consumer-tokens", "/credentials"} {
		op := doc.Paths.Value(path).Get
		got := map[string]*openapi3.Schema{}

		for _, p := range op.Parameters {
			got[p.Value.Name] = p.Value.Schema.Value
		}

		require.Contains(t, got, "limit", path)
		require.Contains(t, got, "offset", path)
		require.InDelta(t, 1, *got["limit"].Min, 0, path)
		require.InDelta(t, hushhush.MaxListPageSize, *got["limit"].Max, 0, path)
		require.InDelta(t, hushhush.DefaultListPageSize, got["limit"].Default, 0, path)
		require.InDelta(t, 0, *got["offset"].Min, 0, path)
		require.NotNil(t, got["offset"].Max, path)

		require.Contains(t, op.Responses.Value("200").Value.Headers, "X-Total-Count", path)
	}
}

func TestCredentialsCanBePaged(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	sess, _, _ := registerTwoCredentials(t, mux)

	req := httptest.NewRequest(http.MethodGet, "/credentials?limit=1", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sess, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, 1, rows(t, rec))
	require.Equal(t, "2", rec.Header().Get("X-Total-Count"))
}
