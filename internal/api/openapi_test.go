package api_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
			name: "list objects",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()
				seedObject(t, s, "contract_list")

				req := httptest.NewRequest(http.MethodGet, "/objects", nil)
				req.Header.Set("Authorization", "Bearer "+issueToken(t, s))

				return req
			},
		},
		{
			name: "list objects without a bearer token",
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodGet, "/objects", nil)
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
		{
			name: "begin registration",
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodPost, "/auth/register/begin", nil)
			},
		},
		{
			name: "begin registration on an existing account without a session",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()
				seedCredential(t, s)

				return httptest.NewRequest(http.MethodPost, "/auth/register/begin", nil)
			},
		},
		{
			name: "begin login with no admin account",
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodPost, "/auth/login/begin", nil)
			},
		},
		{
			name:                   "logout without a session or CSRF token",
			requestIsSchemaInvalid: true,
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
			},
		},
		{
			name: "list credentials without a session",
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				return httptest.NewRequest(http.MethodGet, "/credentials", nil)
			},
		},
		{
			name: "list credentials",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()
				seedCredential(t, s)

				req := httptest.NewRequest(http.MethodGet, "/credentials", nil)
				req.AddCookie(seedSessionCookie(t, s))

				return req
			},
		},
		{
			name: "delete unknown credential",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()
				// Two credentials seeded, so the "last remaining
				// credential" 409 guard doesn't shadow the 404 this case
				// means to exercise.
				seedCredential(t, s)
				seedCredential(t, s)

				sess := seedSessionCookie(t, s)
				sessRow, err := s.GetSession(t.Context(), sess.Value)
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodDelete, "/credentials/does-not-exist", nil)
				req.AddCookie(sess)
				req.Header.Set("X-CSRF-Token", sessRow.CSRFToken)

				return req
			},
		},
		{
			name: "create token",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()

				sess := seedSessionCookie(t, s)
				sessRow, err := s.GetSession(t.Context(), sess.Value)
				require.NoError(t, err)

				b := []byte(`{"description":"contract test token","ttl_seconds":3600}`)
				req := httptest.NewRequest(http.MethodPost, "/tokens", bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")
				req.AddCookie(sess)
				req.Header.Set("X-CSRF-Token", sessRow.CSRFToken)

				return req
			},
		},
		{
			name:                   "create token without a session",
			requestIsSchemaInvalid: true,
			request: func(t *testing.T, _ *store.Store) *http.Request {
				t.Helper()

				b := []byte(`{"description":"unauthenticated","ttl_seconds":3600}`)
				req := httptest.NewRequest(http.MethodPost, "/tokens", bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")

				return req
			},
		},
		{
			name: "list tokens",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()

				req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
				req.AddCookie(seedSessionCookie(t, s))

				return req
			},
		},
		{
			name: "revoke unknown token",
			request: func(t *testing.T, s *store.Store) *http.Request {
				t.Helper()

				sess := seedSessionCookie(t, s)
				sessRow, err := s.GetSession(t.Context(), sess.Value)
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodDelete, "/tokens/0000000000000000", nil)
				req.AddCookie(sess)
				req.Header.Set("X-CSRF-Token", sessRow.CSRFToken)

				return req
			},
		},
	}
}

// seedCredential creates a credential directly through the store, for a
// contract case that only needs an admin account to already exist rather
// than a full registration ceremony.
func seedCredential(t *testing.T, s *store.Store) {
	t.Helper()

	require.NoError(t, s.CreateCredential(t.Context(), store.Credential{
		ID: randomTestID(t), PublicKey: []byte("pubkey"), CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}))
}

// randomTestID returns a short random hex string - a test-local stand-in
// for the ids this package's own unexported randomToken generates
// (api_test is a separate package and can't reach that unexported
// helper), used wherever a test only needs a value unique enough to
// avoid a primary-key collision.
func randomTestID(t *testing.T) string {
	t.Helper()

	b := make([]byte, 8)
	_, err := rand.Read(b)
	require.NoError(t, err)

	return hex.EncodeToString(b)
}

// seedSessionCookie creates a session directly through the store, for a
// contract case that needs an authenticated session without a full
// login ceremony.
func seedSessionCookie(t *testing.T, s *store.Store) *http.Cookie {
	t.Helper()

	id := randomTestID(t)
	csrf := randomTestID(t)

	now := time.Now().UTC()
	require.NoError(t, s.CreateSession(t.Context(), store.Session{
		ID: id, CSRFToken: csrf,
		CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339),
	}))

	return &http.Cookie{Name: "session", Value: id} //nolint:gosec // request-side cookie, no response attributes to set
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

	mux := hushhush.NewMux(s, testPublicURL)
	req := createRequest(t, hushhush.CreateObjectRequest{ID: id, Value: []byte("sealed-ciphertext")}, issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
}
