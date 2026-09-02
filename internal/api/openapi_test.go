package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"
)

// loadSpec parses and validates api/openapi.yaml once per case - every
// contractCase below is checked against this same document, so a stale
// copy of the spec can't quietly pass. redocly lint (lint:api) already
// proves the document is valid OpenAPI on its own; this proves the real
// handlers still match it, the gap CONTRIBUTING.md's "The contract"
// section used to flag.
func loadSpec(t *testing.T) routers.Router {
	t.Helper()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)
	require.NoError(t, doc.Validate(t.Context()))

	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)

	return router
}

// contractCase is one real HTTP round trip through the actual mux, checked
// against the spec's schema for whichever operation it hits - a wrong
// status code, a missing required field, or an undocumented response all
// fail it the same way a real caller would notice them.
//
// requestIsSchemaInvalid marks a case that's deliberately malformed at the
// request level (a query parameter that doesn't match its documented
// pattern, say) - the point of that case is the handler's own response,
// not a clean request, so it skips ValidateRequest rather than failing on
// an error the case exists to trigger.
type contractCase struct {
	name                   string
	requestIsSchemaInvalid bool
	request                func(t *testing.T, s *store.Store) *http.Request
}

func TestHandlersMatchOpenAPISpec(t *testing.T) {
	t.Parallel()

	router := loadSpec(t)

	for _, tc := range contractCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkContractCase(t, router, tc)
		})
	}
}

// contractCases is the table TestHandlersMatchOpenAPISpec runs - split out
// so the test function itself stays short; the cases are the substance.
//
//nolint:funlen // a literal table, one entry per documented operation - length is case count, not control-flow complexity
func contractCases() []contractCase {
	return []contractCase{
		{
			name: "health",
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodGet, "/healthz", nil)
			},
		},
		{
			name: "create object",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()

				return createRequest(t, hushhush.CreateObjectRequest{
					ID:     "contract_create",
					Value:  []byte("sealed-ciphertext"),
					UsedBy: []string{"homelab/vps-docker"},
				}, issueToken(t, s))
			},
		},
		{
			name: "create object without a bearer token",
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return createRequest(t, hushhush.CreateObjectRequest{
					ID: "contract_create_unauth", Value: []byte("v"),
				}, "")
			},
		},
		{
			name: "get object",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()
				seedObject(t, s, "contract_get")

				return httptest.NewRequest(http.MethodGet, "/objects/contract_get", nil)
			},
		},
		{
			name: "get unknown object",
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodGet, "/objects/contract_missing", nil)
			},
		},
		{
			name: "get object used-by",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()
				seedObject(t, s, "contract_used_by")

				return httptest.NewRequest(http.MethodGet, "/objects/contract_used_by/used-by", nil)
			},
		},
		{
			name: "update object",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()
				seedObject(t, s, "contract_update")

				b := []byte(`{"value":"bmV3LXNlYWxlZC12YWx1ZQ=="}`)
				req := httptest.NewRequest(http.MethodPut, "/objects/contract_update", bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+issueToken(t, s))

				return req
			},
		},
		{
			name: "delete object",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()
				seedObject(t, s, "contract_delete")

				req := httptest.NewRequest(http.MethodDelete, "/objects/contract_delete", nil)
				req.Header.Set("Authorization", "Bearer "+issueToken(t, s))

				return req
			},
		},
		{
			name: "query audit log",
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodGet, "/audit-log?object_id=contract_get", nil)
			},
		},
		{
			name:                   "query audit log with a malformed filter",
			requestIsSchemaInvalid: true,
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodGet, "/audit-log?from=not-a-timestamp", nil)
			},
		},
	}
}

// checkContractCase runs one contractCase's request through the real mux
// and validates both directions against router's spec.
func checkContractCase(t *testing.T, router routers.Router, tc contractCase) {
	t.Helper()

	mux, s := newTestMux(t)
	req := tc.request(t, s)

	// api/openapi.yaml's servers entry matches {scheme}://{host} on its
	// default variables (https, localhost:8080) - the gorillamux router
	// matches a request against that same template, and
	// httptest.NewRequest leaves req.Host as "example.com" by default,
	// which never matches it.
	req.URL.Scheme = "http"
	req.URL.Host = "localhost:8080"
	req.Host = "localhost:8080"

	route, pathParams, err := router.FindRoute(req)
	require.NoError(t, err)

	reqInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		// bearerAuth is checked against components.securitySchemes, not
		// against a real caller - NoopAuthenticationFunc treats any
		// presented scheme as satisfied, since what's under test here is
		// the request/response shape, not token validity (already covered
		// by create_test.go and friends).
		Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}

	if tc.requestIsSchemaInvalid {
		require.Error(t, openapi3filter.ValidateRequest(t.Context(), reqInput))
	} else {
		require.NoError(t, openapi3filter.ValidateRequest(t.Context(), reqInput))
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	respInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 rec.Code,
		Header:                 rec.Header(),
	}
	respInput.SetBodyBytes(rec.Body.Bytes())

	require.NoError(t, openapi3filter.ValidateResponse(t.Context(), respInput))
}

// seedObject creates an object directly through the mux so a case that
// exercises get/update/delete/used-by has one to act on, without coupling
// this suite to create's own request-building.
func seedObject(t *testing.T, s *store.Store, id string) {
	t.Helper()

	mux := hushhush.NewMux(s)
	req := createRequest(t, hushhush.CreateObjectRequest{ID: id, Value: []byte("sealed-ciphertext")}, issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
}
