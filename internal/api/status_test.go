package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestAuthStatusReportsNoAdminAccountOnAFreshInstall(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var status hushhush.AuthStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	require.False(t, status.Bootstrapped)
}

func TestAuthStatusReportsAnAdminAccountOnceOneExists(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	registerCredential(t, mux, nil, "first")

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var status hushhush.AuthStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	require.True(t, status.Bootstrapped)
}

func authStatusWith(t *testing.T, mux *http.ServeMux, cookie *http.Cookie) hushhush.AuthStatus {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var status hushhush.AuthStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))

	return status
}

func TestAuthStatusIsNotAuthenticatedWithoutASessionCookie(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	require.False(t, authStatusWith(t, mux, nil).Authenticated)
}

func TestAuthStatusIsAuthenticatedWithAValidSession(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	require.True(t, authStatusWith(t, mux, seedSession(t, s)).Authenticated)
}

func TestAuthStatusIsNotAuthenticatedWithAnExpiredOrUnknownSession(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	now := time.Now().UTC()
	require.NoError(t, s.CreateSession(t.Context(), store.Session{
		ID: "expired-session", CSRFToken: "csrf",
		CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339),
	}))

	for _, value := range []string{"expired-session", "tampered-value"} {
		cookie := &http.Cookie{Name: "session", Value: value} //nolint:gosec // a request-side Cookie header carries no Secure/HttpOnly/SameSite attributes to set
		require.False(t, authStatusWith(t, mux, cookie).Authenticated, value)
	}
}

func TestAuthStatusSetsNoCookies(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Empty(t, rec.Result().Cookies())
}

// TestAuthIdentityRequiresACredential covers GET /auth/identity's own gating -
// unlike /auth/status, it's never answered anonymously (status.go's
// handleAuthIdentity doc comment: an age public key isn't secret, but
// every other endpoint that exposes stored data stays behind a real
// session or bearer token, and this keeps that same posture).
func TestAuthIdentityRequiresACredential(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/identity", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthIdentityReportsEmptyPublicKeyBeforeEscrow(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	_, _, regRec := registerCredential(t, mux, nil, "first")
	sessionCookie := cookieFrom(t, regRec, "session")

	req := httptest.NewRequest(http.MethodGet, "/auth/identity", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var identity hushhush.OwnerIdentity
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &identity))
	require.Empty(t, identity.PublicKey)
}

// TestAuthIdentityReportsEscrowedPublicKeyOnceSet is what the create/edit
// dialog's owner-recipient checkbox actually reads once an account has
// completed a real first registration (registerCredentialWithEscrow's own
// full flow is register_test.go's job; SetUserEscrow directly is enough
// here to pin this endpoint's own response shape).
func TestAuthIdentityReportsEscrowedPublicKeyOnceSet(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	_, _, regRec := registerCredential(t, mux, nil, "first")
	sessionCookie := cookieFrom(t, regRec, "session")

	userID, err := s.CurrentUserID(context.Background())
	require.NoError(t, err)
	require.NoError(t, s.SetUserEscrow(context.Background(), userID, "age1ownerkey", "recovery-wrapped"))

	req := httptest.NewRequest(http.MethodGet, "/auth/identity", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var identity hushhush.OwnerIdentity
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &identity))
	require.Equal(t, "age1ownerkey", identity.PublicKey)
}

// TestAuthIdentityAcceptsAWriteBearerToken is what lets an SDK client, which
// holds an API key and never a session, honour keep_readable_copy: it has to
// fetch the owner's public key to add as a sealing recipient. The key is the
// same whichever credential asks, since there is one user row to read it from.
func TestAuthIdentityAcceptsAWriteBearerToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	_, _, _ = registerCredential(t, mux, nil, "first")

	userID, err := s.CurrentUserID(context.Background())
	require.NoError(t, err)
	require.NoError(t, s.SetUserEscrow(context.Background(), userID, "age1ownerkey", "recovery-wrapped"))

	req := httptest.NewRequest(http.MethodGet, "/auth/identity", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var identity hushhush.OwnerIdentity
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &identity))
	require.Equal(t, "age1ownerkey", identity.PublicKey)
}

func TestAuthIdentityRejectsAnInvalidBearerToken(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/identity", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
