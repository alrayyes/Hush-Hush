package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// registerTwoCredentials bootstraps the admin account with one
// credential, then adds a second under the same session - the shape most
// of these tests need to exercise a real "not the last one" delete.
func registerTwoCredentials(t *testing.T, mux *http.ServeMux) (sessionCookie, csrfToken, firstID string) {
	t.Helper()

	_, _, firstRec := registerCredential(t, mux, nil, "first")
	sess := cookieFrom(t, firstRec, "session")
	csrf := cookieFrom(t, firstRec, "csrf_token")

	_, _, secondRec := registerCredential(t, mux, sess, "second")
	require.Equal(t, http.StatusCreated, secondRec.Code, secondRec.Body.String())

	creds := listCredentials(t, mux, sess)
	require.Len(t, creds, 2)

	return sess.Value, csrf.Value, creds[0].ID
}

func listCredentials(t *testing.T, mux *http.ServeMux, sessionCookie *http.Cookie) []hushhush.Credential {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/credentials", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var creds []hushhush.Credential
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &creds))

	return creds
}

func TestListCredentialsExcludesPublicKeyMaterial(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	_, _, firstRec := registerCredential(t, mux, nil, "my passkey")
	sess := cookieFrom(t, firstRec, "session")

	req := httptest.NewRequest(http.MethodGet, "/credentials", nil)
	req.AddCookie(sess)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "publicKey")
	require.NotContains(t, rec.Body.String(), "signCount")

	var creds []hushhush.Credential
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &creds))
	require.Len(t, creds, 1)
	require.Equal(t, "my passkey", creds[0].Nickname)
}

func TestRenameCredentialUpdatesNickname(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	_, _, firstRec := registerCredential(t, mux, nil, "old name")
	sess := cookieFrom(t, firstRec, "session")
	csrf := cookieFrom(t, firstRec, "csrf_token")
	creds := listCredentials(t, mux, sess)

	body, err := json.Marshal(hushhush.CredentialRenameRequest{Nickname: "new name"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/credentials/"+creds[0].ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sess)
	req.Header.Set("X-CSRF-Token", csrf.Value)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	updated := listCredentials(t, mux, sess)
	require.Equal(t, "new name", updated[0].Nickname)
}

func TestDeleteNonLastCredentialRemovesIt(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	sessValue, csrfValue, firstID := registerTwoCredentials(t, mux)

	req := httptest.NewRequest(http.MethodDelete, "/credentials/"+firstID, nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessValue}) //nolint:gosec // request-side cookie, no response attributes to set
	req.Header.Set("X-CSRF-Token", csrfValue)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	remaining := listCredentials(t, mux, &http.Cookie{Name: "session", Value: sessValue}) //nolint:gosec // request-side cookie
	require.Len(t, remaining, 1)
}

func TestDeletedCredentialCanNoLongerLogIn(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	// registerCredential's own second call (under the first one's session)
	// gives full control over which authenticator/credential pair to
	// delete and then attempt to log in with, unlike
	// registerTwoCredentials's fixed pair.
	_, _, firstRec := registerCredential(t, mux, nil, "keep")
	sess := cookieFrom(t, firstRec, "session")
	csrf := cookieFrom(t, firstRec, "csrf_token")
	authenticator, credential, _ := registerCredential(t, mux, sess, "delete me")

	deleteID := base64.RawURLEncoding.EncodeToString(credential.ID)
	req := httptest.NewRequest(http.MethodDelete, "/credentials/"+deleteID, nil)
	req.AddCookie(sess)
	req.Header.Set("X-CSRF-Token", csrf.Value)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	loginRec := loginWith(t, mux, authenticator, credential)
	require.Equal(t, http.StatusUnauthorized, loginRec.Code)
}

func TestDeleteLastRemainingCredentialIsRefused(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	_, _, firstRec := registerCredential(t, mux, nil, "only one")
	sess := cookieFrom(t, firstRec, "session")
	csrf := cookieFrom(t, firstRec, "csrf_token")
	creds := listCredentials(t, mux, sess)
	require.Len(t, creds, 1)

	req := httptest.NewRequest(http.MethodDelete, "/credentials/"+creds[0].ID, nil)
	req.AddCookie(sess)
	req.Header.Set("X-CSRF-Token", csrf.Value)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	stillThere, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, stillThere, 1)
}

func TestListCredentialsWithoutASessionIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/credentials", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDeleteCredentialWithoutCSRFTokenIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	sessValue, _, firstID := registerTwoCredentials(t, mux)

	req := httptest.NewRequest(http.MethodDelete, "/credentials/"+firstID, nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessValue}) //nolint:gosec // request-side cookie
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
