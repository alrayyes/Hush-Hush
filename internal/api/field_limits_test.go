package api_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// The limits the server enforces on a request's fields, a policy chosen once
// and stated here (alrayyes/hush-hush#617). The spec carries the same numbers
// and TestSpecStatesTheFieldLimitsTheHandlersEnforce keeps the two together.

func chars(n int) string { return strings.Repeat("a", n) }

func names(n, length int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = chars(length)
	}

	return out
}

type limitEndpoint struct {
	name   string
	method string
	path   string
	base   map[string]any
}

var limitEndpoints = map[string]limitEndpoint{
	"create object": {
		"create object", http.MethodPost, "/objects",
		map[string]any{"slug": "s", "value": sealedFixture},
	},
	"update object": {
		"update object", http.MethodPut, "/objects/s",
		map[string]any{"value": sealedFixture},
	},
	"add consumer": {
		"add consumer", http.MethodPost, "/consumers",
		map[string]any{"name": "c"},
	},
	"update consumer": {
		"update consumer", http.MethodPatch, "/consumers/c",
		map[string]any{"name": "c"},
	},
	"create token": {
		"create token", http.MethodPost, "/tokens",
		map[string]any{"description": "d"},
	},
	"create consumer token": {
		"create consumer token", http.MethodPost, "/consumer-tokens",
		map[string]any{"consumer": "c", "description": "d"},
	},
	"rename credential": {
		"rename credential", http.MethodPatch, "/credentials/x",
		map[string]any{"nickname": "n"},
	},
}

type limitProbe struct {
	endpoint string
	field    string
	atLimit  any
	over     any
}

func limitProbes() []limitProbe {
	return []limitProbe{
		{"create object", "slug", chars(128), chars(129)},
		{"create object", "description", chars(1000), chars(1001)},
		{"create object", "used_by", names(100, 1), names(101, 1)},
		{"create object", "used_by", names(1, 128), names(1, 129)},
		{"update object", "used_by", names(100, 1), names(101, 1)},
		{"update object", "used_by", names(1, 128), names(1, 129)},
		{"add consumer", "name", chars(128), chars(129)},
		{"update consumer", "name", chars(128), chars(129)},
		{"update consumer", "public_key", chars(512), chars(513)},
		{"create token", "description", chars(200), chars(201)},
		{"create consumer token", "consumer", chars(128), chars(129)},
		{"create consumer token", "description", chars(200), chars(201)},
		{"rename credential", "nickname", chars(100), chars(101)},
	}
}

func sendLimitProbe(t *testing.T, ep limitEndpoint, field string, value any) *httptest.ResponseRecorder {
	t.Helper()

	body := map[string]any{}
	maps.Copy(body, ep.base)

	body[field] = value

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, tokenRequest(t, ep.method, ep.path, raw, sessionCookie, sess.CSRFToken, true))

	return rec
}

func TestAFieldOverItsLimitIsRefusedWithTheLimit(t *testing.T) {
	t.Parallel()

	for _, p := range limitProbes() {
		t.Run(p.endpoint+" "+p.field, func(t *testing.T) {
			t.Parallel()

			rec := sendLimitProbe(t, limitEndpoints[p.endpoint], p.field, p.over)

			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), p.field, "the message should name the field")
			require.Contains(t, rec.Body.String(), "at most", "the message should name the limit")
		})
	}
}

func TestAFieldAtItsLimitIsNotRefusedForItsSize(t *testing.T) {
	t.Parallel()

	for _, p := range limitProbes() {
		t.Run(p.endpoint+" "+p.field, func(t *testing.T) {
			t.Parallel()

			rec := sendLimitProbe(t, limitEndpoints[p.endpoint], p.field, p.atLimit)

			require.NotEqual(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			require.NotEqual(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
		})
	}
}

// Registration finishes before any user exists, so no session is needed to
// reach the limit check, which has to come before the credential is verified.
func TestRegistrationFieldsOverTheirLimitAreRefused(t *testing.T) {
	t.Parallel()

	for field, over := range map[string]string{
		"nickname":                  chars(101),
		"public_key":                chars(513),
		"wrapped_identity":          chars(4097),
		"recovery_wrapped_identity": chars(4097),
	} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()

			mux, _ := newTestMux(t)
			raw, err := json.Marshal(map[string]any{"credential": map[string]any{}, "nickname": "n", field: over})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/register/finish", strings.NewReader(string(raw)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), field)
		})
	}
}

func TestMCPInjectRefusesAFieldOverItsLimit(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	result := callTool(t, mux, issueToken(t, s), "inject", map[string]any{
		"slug":        "mcp_long_description",
		"value":       sealedFixture,
		"description": chars(1001),
	})

	require.True(t, result.IsError, "content: %+v", result.Content)
}

// The spec states each limit so a client can show it instead of guessing, and
// the handlers enforce it. Every request type's limits are declared once, as
// struct tags (hushhush.RequestLimits), and this compares each against the
// property of the same name in the spec.
func TestSpecStatesTheFieldLimitsTheHandlersEnforce(t *testing.T) {
	t.Parallel()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	checked := 0

	for schema, fields := range hushhush.RequestLimits() {
		props := doc.Components.Schemas[schema].Value.Properties

		for _, f := range fields {
			prop := props[f.JSONName]
			require.NotNil(t, prop, "%s.%s is not in the spec", schema, f.JSONName)

			checked++

			target := prop.Value

			if f.MaxItems != 0 {
				items := effectiveSchema(prop.Value, func(s *openapi3.Schema) bool { return s.MaxItems != nil })
				require.NotNil(t, items, "%s.%s needs maxItems", schema, f.JSONName)
				require.EqualValues(t, f.MaxItems, *items.MaxItems, "%s.%s maxItems", schema, f.JSONName)

				target = items.Items.Value
			}

			require.NotNil(t, target.MaxLength, "%s.%s needs maxLength", schema, f.JSONName)
			require.EqualValues(t, f.MaxLength, *target.MaxLength, "%s.%s maxLength", schema, f.JSONName)
		}
	}

	require.GreaterOrEqual(t, checked, 12)
}

// effectiveSchema finds the schema that carries a constraint, looking through
// allOf, which is how the spec adds a description beside a shared $ref.
func effectiveSchema(s *openapi3.Schema, has func(*openapi3.Schema) bool) *openapi3.Schema {
	if has(s) {
		return s
	}

	for _, part := range s.AllOf {
		if found := effectiveSchema(part.Value, has); found != nil {
			return found
		}
	}

	return nil
}
