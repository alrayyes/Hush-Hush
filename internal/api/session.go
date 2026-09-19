package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/go-webauthn/webauthn/webauthn"
)

// sessionContextKey is how requireSession hands the validated
// store.Session down to a handler that needs it (requireCSRF), without a
// second database lookup.
type sessionContextKey struct{}

// errCSRFWithoutSession means requireCSRF was composed onto a route
// without requireSession first putting a session in context - a routing
// mistake in this package, never a condition a caller can trigger.
var errCSRFWithoutSession = errors.New("requireCSRF used without requireSession")

// sessionFromCookie returns the session cookie's raw value, or false if
// the request carries none.
func sessionFromCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}

	return c.Value, true
}

// hasValidSession reports whether r carries a currently valid session -
// used by the registration ceremony, which needs to know only whether
// the caller is already authenticated, not the session itself.
func hasValidSession(r *http.Request, s objectStore) bool {
	id, ok := sessionFromCookie(r)
	if !ok {
		return false
	}

	_, err := s.GetSession(r.Context(), id)

	return err == nil
}

// requireSession rejects a request unless it carries a valid, unexpired
// session, and makes that session available to a wrapped requireCSRF via
// the request context. A session never authenticates a request to an
// endpoint that requires the write bearer token (auth/spec.md's "A
// session does not substitute for a bearer token" scenario) - this
// middleware is never used on those routes.
func requireSession(s objectStore, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := sessionFromCookie(r)
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid session")

			return
		}

		sess, err := s.GetSession(r.Context(), id)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid session")

			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, sess)))
	}
}

// requireCSRF rejects a state-changing, session-authenticated request
// unless its X-CSRF-Token header matches the session's own stored token -
// the gap SameSite=Lax alone leaves open, since it still allows a
// cross-site top-level GET navigation to carry the session cookie
// (design.md's "Session storage" decision). Must be composed inside
// requireSession, which is what puts the session in context.
func requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := r.Context().Value(sessionContextKey{}).(store.Session)
		if !ok {
			writeInternalError(w, r, errCSRFWithoutSession)

			return
		}

		if sess.CSRFToken == "" || r.Header.Get("X-CSRF-Token") != sess.CSRFToken {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid CSRF token")

			return
		}

		next(w, r)
	}
}

// issueSession creates a new session and sets its cookie. Called only
// when the caller doesn't already have a valid one - registering an
// additional passkey while already logged in leaves the active session
// untouched (auth/spec.md's "Registering an additional passkey" scenario).
func issueSession(w http.ResponseWriter, r *http.Request, s objectStore) error {
	id, err := randomToken(32)
	if err != nil {
		return err
	}

	csrf, err := randomToken(32)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := s.CreateSession(r.Context(), store.Session{
		ID: id, CSRFToken: csrf,
		CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(sessionTTL).Format(time.RFC3339),
	}); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: id, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionTTL.Seconds()),
	})

	// Deliberately NOT httpOnly: the double-submit pattern requireCSRF
	// checks against needs frontend JS to read this cookie and echo its
	// value back in the X-CSRF-Token header. It carries no authority on
	// its own - only the session cookie (httpOnly) does - so a script on
	// another origin reading it via document.cookie gains nothing without
	// also holding the session cookie, which it can't read or send
	// cross-site.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // HttpOnly omitted deliberately, see comment above
		Name: csrfCookieName, Value: csrf, Path: "/",
		Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionTTL.Seconds()),
	})

	return nil
}

// handleLogout invalidates the presented session immediately.
func handleLogout(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, _ := r.Context().Value(sessionContextKey{}).(store.Session)

		if err := s.DeleteSession(r.Context(), sess.ID); err != nil {
			writeInternalError(w, r, err)

			return
		}

		http.SetCookie(w, &http.Cookie{
			Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		})
		http.SetCookie(w, &http.Cookie{ //nolint:gosec // HttpOnly omitted deliberately, see issueSession
			Name: csrfCookieName, Value: "", Path: "/", MaxAge: -1,
			Secure: true, SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// saveCeremonyAndSetCookie stores session's WebAuthn session data,
// opaque, keyed by a fresh random ceremony id, and sets that id as an
// httpOnly cookie - the "opaque session cookie" the go-webauthn package
// documentation recommends, except the session data itself lives
// server-side rather than in the cookie.
func saveCeremonyAndSetCookie(w http.ResponseWriter, r *http.Request, s objectStore, kind string, session *webauthn.SessionData) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal ceremony session: %w", err)
	}

	id, err := randomToken(16)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := s.SaveCeremony(r.Context(), id, kind, data,
		now.Format(time.RFC3339), now.Add(ceremonyTTL).Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("save ceremony: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name: ceremonyCookieName, Value: id, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(ceremonyTTL.Seconds()),
	})

	return nil
}

// consumeCeremony reads the ceremony cookie, looks up and deletes its
// stored session data (one-time use), and unmarshals it back into a
// webauthn.SessionData.
func consumeCeremony(r *http.Request, s objectStore, kind string) (webauthn.SessionData, error) {
	c, err := r.Cookie(ceremonyCookieName)
	if err != nil {
		return webauthn.SessionData{}, store.ErrCeremonyNotFound
	}

	data, err := s.GetAndDeleteCeremony(r.Context(), c.Value, kind)
	if err != nil {
		return webauthn.SessionData{}, fmt.Errorf("get ceremony: %w", err)
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return webauthn.SessionData{}, fmt.Errorf("unmarshal ceremony session: %w", err)
	}

	return session, nil
}

func clearCeremonyCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: ceremonyCookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}
