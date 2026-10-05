package api_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// What a query parameter or a header may hold (alrayyes/hush-hush#617). The
// server enforces these on every route, and the spec states the same numbers.

func getWith(t *testing.T, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	mux, s := newTestMux(t)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

var stringQueryLimits = map[string]int{
	"q": 128, "used_by": 128, "object_id": 128, "caller": 128, "actor": 128, "from": 64, "to": 64,
}

var stringQueryPaths = map[string]string{
	"q": "/consumers", "used_by": "/objects", "object_id": "/audit-log",
	"caller": "/audit-log", "actor": "/audit-log", "from": "/audit-log", "to": "/audit-log",
}

func TestAQueryValueOverItsLimitIsRefusedWithTheLimit(t *testing.T) {
	t.Parallel()

	for name, limit := range stringQueryLimits {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := getWith(t, stringQueryPaths[name]+"?"+name+"="+chars(limit+1), nil)

			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), name)
			require.Contains(t, rec.Body.String(), strconv.Itoa(limit))
		})
	}
}

func TestAQueryValueAtItsLimitIsNotRefusedForItsSize(t *testing.T) {
	t.Parallel()

	for name, limit := range stringQueryLimits {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := getWith(t, stringQueryPaths[name]+"?"+name+"="+chars(limit), nil)

			require.NotEqual(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
		})
	}
}

func TestMoreThanTenTagFiltersAreRefused(t *testing.T) {
	t.Parallel()

	q := url.Values{"tag": strings.Split(strings.Repeat("a,", 10)+"b", ",")}

	require.Equal(t, http.StatusBadRequest, getWith(t, "/objects?"+q.Encode(), nil).Code)
	require.Equal(t, http.StatusOK, getWith(t, "/objects?tag=a", nil).Code)
}

func TestAHeaderOverItsLimitIsRefusedWithTheLimit(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"X-Caller", "X-CSRF-Token"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			over := getWith(t, "/objects", map[string]string{name: chars(129)})
			require.Equal(t, http.StatusUnprocessableEntity, over.Code, over.Body.String())
			require.Contains(t, over.Body.String(), name)
			require.Contains(t, over.Body.String(), "128")

			at := getWith(t, "/objects", map[string]string{name: chars(128)})
			require.NotEqual(t, http.StatusUnprocessableEntity, at.Code, at.Body.String())
		})
	}
}

func TestAnIntegerQueryOutsideItsRangeIsRefused(t *testing.T) {
	t.Parallel()

	for _, target := range []string{
		"/consumers?page=2147483648",
		"/audit-log?after=-1",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, http.StatusBadRequest, getWith(t, target, nil).Code)
		})
	}

	require.Equal(t, http.StatusOK, getWith(t, "/consumers?page=2147483647", nil).Code)
	require.Equal(t, http.StatusOK, getWith(t, "/audit-log?after=0", nil).Code)
}

// Every string parameter the spec declares in a query or a header states the
// limit the server enforces on it.
func TestSpecStatesTheSameParameterLimitsTheServerEnforces(t *testing.T) {
	t.Parallel()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	headerLimits := map[string]int{"X-Caller": 128, "X-CSRF-Token": 128}
	checked := 0

	// An operation that takes a limited parameter can answer 422 for it.
	has422 := func(op *openapi3.Operation) bool { return op.Responses.Value("422") != nil }

	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			for _, ref := range op.Parameters {
				p := ref.Value
				if p.Schema.Value.Type == nil || !p.Schema.Value.Type.Is("string") || p.Schema.Value.Format == "binary" || len(p.Schema.Value.Enum) > 0 {
					continue
				}

				var want int
				switch p.In {
				case "query":
					want = stringQueryLimits[p.Name]
				case "header":
					want = headerLimits[p.Name]
				default:
					continue
				}

				checked++

				require.True(t, has422(op), "%s %s takes %s, so it can answer 422", method, path, p.Name)
				require.NotZero(t, want, "%s %s %s has no limit in the server's table", method, path, p.Name)
				require.NotNil(t, p.Schema.Value.MaxLength, "%s %s %s needs maxLength", method, path, p.Name)
				require.EqualValues(t, want, *p.Schema.Value.MaxLength, "%s %s %s", method, path, p.Name)
			}
		}
	}

	require.Positive(t, checked)
}
