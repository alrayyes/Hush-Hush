package api

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog logs one structured line per request, after the handler returns,
// because the status isn't known before then (rules/go.md, "HTTP servers").
// It wraps the whole mux from the outside, so a request the auth layer
// rejects still gets a line.
//
// The line carries method, path, status, duration_ms and the caller's
// address, the same flat keys the rest of this package logs with. It never
// carries the query string (a token can ride in one), a header, a cookie or
// a body: path is r.URL.Path alone.
//
// The level follows the response: 5xx is Error, 4xx is Warn, anything else
// Info. A /healthz or /readyz probe that succeeds is Debug, since it fires
// constantly and says nothing; one that fails keeps the level its status gives.
func AccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		status := rec.statusCode()
		logger.Log(r.Context(), accessLevel(r.URL.Path, status), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
			"remote", sourceIPFrom(r),
		)
	})
}

// accessLevel picks a request line's level from its status, quietening a
// healthy probe.
func accessLevel(path string, status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	case path == "/healthz" || path == "/readyz":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// statusRecorder remembers the status a handler wrote. http.ResponseWriter
// doesn't expose it, and a handler that only calls Write has sent a 200
// without ever saying so.
type statusRecorder struct {
	http.ResponseWriter

	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status, r.wroteHeader = status, true
	}

	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.status, r.wroteHeader = http.StatusOK, true
	}

	// A pass-through: the caller of Write must see the underlying writer's own
	// error, not one wrapped by a logging layer.
	return r.ResponseWriter.Write(b) //nolint:wrapcheck // see above
}

// Flush passes through, because the MCP endpoint streams and a wrapper that
// hid http.Flusher would stop it. A flush before any write sends the headers,
// so it counts as the 200 it is.
func (r *statusRecorder) Flush() {
	if !r.wroteHeader {
		r.status, r.wroteHeader = http.StatusOK, true
	}

	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the real writer for anything else
// it supports (hijacking, deadlines) without this type listing each one.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) statusCode() int {
	if !r.wroteHeader {
		return http.StatusOK
	}

	return r.status
}
