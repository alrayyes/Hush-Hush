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

// registerCredentialWithEscrow runs a full registration ceremony the same
// way registerCredential does, but with the escrow fields a real client
// sends alongside the attestation - group 3's own request-body additions
// (wrapped_identity always, public_key/recovery_wrapped_identity only on
// a first-ever registration).
func registerCredentialWithEscrow(
	t *testing.T, mux *http.ServeMux, sessionCookie *http.Cookie, nickname string,
	req hushhush.RegistrationFinishRequest,
) *httptest.ResponseRecorder {
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

	req.Nickname = nickname
	req.Credential = json.RawMessage(attestation)

	body, err := json.Marshal(req)
	require.NoError(t, err)

	finishReq := httptest.NewRequest(http.MethodPost, "/auth/register/finish", bytes.NewReader(body))
	finishReq.Header.Set("Content-Type", "application/json")
	finishReq.AddCookie(cookieFrom(t, beginRec, "webauthn_ceremony"))

	if sessionCookie != nil {
		finishReq.AddCookie(sessionCookie)
	}

	finishRec := httptest.NewRecorder()
	mux.ServeHTTP(finishRec, finishReq)

	return finishRec
}

// TestFirstRegistrationStoresEscrowedIdentityPublicKeyAndRecoveryWrap
// covers tasks.md group 3.2/3.3: the escrowed identity's public key and
// recovery-phrase-wrapped copy sent on a first-ever registration land on
// the users row, and this PRF-capable credential's own wrapped copy lands
// on its own row - specs/users/spec.md's "Escrowed identity generated
// once" and "Per-credential wrapping of the escrowed identity"
// requirements.
func TestFirstRegistrationStoresEscrowedIdentityPublicKeyAndRecoveryWrap(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := registerCredentialWithEscrow(t, mux, nil, "YubiKey 5C", hushhush.RegistrationFinishRequest{
		PublicKey:               "age1exampleexampleexample",
		RecoveryWrappedIdentity: "recovery-wrapped-identity",
		WrappedIdentity:         "prf-wrapped-identity",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var cred hushhush.Credential
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cred))
	require.Equal(t, "prf-wrapped-identity", cred.WrappedIdentity)

	stored, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Equal(t, "prf-wrapped-identity", stored[0].WrappedIdentity)
	require.NotEmpty(t, stored[0].UserID)

	var publicKey, recoveryWrapped string
	require.NoError(t, s.DB().QueryRow(
		`SELECT public_key, recovery_wrapped_identity FROM users WHERE id = ?`, stored[0].UserID,
	).Scan(&publicKey, &recoveryWrapped))
	require.Equal(t, "age1exampleexampleexample", publicKey)
	require.Equal(t, "recovery-wrapped-identity", recoveryWrapped)
}

// TestRegistrationWithoutPRFSupportStoresNoWrappedIdentity covers
// tasks.md group 3.1/3.3: a registration that sends no wrapped_identity
// (the client's PRF-detection came back negative) still succeeds, and no
// wrapped copy is stored against that credential - specs/users/spec.md's
// "Authenticator without PRF support registers without wrapping"
// scenario.
func TestRegistrationWithoutPRFSupportStoresNoWrappedIdentity(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := registerCredentialWithEscrow(t, mux, nil, "non-PRF key", hushhush.RegistrationFinishRequest{
		PublicKey:               "age1exampleexampleexample",
		RecoveryWrappedIdentity: "recovery-wrapped-identity",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var cred hushhush.Credential
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cred))
	require.Empty(t, cred.WrappedIdentity)

	stored, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Empty(t, stored[0].WrappedIdentity)
}

// TestRegisteringASecondPRFCapablePasskeyWrapsASecondCopy covers
// specs/users/spec.md's "Registering a second PRF-capable passkey wraps a
// second copy" scenario: a second credential's own wrapped_identity is
// stored independently of the first's, with no public_key/
// recovery_wrapped_identity resent (the identity was already escrowed).
func TestRegisteringASecondPRFCapablePasskeyWrapsASecondCopy(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	firstRec := registerCredentialWithEscrow(t, mux, nil, "first", hushhush.RegistrationFinishRequest{
		PublicKey:               "age1exampleexampleexample",
		RecoveryWrappedIdentity: "recovery-wrapped-identity",
		WrappedIdentity:         "wrapped-copy-1",
	})
	require.Equal(t, http.StatusCreated, firstRec.Code, firstRec.Body.String())
	sessionCookie := cookieFrom(t, firstRec, "session")

	secondRec := registerCredentialWithEscrow(t, mux, sessionCookie, "second", hushhush.RegistrationFinishRequest{
		WrappedIdentity: "wrapped-copy-2",
	})
	require.Equal(t, http.StatusCreated, secondRec.Code, secondRec.Body.String())

	stored, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.Len(t, stored, 2)

	byNickname := map[string]string{}
	for _, c := range stored {
		byNickname[c.Nickname] = c.WrappedIdentity
	}
	require.Equal(t, "wrapped-copy-1", byNickname["first"])
	require.Equal(t, "wrapped-copy-2", byNickname["second"])
}

// TestALaterRegistrationCannotOverwriteTheEscrowedIdentity covers
// specs/users/spec.md's "Escrowed identity generated once" scenario
// against a buggy or malicious later registration request that resends
// public_key/recovery_wrapped_identity: the server never trusts a client's
// say-so on when "first" is, so the original identity survives.
func TestALaterRegistrationCannotOverwriteTheEscrowedIdentity(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	firstRec := registerCredentialWithEscrow(t, mux, nil, "first", hushhush.RegistrationFinishRequest{
		PublicKey:               "age1original",
		RecoveryWrappedIdentity: "recovery-wrapped-original",
		WrappedIdentity:         "wrapped-copy-1",
	})
	require.Equal(t, http.StatusCreated, firstRec.Code, firstRec.Body.String())
	sessionCookie := cookieFrom(t, firstRec, "session")

	secondRec := registerCredentialWithEscrow(t, mux, sessionCookie, "second", hushhush.RegistrationFinishRequest{
		PublicKey:               "age1attacker",
		RecoveryWrappedIdentity: "recovery-wrapped-attacker",
		WrappedIdentity:         "wrapped-copy-2",
	})
	require.Equal(t, http.StatusCreated, secondRec.Code, secondRec.Body.String())

	stored, err := s.ListCredentials(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, stored)

	var publicKey, recoveryWrapped string
	require.NoError(t, s.DB().QueryRow(
		`SELECT public_key, recovery_wrapped_identity FROM users WHERE id = ?`, stored[0].UserID,
	).Scan(&publicKey, &recoveryWrapped))
	require.Equal(t, "age1original", publicKey)
	require.Equal(t, "recovery-wrapped-original", recoveryWrapped)
}

func TestRegistrationWithoutPublicURLConfiguredIsBadRequest(t *testing.T) {
	t.Parallel()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	mux := hushhush.NewMux(s, "", testWebBuild(), testVersion)

	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
