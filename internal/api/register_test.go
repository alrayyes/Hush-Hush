package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/descope/virtualwebauthn"
	"github.com/stretchr/testify/require"
)

// testRelyingParty matches testPublicURL, so a registration or login
// ceremony run against it verifies against the same relying party the
// test mux itself is configured with.
func testRelyingParty() virtualwebauthn.RelyingParty {
	return virtualwebauthn.RelyingParty{ID: "hush-hush.example.test", Name: "Hush Hush", Origin: testPublicURL}
}

// cookieFrom finds a cookie by name among rec's Set-Cookie headers, or
// fails the test - every ceremony test needs its Begin call's cookie to
// carry into the matching Finish call, the same way a browser would.
func cookieFrom(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}

	t.Fatalf("no %q cookie in response", name)

	return nil
}

// registerCredential runs a full registration ceremony (begin, then
// finish with a fresh virtual authenticator's attestation) and returns
// the authenticator, the credential it registered, and the finish
// response - a session cookie set on that response if the caller had
// none, in this repo's case.
func registerCredential(t *testing.T, mux *http.ServeMux, sessionCookie *http.Cookie, nickname string) (
	virtualwebauthn.Authenticator, virtualwebauthn.Credential, *httptest.ResponseRecorder,
) {
	t.Helper()

	beginReq := httptest.NewRequest(http.MethodPost, "/auth/register/begin", nil)
	if sessionCookie != nil {
		beginReq.AddCookie(sessionCookie)
	}

	beginRec := httptest.NewRecorder()
	mux.ServeHTTP(beginRec, beginReq)
	require.Equal(t, http.StatusOK, beginRec.Code, beginRec.Body.String())

	options, err := virtualwebauthn.ParseAttestationOptions(beginRec.Body.String())
	require.NoError(t, err)

	authenticator := virtualwebauthn.NewAuthenticator()
	credential := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
	attestation := virtualwebauthn.CreateAttestationResponse(testRelyingParty(), authenticator, credential, *options)

	body := fmt.Sprintf(`{"credential":%s,"nickname":%q}`, attestation, nickname)
	finishReq := httptest.NewRequest(http.MethodPost, "/auth/register/finish", bytes.NewReader([]byte(body)))
	finishReq.Header.Set("Content-Type", "application/json")
	finishReq.AddCookie(cookieFrom(t, beginRec, "webauthn_ceremony"))

	if sessionCookie != nil {
		finishReq.AddCookie(sessionCookie)
	}

	finishRec := httptest.NewRecorder()
	mux.ServeHTTP(finishRec, finishReq)

	authenticator.AddCredential(credential)

	return authenticator, credential, finishRec
}

func TestFirstRegistrationCreatesAccountAndIssuesSession(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	_, _, rec := registerCredential(t, mux, nil, "YubiKey 5C")

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var cred hushhush.Credential
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cred))
	require.Equal(t, "YubiKey 5C", cred.Nickname)
	require.NotEmpty(t, cookieFrom(t, rec, "session").Value)

	stored, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, stored, 1)
}

func TestRegisteringWithoutASessionOnAnExistingAccountIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	registerCredential(t, mux, nil, "first passkey")

	beginReq := httptest.NewRequest(http.MethodPost, "/auth/register/begin", nil)
	beginRec := httptest.NewRecorder()
	mux.ServeHTTP(beginRec, beginReq)

	require.Equal(t, http.StatusUnauthorized, beginRec.Code)
}

func TestRegisteringAnAdditionalPasskeyKeepsTheExistingSession(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	_, _, firstRec := registerCredential(t, mux, nil, "first passkey")
	sessionCookie := cookieFrom(t, firstRec, "session")

	_, _, secondRec := registerCredential(t, mux, sessionCookie, "second passkey")
	require.Equal(t, http.StatusCreated, secondRec.Code, secondRec.Body.String())

	// No new session cookie is set - the existing one is left untouched
	// (auth/spec.md's "Registering an additional passkey" scenario).
	for _, c := range secondRec.Result().Cookies() {
		require.NotEqual(t, "session", c.Name, "a second session cookie was issued")
	}

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, creds, 2)

	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)
	require.Equal(t, sessionCookie.Value, sess.ID)
}

func TestRegistrationWithoutPublicURLConfiguredIsBadRequest(t *testing.T) {
	t.Parallel()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	mux := hushhush.NewMux(s, "", testWebBuild())

	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
