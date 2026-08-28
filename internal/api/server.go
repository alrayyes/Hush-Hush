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

	return mux
}

// requireBearerToken rejects a request unless it carries an Authorization
// header of exactly "Bearer <token>", comparing in constant time so a
// request can't learn anything about the token from response timing.
func requireBearerToken(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")

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

// writeError writes the Error body every documented error response carries
// (components.schemas.Error in api/openapi.yaml). Handlers go through this
// rather than building Error{} literals inline, so the shape can't drift
// between them.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, Error{Error: msg})
}

// writeInternalError logs err at Error level and writes a generic 500 - the
// client gets no detail on a failure that's this service's own, but an
// operator needs the real cause, which the generic body never carries. A
// 4xx is the caller's own mistake and goes through writeError instead,
// unlogged: it's expected control flow, not a failure to investigate.
func writeInternalError(w http.ResponseWriter, err error) {
	slog.Error("internal error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
