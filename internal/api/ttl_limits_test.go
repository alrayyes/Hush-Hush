package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

const (
	// The policy under test (alrayyes/hush-hush#537): a token lasts 90 days
	// unless asked otherwise, and never more than 365.
	wantDefaultTTL = 90 * 24 * time.Hour
	wantMaxTTL     = 365 * 24 * time.Hour
)

// ttlEndpoint is one of the four routes that take ttl_seconds: create and
// rotate, for write tokens and for consumer tokens. body builds the JSON
// request for a given ttl field fragment ("" leaves ttl_seconds out).
type ttlEndpoint struct {
	name string
	path func(t *testing.T, s *store.Store) string
	body func(ttl string) string
}

func withTTL(fields, ttl string) string {
	if ttl == "" {
		return "{" + fields + "}"
	}
	if fields == "" {
		return "{" + ttl + "}"
	}

	return "{" + fields + "," + ttl + "}"
}

func ttlEndpoints() []ttlEndpoint {
	return []ttlEndpoint{
		{
			name: "create write token",
			path: func(*testing.T, *store.Store) string { return "/tokens" },
			body: func(ttl string) string { return withTTL(`"description":"ttl"`, ttl) },
		},
		{
			name: "rotate write token",
			path: func(t *testing.T, s *store.Store) string {
				t.Helper()
				wt, _, err := s.CreateWriteToken(t.Context(), "to rotate", time.Hour, "admin")
				require.NoError(t, err)

				return "/tokens/" + wt.ID + "/rotate"
			},
			body: func(ttl string) string { return withTTL("", ttl) },
		},
		{
			name: "create consumer token",
			path: func(*testing.T, *store.Store) string { return "/consumer-tokens" },
			body: func(ttl string) string { return withTTL(`"consumer":"homelab","description":"ttl"`, ttl) },
		},
		{
			name: "rotate consumer token",
			path: func(t *testing.T, s *store.Store) string {
				t.Helper()
				ct, _, err := s.CreateConsumerToken(t.Context(), "homelab", "to rotate", time.Hour)
				require.NoError(t, err)

				return "/consumer-tokens/" + ct.ID + "/rotate"
			},
			body: func(ttl string) string { return withTTL("", ttl) },
		},
	}
}

// postTTL sends one request to ep as an admin session and returns the
// recorder.
func postTTL(t *testing.T, ep ttlEndpoint, ttl string) *httptest.ResponseRecorder {
	t.Helper()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := tokenRequest(t, http.MethodPost, ep.path(t, s), []byte(ep.body(ttl)), sessionCookie, sess.CSRFToken, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func expiresIn(t *testing.T, rec *httptest.ResponseRecorder) time.Duration {
	t.Helper()

	var got struct {
		ExpiresAt string `json:"expires_at"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), rec.Body.String())

	expires, err := time.Parse(time.RFC3339, got.ExpiresAt)
	require.NoError(t, err)

	return time.Until(expires)
}

func TestTokenTTLAboveTheMaximumIsRejectedWithTheLimit(t *testing.T) {
	t.Parallel()

	over := fmt.Sprintf(`"ttl_seconds":%d`, int64(wantMaxTTL.Seconds())+1)

	for _, ep := range ttlEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			rec := postTTL(t, ep, over)
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "31536000", "the message should name the limit")
		})
	}
}

func TestTokenTTLAtTheMaximumIsAccepted(t *testing.T) {
	t.Parallel()

	atMax := fmt.Sprintf(`"ttl_seconds":%d`, int64(wantMaxTTL.Seconds()))

	for _, ep := range ttlEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			rec := postTTL(t, ep, atMax)
			require.Contains(t, []int{http.StatusOK, http.StatusCreated}, rec.Code, rec.Body.String())
			require.InDelta(t, wantMaxTTL.Seconds(), expiresIn(t, rec).Seconds(), 60)
		})
	}
}

func TestTokenTTLDefaultsToNinetyDaysWhenOmitted(t *testing.T) {
	t.Parallel()

	for _, ep := range ttlEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()

			rec := postTTL(t, ep, "")
			require.Contains(t, []int{http.StatusOK, http.StatusCreated}, rec.Code, rec.Body.String())
			require.InDelta(t, wantDefaultTTL.Seconds(), expiresIn(t, rec).Seconds(), 60)
		})
	}
}

// The maximum applies when a token is minted or rotated, never to one
// already issued, so nothing stops working on upgrade.
func TestTokenIssuedBeyondTheMaximumKeepsWorking(t *testing.T) {
	t.Parallel()

	_, s := newTestMux(t)

	_, value, err := s.CreateWriteToken(t.Context(), "issued before the limit", 2*wantMaxTTL, "admin")
	require.NoError(t, err)

	valid, err := s.ValidateWriteToken(t.Context(), value)
	require.NoError(t, err)
	require.True(t, valid)
}

// The spec states the limit so a client can show it instead of guessing,
// and the handlers enforce it. This pins the two together: change one
// without the other and this fails.
func TestSpecStatesTheSameTTLLimitTheHandlersEnforce(t *testing.T) {
	t.Parallel()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	ttl := doc.Components.Schemas["TokenTtlSeconds"]
	require.NotNil(t, ttl, "components.schemas.TokenTtlSeconds is where the limit lives")

	require.NotNil(t, ttl.Value.Max)
	require.InDelta(t, hushhush.MaxTokenTTL.Seconds(), *ttl.Value.Max, 0)
	require.InDelta(t, hushhush.DefaultTokenTTL.Seconds(), ttl.Value.Default, 0)

	for _, name := range []string{"CreateTokenRequest", "RotateTokenRequest", "CreateConsumerTokenRequest", "RotateConsumerTokenRequest"} {
		prop := doc.Components.Schemas[name].Value.Properties["ttl_seconds"]
		require.Equal(t, "#/components/schemas/TokenTtlSeconds", prop.Ref, "%s should take its limit from the shared schema", name)
	}
}
