package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// testPublicURL is the PUBLIC_URL every handler test's mux is configured
// with, so WebAuthn ceremony tests have a valid relying party to begin a
// ceremony against without each one repeating the value.
const testPublicURL = "https://hush-hush.example.test"

// testVersion is the version every handler test's mux reports from
// /healthz.
const testVersion = "test-version"

// testIndexHTML is what every handler test's mux serves as the embedded
// SPA's index.html - static_test.go checks its exact content, and every
// other test just needs some fixed, recognizable body so a static-route
// case can't be confused with a real API response.
const testIndexHTML = "<html>test placeholder</html>"

// testWebBuild stands in for the real embedded SPA build in every
// handler test - an in-memory fs.FS (testing/fstest.MapFS), not the real
// cmd/hush-hush/web/build/ placeholder, so this package doesn't need a
// filesystem path back to it.
func testWebBuild() fstest.MapFS {
	return fstest.MapFS{
		"index.html":       {Data: []byte(testIndexHTML)},
		"_app/version.txt": {Data: []byte("test")},
	}
}

// newTestMux and its backing store are shared by every handler test in this
// package - each test gets its own in-memory database.
func newTestMux(t *testing.T) (*http.ServeMux, *store.Store) {
	t.Helper()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	return hushhush.NewMux(s, testPublicURL, testWebBuild(), testVersion), s
}

// issueToken mints a write token valid against s, for a test that needs a
// real one rather than exercising the rejection path itself.
func issueToken(t *testing.T, s *store.Store) string {
	t.Helper()

	_, token, err := s.CreateWriteToken(t.Context(), "test", time.Hour, "")
	require.NoError(t, err)

	return token
}

func createRequest(t *testing.T, body hushhush.CreateObjectRequest, token string) *http.Request {
	t.Helper()

	b, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/objects", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestCreateObjectRoundTripsThroughStorageUnchanged(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sealed := sealedFixture

	req := createRequest(t, hushhush.CreateObjectRequest{
		Slug:   "mattermost_deploy_webhook",
		Value:  sealed,
		UsedBy: []string{"homelab/vps-docker"},
	}, issueToken(t, s))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, "mattermost_deploy_webhook", meta.Slug)
	require.Equal(t, []string{"homelab/vps-docker"}, meta.UsedBy)

	// Verified via the store directly, not an HTTP GET - the get endpoint
	// is a separate ticket (alrayyes/hush-hush#32) and PR.
	obj, err := s.GetObject(context.Background(), "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, sealed, obj.Value)
}

func TestCreateObjectWithDescriptionReturnsItInMetadata(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{
		Slug:        "mattermost_deploy_webhook",
		Value:       sealedFixture,
		Description: "prod deploy webhook",
	}, issueToken(t, s))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, "prod deploy webhook", meta.Description)
}

func TestCreateObjectWithoutBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{Slug: "x", Value: sealedFixture}, "")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var body hushhush.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Error)
}

func TestCreateObjectWithWrongBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{Slug: "x", Value: sealedFixture}, "wrong-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCreateObjectDuplicateIDConflicts(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	first := createRequest(t, hushhush.CreateObjectRequest{Slug: "dup", Value: sealedFixture}, token)
	mux.ServeHTTP(httptest.NewRecorder(), first)

	second := createRequest(t, hushhush.CreateObjectRequest{Slug: "dup", Value: sealedFixtureNew}, token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, second)

	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateObjectValueIsBase64EncodedOverTheWire(t *testing.T) {
	// The wire format (CreateObjectRequest.Value json:"value" with
	// format:byte in the spec) is base64 text, not raw bytes - confirms the
	// JSON encoding actually round-trips through base64 rather than relying
	// on Go's own []byte<->JSON convention going unnoticed.
	t.Parallel()

	mux, s := newTestMux(t)

	raw := sealedFixture
	payload := struct {
		Slug  string `json:"slug"`
		Value string `json:"value"`
	}{Slug: "b64check", Value: base64.StdEncoding.EncodeToString(raw)}

	b, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/objects", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}

// TestCreateObjectOwnerOptsInToKeepAReadableCopy covers
// specs/secret-objects/spec.md's "Owner opts in to keep a readable copy"
// scenario: setting keep_readable_copy on a create request is accepted
// and echoed back in the response. The server never decrypts, so it has
// no way to verify Value's own recipients here - proving the owner's
// escrowed identity can actually decrypt what this flag describes is
// cmd/hush-hush/web/e2e/journey.spec.ts's job (tasks.md's 5.2), against
// the real client-side sealing this flag accompanies.
func TestCreateObjectOwnerOptsInToKeepAReadableCopy(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{
		Slug: "grafana_admin_password", Value: sealedFixture,
		KeepReadableCopy: true,
	}, issueToken(t, s))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.True(t, meta.KeepReadableCopy)
}

// TestCreateObjectOwnerRecipientIsNotTheDefault covers specs/secret-
// objects/spec.md's "Owner recipient is not the default" scenario:
// omitting keep_readable_copy is what a request that never mentions it
// (every other create test in this file) already exercises, so this
// pins the response's own default explicitly instead of relying on
// Go's zero value going unnoticed.
func TestCreateObjectOwnerRecipientIsNotTheDefault(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{
		Slug: "mattermost_deploy_webhook", Value: sealedFixture,
	}, issueToken(t, s))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.False(t, meta.KeepReadableCopy)
}

// TestCreateObjectRecordsOwnerFromTheCreatingSession covers specs/secret-
// objects/spec.md's "Owner recorded from the creating session" scenario:
// a create call, whether authenticated by a bearer token or a session,
// records the sole existing user as the object's owner - accountability
// metadata only, per design.md's "owner_id is accountability metadata,
// not an access-control mechanism" decision, verified via the store
// directly since ObjectMetadata never returns it.
func TestCreateObjectRecordsOwnerFromTheCreatingSession(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{
		Slug: "mattermost_deploy_webhook", Value: sealedFixture,
	}, issueToken(t, s))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	wantOwnerID, err := s.CurrentUserID(context.Background())
	require.NoError(t, err)

	obj, err := s.GetObject(context.Background(), "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, wantOwnerID, obj.OwnerID)
}
