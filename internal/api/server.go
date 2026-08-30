package api

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/alrayyes/hush-hush/internal/store"
)

// NewMux wires the handlers registered against api/openapi.yaml. Every
// create, update, and delete call is checked against s's issued write
// tokens (alrayyes/hush-hush#72) - reads need no authorization, per the
// settled v1 design (openspec/changes/secrets-object-store/design.md).
func NewMux(s *store.Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("POST /objects", requireWriteToken(s, handleCreateObject(s)))
	mux.HandleFunc("GET /objects/{id}", handleGetObject(s))
	mux.HandleFunc("GET /objects/{id}/used-by", handleGetObjectUsedBy(s))
	mux.HandleFunc("PUT /objects/{id}", requireWriteToken(s, handleUpdateObject(s)))
	mux.HandleFunc("DELETE /objects/{id}", requireWriteToken(s, handleDeleteObject(s)))
	mux.HandleFunc("GET /audit-log", handleQueryAuditLog(s))

	return mux
}

// requireWriteToken rejects a request unless it carries an Authorization
// header of "Bearer <token>" naming a currently issued, unexpired write
// token (store.ValidateWriteToken) - an unknown, malformed, expired, or
// revoked token are all the same 401 to the caller.
func requireWriteToken(s *store.Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || got == "" {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid bearer token")

			return
		}

		valid, err := s.ValidateWriteToken(r.Context(), got)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		if !valid {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid bearer token")

			return
		}

		next(w, r)
	}
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
