package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
)

// objectStore is what this package needs from a store - defined here,
// the consuming package, per go.md's "define interfaces in the consuming
// package, not alongside the implementation". *store.Store satisfies it;
// a handler-level test can satisfy it with a fake instead of a real
// database (go-test.md's "reach for a fake before a mock").
type objectStore interface {
	CreateObject(ctx context.Context, id string, value []byte, usedBy []string, description string) error
	GetObject(ctx context.Context, id string) (store.Object, error)
	ListObjects(ctx context.Context, filter store.ObjectFilter) ([]store.Object, error)
	UpdateObject(ctx context.Context, id string, value []byte) error
	DeleteObject(ctx context.Context, id string) error
	RecordAuditLog(ctx context.Context, objectID string, action store.AuditAction, caller, ip, actorType, actorID string) error
	QueryAuditLog(ctx context.Context, filter store.AuditLogFilter) ([]store.AuditLogEntry, error)
	ValidateWriteToken(ctx context.Context, token string) (bool, error)
	CreateWriteToken(ctx context.Context, description string, ttl time.Duration, owner string) (store.WriteToken, string, error)
	ListWriteTokens(ctx context.Context) ([]store.WriteToken, error)
	RevokeWriteToken(ctx context.Context, id string) error

	CreateCredential(ctx context.Context, c store.Credential) error
	ListCredentials(ctx context.Context) ([]store.Credential, error)
	UpdateCredentialUsage(ctx context.Context, id string, signCount uint32, lastUsedAt string) error
	RenameCredential(ctx context.Context, id, nickname string) error
	DeleteCredential(ctx context.Context, id string) error

	CreateSession(ctx context.Context, sess store.Session) error
	GetSession(ctx context.Context, id string) (store.Session, error)
	DeleteSession(ctx context.Context, id string) error

	SaveCeremony(ctx context.Context, id, kind string, data []byte, createdAt, expiresAt string) error
	GetAndDeleteCeremony(ctx context.Context, id, kind string) ([]byte, error)
}

// NewMux wires the handlers registered against api/openapi.yaml. Every
// create, update, and delete call is checked against s's issued write
// tokens (alrayyes/hush-hush#72) - reads need no authorization, per the
// settled v1 design (openspec/changes/secrets-object-store/design.md).
//
// publicURL is the server's own PUBLIC_URL config, used only to derive
// the WebAuthn relying party's RPID/RPOrigins (design.md's "Relying party
// ID/origin" decision) - empty is a valid, expected value for a
// deployment that hasn't enabled the web UI yet, in which case every
// /auth/* ceremony endpoint answers with a configuration error rather
// than the server refusing to start.
func NewMux(s objectStore, publicURL string) *http.ServeMux {
	wa, err := newWebAuthn(publicURL)
	if err != nil {
		wa = nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("POST /objects", requireWriteAccess(s, true, handleCreateObject(s)))
	mux.HandleFunc("GET /objects", requireWriteAccess(s, false, handleListObjects(s)))
	mux.HandleFunc("GET /objects/{id}", handleGetObject(s))
	mux.HandleFunc("GET /objects/{id}/used-by", handleGetObjectUsedBy(s))
	mux.HandleFunc("PUT /objects/{id}", requireWriteAccess(s, true, handleUpdateObject(s)))
	mux.HandleFunc("DELETE /objects/{id}", requireWriteAccess(s, true, handleDeleteObject(s)))
	mux.HandleFunc("GET /audit-log", handleQueryAuditLog(s))

	mux.HandleFunc("POST /auth/register/begin", handleBeginRegistration(s, wa))
	mux.HandleFunc("POST /auth/register/finish", handleFinishRegistration(s, wa))
	mux.HandleFunc("POST /auth/login/begin", handleBeginLogin(s, wa))
	mux.HandleFunc("POST /auth/login/finish", handleFinishLogin(s, wa))
	mux.HandleFunc("POST /auth/logout", requireSession(s, requireCSRF(handleLogout(s))))

	mux.HandleFunc("GET /credentials", requireSession(s, handleListCredentials(s)))
	mux.HandleFunc("PATCH /credentials/{id}", requireSession(s, requireCSRF(handleRenameCredential(s))))
	mux.HandleFunc("DELETE /credentials/{id}", requireSession(s, requireCSRF(handleDeleteCredential(s))))

	mux.HandleFunc("POST /tokens", requireSession(s, requireCSRF(handleCreateToken(s))))
	mux.HandleFunc("GET /tokens", requireSession(s, handleListTokens(s)))
	mux.HandleFunc("DELETE /tokens/{id}", requireSession(s, requireCSRF(handleRevokeToken(s))))

	return mux
}

// requireWriteAccess rejects a request unless it carries a valid write
// bearer token OR a valid session - two independent, equally valid
// credentials for /objects (openspec/changes/web-ui/design.md's
// "/objects accepts a valid session" decision; the web UI holds a
// session, never a bearer token, and this is what lets its secrets
// overview work at all). requireCSRFForSession additionally requires a
// session's own CSRF token on a state-changing request authenticated by
// session - never checked for a bearer-token-authenticated one, which has
// no session or CSRF token to present.
func requireWriteAccess(s objectStore, requireCSRFForSession bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		validToken, err := bearerTokenValid(r, s)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		if validToken {
			next(w, r)

			return
		}

		sess, ok := validSession(r, s)
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid bearer token or session")

			return
		}

		if requireCSRFForSession && (sess.CSRFToken == "" || r.Header.Get("X-CSRF-Token") != sess.CSRFToken) {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid CSRF token")

			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, sess)))
	}
}

