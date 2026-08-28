package api

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/alrayyes/hush-hush/internal/store"
)

// NewMux wires the handlers registered against api/openapi.yaml. writerToken
// is the single v1 write-path bearer token, checked on every create, update,
// and delete call - reads need no authorization, per the settled v1 design
// (openspec/changes/secrets-object-store/design.md).
func NewMux(s *store.Store, writerToken string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("POST /objects", requireBearerToken(writerToken, handleCreateObject(s)))
	mux.HandleFunc("GET /objects/{id}", handleGetObject(s))
	mux.HandleFunc("GET /objects/{id}/used-by", handleGetObjectUsedBy(s))
	mux.HandleFunc("PUT /objects/{id}", requireBearerToken(writerToken, handleUpdateObject(s)))

	return mux
}

// requireBearerToken rejects a request unless it carries an Authorization
// header of exactly "Bearer <token>", comparing in constant time so a
// request can't learn anything about the token from response timing.
func requireBearerToken(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid bearer token")

			return
		}
		next(w, r)
	}
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
