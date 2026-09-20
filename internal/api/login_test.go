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

// TestLoginWithABackupEligibleCredentialsNonAdvancingCounterSucceeds covers
// the gap auth/spec.md's original "Cloned authenticator detected" scenario
// left: a synced/multi-device passkey (BE flag set) commonly reports a
// signature counter that never advances, or that resets across devices -
// Apple's and Google's own passkey documentation says as much, and
// go-webauthn's own Authenticator.CloneWarning doc comment leaves treating
// it as a hard rejection or not as "Relying Party-specific". Rejecting here
// would lock a real admin out of their own account on the second login
// with a synced passkey - alrayyes/hush-hush#260.
func TestLoginWithABackupEligibleCredentialsNonAdvancingCounterSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	beginReq := httptest.NewRequest(http.MethodPost, "/auth/register/begin", nil)
	beginRec := httptest.NewRecorder()
	mux.ServeHTTP(beginRec, beginReq)
	require.Equal(t, http.StatusOK, beginRec.Code, beginRec.Body.String())

	options, err := virtualwebauthn.ParseAttestationOptions(beginRec.Body.String())
	require.NoError(t, err)

	// hasResidentKey/BackupEligible - a real synced passkey (Chrome's own
	// password manager, iCloud Keychain, ...), unlike register_test.go's
	// plain registerCredential helper's device-bound authenticator.
	authenticator := virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{
		BackupEligible: true,
		BackupState:    true,
	})
	credential := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
	attestation := virtualwebauthn.CreateAttestationResponse(testRelyingParty(), authenticator, credential, *options)

	body := fmt.Sprintf(`{"credential":%s,"nickname":%q}`, attestation, "synced passkey")
	finishReq := httptest.NewRequest(http.MethodPost, "/auth/register/finish", bytes.NewReader([]byte(body)))
	finishReq.Header.Set("Content-Type", "application/json")
	finishReq.AddCookie(cookieFrom(t, beginRec, "webauthn_ceremony"))
	finishRec := httptest.NewRecorder()
	mux.ServeHTTP(finishRec, finishReq)
	require.Equal(t, http.StatusCreated, finishRec.Code, finishRec.Body.String())

	authenticator.AddCredential(credential)

	// Counter left at its zero value for both logins - exactly what a
	// counter-less synced passkey reports every time.
	require.Equal(t, http.StatusNoContent, loginWith(t, mux, authenticator, credential).Code)
	rec := loginWith(t, mux, authenticator, credential)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	creds, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, creds[0].LastUsedAt, "a successful login must still record its usage")
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