// bearerTokenValid reports whether r carries a currently valid write
// bearer token - false (with no error) for a missing or unknown one, an
// error only for a genuine lookup failure.
func bearerTokenValid(r *http.Request, s objectStore) (bool, error) {
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || got == "" {
		return false, nil
	}

	valid, err := s.ValidateWriteToken(r.Context(), got)
	if err != nil {
		return false, fmt.Errorf("validate write token: %w", err)
	}

	return valid, nil
}

// validSession returns r's session if it carries a valid, unexpired one.
func validSession(r *http.Request, s objectStore) (store.Session, bool) {
	id, ok := sessionFromCookie(r)
	if !ok {
		return store.Session{}, false
	}

	sess, err := s.GetSession(r.Context(), id)
	if err != nil {
		return store.Session{}, false
	}

	return sess, true
}

// actorFrom returns the verified actor (type, id) that authenticated r -
// a session put in context by requireWriteAccess or requireSession, or
// both empty when neither did (an unauthenticated read, or - until
// alrayyes/hush-hush#214 adds token attribution too - a
// bearer-token-authenticated write).
func actorFrom(r *http.Request) (actorType, actorID string) {
	if _, ok := r.Context().Value(sessionContextKey{}).(store.Session); ok {
		return "session", string(adminUserID)
	}

	return "", ""
}

// callerFrom returns the caller's self-presented identity, or "" if none
// was given. X-Caller is a courtesy label, not a verified identity -
// api/openapi.yaml's `caller` parameter.
func callerFrom(r *http.Request) string {
	return r.Header.Get("X-Caller")
}

// sourceIPFrom returns the immediate TCP peer's address, with any port
// stripped - r.RemoteAddr, unlike X-Caller, is never self-reported.
//
// This service isn't deployed behind a reverse proxy or load balancer, so
// there's no X-Forwarded-For (or similar) support here yet: trusting that
// header without knowing which upstream hop to trust it from would let any
// caller spoof its own recorded IP, trading one spoofable value for
// another. Add it once a real deployment needs it, scoped to the actual
// proxy in front of it.
func sourceIPFrom(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes the Error body every documented 4xx response carries
// (components.schemas.Error in api/openapi.yaml) and logs it at Warn - a
// bad payload, a missing token, an unknown id, or a conflicting write is
// the caller's own mistake, one the service handles and moves past, not a
// failure to investigate. Only method, path, and status go in the log
// line: nothing here ever carries the Authorization header or request
// body a caller sent.
func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	slog.WarnContext(r.Context(), "request rejected",
		"method", r.Method, "path", r.URL.Path, "status", status, "reason", msg)
	writeJSON(w, status, Error{Error: msg})
}

// writeInternalError logs err at Error level and writes a generic 500 - the
// client gets no detail on a failure that's this service's own, but an
// operator needs the real cause, which the generic body never carries.
func writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "internal error",
		"method", r.Method, "path", r.URL.Path, "error", err)
	writeJSON(w, http.StatusInternalServerError, Error{Error: "internal error"})
}
