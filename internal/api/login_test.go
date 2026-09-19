package api_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/descope/virtualwebauthn"
	"github.com/stretchr/testify/require"
)

// loginWith runs a full login ceremony (begin, then finish with the
// authenticator's assertion for credential, whose Counter field the
// caller has already set to whatever value this attempt should assert)
// and returns the finish response.
func loginWith(t *testing.T, mux *http.ServeMux, authenticator virtualwebauthn.Authenticator, credential virtualwebauthn.Credential) *httptest.ResponseRecorder {
	t.Helper()

	beginReq := httptest.NewRequest(http.MethodPost, "/auth/login/begin", nil)
	beginRec := httptest.NewRecorder()
	mux.ServeHTTP(beginRec, beginReq)
	require.Equal(t, http.StatusOK, beginRec.Code, beginRec.Body.String())

	options, err := virtualwebauthn.ParseAssertionOptions(beginRec.Body.String())
	require.NoError(t, err)

	assertion := virtualwebauthn.CreateAssertionResponse(testRelyingParty(), authenticator, credential, *options)

	finishReq := httptest.NewRequest(http.MethodPost, "/auth/login/finish",
		bytes.NewReader([]byte(fmt.Sprintf(`{"credential":%s}`, assertion))))
	finishReq.Header.Set("Content-Type", "application/json")
	finishReq.AddCookie(cookieFrom(t, beginRec, "webauthn_ceremony"))

	finishRec := httptest.NewRecorder()
	mux.ServeHTTP(finishRec, finishReq)

	return finishRec
}

func TestSuccessfulLoginIssuesARegeneratedSessionAndUpdatesUsage(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	authenticator, credential, regRec := registerCredential(t, mux, nil, "test key")
	firstSession := cookieFrom(t, regRec, "session")

	credential.Counter = 1
	loginRec := loginWith(t, mux, authenticator, credential)

	require.Equal(t, http.StatusNoContent, loginRec.Code, loginRec.Body.String())

	loginSession := cookieFrom(t, loginRec, "session")
	require.NotEqual(t, firstSession.Value, loginSession.Value, "login did not regenerate the session id")

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, creds, 1)
	require.EqualValues(t, 1, creds[0].SignCount)
	require.NotEmpty(t, creds[0].LastUsedAt)
}

func TestLoginWithNonAdvancingCounterIsRejectedAsClonedAuthenticator(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	authenticator, credential, _ := registerCredential(t, mux, nil, "test key")

	credential.Counter = 5
	require.Equal(t, http.StatusNoContent, loginWith(t, mux, authenticator, credential).Code)

	// Same counter value again - a real authenticator's counter only
	// advances, so this is exactly the clone signal auth/spec.md's
	// "Cloned authenticator detected" scenario describes.
	rec := loginWith(t, mux, authenticator, credential)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.EqualValues(t, 5, creds[0].SignCount, "a rejected login must not update the stored counter")
}

func TestLoginBeginWithNoAdminAccountIsBadRequest(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/begin", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLoginWithAnUnregisteredCredentialIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	registerCredential(t, mux, nil, "the real passkey")

	stranger := virtualwebauthn.NewAuthenticator()
	strangerCred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
	stranger.AddCredential(strangerCred)

	rec := loginWith(t, mux, stranger, strangerCred)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
