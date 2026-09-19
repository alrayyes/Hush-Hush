package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestLogoutWithoutASessionIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogoutWithoutCSRFHeaderIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogoutInvalidatesTheSession(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	_, err = s.GetSession(t.Context(), sessionCookie.Value)
	require.ErrorIs(t, err, store.ErrSessionNotFound)
}

func TestExpiredSessionIsTreatedAsUnauthenticated(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	now := time.Now().UTC()
	require.NoError(t, s.CreateSession(t.Context(), store.Session{
		ID: "expired-session", CSRFToken: "csrf",
		CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339),
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "expired-session"}) //nolint:gosec // a request-side Cookie header carries no Secure/HttpOnly/SameSite attributes to set
	req.Header.Set("X-CSRF-Token", "csrf")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// A session authenticating /objects on its own terms, alongside the
// bearer token, is covered by write_access_test.go - this file's own
// scope is the /auth/* session/CSRF middleware itself.

// seedSession creates a session directly through the store, for a test
// that needs a valid one without going through a full registration or
// login ceremony.
func seedSession(t *testing.T, s *store.Store) *http.Cookie {
	t.Helper()

	now := time.Now().UTC()
	sess := store.Session{
		ID: "test-session", CSRFToken: "test-csrf",
		CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339),
	}
	require.NoError(t, s.CreateSession(t.Context(), sess))

	return &http.Cookie{Name: "session", Value: sess.ID} //nolint:gosec // a request-side Cookie header carries no Secure/HttpOnly/SameSite attributes to set
}
