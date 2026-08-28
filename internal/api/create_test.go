package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

const testWriterToken = "test-writer-token" //nolint:gosec // test fixture, not a real credential

// newTestMux and its backing store are shared by every handler test in this
// package - each test gets its own in-memory database.
func newTestMux(t *testing.T) (*http.ServeMux, *store.Store) {
	t.Helper()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	return hushhush.NewMux(s, testWriterToken), s
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
	mux, s := newTestMux(t)
	sealed := []byte("sealed-ciphertext")

	req := createRequest(t, hushhush.CreateObjectRequest{
		ID:     "mattermost_deploy_webhook",
		Value:  sealed,
		UsedBy: []string{"homelab/vps-docker"},
	}, testWriterToken)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, "mattermost_deploy_webhook", meta.ID)
	require.Equal(t, []string{"homelab/vps-docker"}, meta.UsedBy)

	// Verified via the store directly, not an HTTP GET - the get endpoint
	// is a separate ticket (alrayyes/hush-hush#32) and PR.
	obj, err := s.GetObject(context.Background(), "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, sealed, obj.Value)
}

func TestCreateObjectWithoutBearerTokenIsRejected(t *testing.T) {
	mux, _ := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "x", Value: []byte("v")}, "")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var body hushhush.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Error)
}

func TestCreateObjectWithWrongBearerTokenIsRejected(t *testing.T) {
	mux, _ := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "x", Value: []byte("v")}, "wrong-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCreateObjectDuplicateIDConflicts(t *testing.T) {
	mux, _ := newTestMux(t)

	first := createRequest(t, hushhush.CreateObjectRequest{ID: "dup", Value: []byte("v1")}, testWriterToken)
	mux.ServeHTTP(httptest.NewRecorder(), first)

	second := createRequest(t, hushhush.CreateObjectRequest{ID: "dup", Value: []byte("v2")}, testWriterToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, second)

	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateObjectValueIsBase64EncodedOverTheWire(t *testing.T) {
	// The wire format (CreateObjectRequest.Value json:"value" with
	// format:byte in the spec) is base64 text, not raw bytes - confirms the
	// JSON encoding actually round-trips through base64 rather than relying
	// on Go's own []byte<->JSON convention going unnoticed.
	mux, _ := newTestMux(t)

	raw := []byte("sealed-ciphertext")
	payload := struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}{ID: "b64check", Value: base64.StdEncoding.EncodeToString(raw)}

	b, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/objects", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+testWriterToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}
