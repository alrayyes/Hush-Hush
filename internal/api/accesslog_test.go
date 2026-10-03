package api_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// logLine is one JSON line from slog's JSON handler.
type logLine map[string]any

// accessLogged runs one request through AccessLog wrapped around next, with a
// logger that keeps everything down to debug, and returns the lines it wrote.
func accessLogged(t *testing.T, next http.Handler, req *http.Request) []logLine {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rec := httptest.NewRecorder()
	hushhush.AccessLog(logger, next).ServeHTTP(rec, req)

	var lines []logLine
	for raw := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var line logLine
		require.NoError(t, json.Unmarshal([]byte(raw), &line), raw)
		lines = append(lines, line)
	}

	return lines
}

func respondWith(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
}

func TestAccessLogWritesOneLineAfterTheHandlerReturns(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/objects", nil)
	req.RemoteAddr = "203.0.113.7:51234"
	lines := accessLogged(t, respondWith(http.StatusCreated), req)

	require.Len(t, lines, 1)
	require.Equal(t, "POST", lines[0]["method"])
	require.Equal(t, "/objects", lines[0]["path"])
	require.EqualValues(t, http.StatusCreated, lines[0]["status"])
	require.Equal(t, "203.0.113.7", lines[0]["remote"], "the caller's address without its port")
	require.Contains(t, lines[0], "duration_ms")
}

func TestAccessLogLevelFollowsTheStatus(t *testing.T) {
	t.Parallel()

	for status, level := range map[int]string{
		http.StatusOK:                  "INFO",
		http.StatusCreated:             "INFO",
		http.StatusFound:               "INFO",
		http.StatusBadRequest:          "WARN",
		http.StatusUnauthorized:        "WARN",
		http.StatusNotFound:            "WARN",
		http.StatusInternalServerError: "ERROR",
		http.StatusBadGateway:          "ERROR",
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			lines := accessLogged(t, respondWith(status), httptest.NewRequest(http.MethodGet, "/objects", nil))
			require.Len(t, lines, 1)
			require.Equal(t, level, lines[0]["level"])
		})
	}
}

func TestAccessLogTreatsAHandlerThatNeverSetsAStatusAsA200(t *testing.T) {
	t.Parallel()

	body := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hello")) })
	lines := accessLogged(t, body, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Len(t, lines, 1)
	require.EqualValues(t, http.StatusOK, lines[0]["status"])
	require.Equal(t, "INFO", lines[0]["level"])
}

func TestAccessLogKeepsHealthProbesAtDebug(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/healthz", "/readyz"} {
		lines := accessLogged(t, respondWith(http.StatusOK), httptest.NewRequest(http.MethodGet, path, nil))
		require.Len(t, lines, 1, path)
		require.Equal(t, "DEBUG", lines[0]["level"], path)
	}

	// A failing probe is still worth seeing: only the routine success is quiet.
	lines := accessLogged(t, respondWith(http.StatusServiceUnavailable), httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Len(t, lines, 1)
	require.Equal(t, "ERROR", lines[0]["level"])
}

func TestAccessLogNeverWritesASecret(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/objects?token=query-secret&slug=x", strings.NewReader("body-secret"))
	req.Header.Set("Authorization", "Bearer header-secret")
	req.Header.Set("Cookie", "session=cookie-secret")
	req.Header.Set("X-CSRF-Token", "csrf-secret")

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	hushhush.AccessLog(logger, respondWith(http.StatusOK)).ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	require.NotEmpty(t, out)
	for _, secret := range []string{"query-secret", "header-secret", "cookie-secret", "csrf-secret", "body-secret", "token="} {
		require.NotContains(t, out, secret)
	}
	require.Contains(t, out, `"path":"/objects"`)
}

// The MCP endpoint streams, so the wrapper has to pass Flush through.
func TestAccessLogStillLetsAHandlerFlush(t *testing.T) {
	t.Parallel()

	var isFlusher, flushed bool
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		isFlusher = ok
		if ok {
			f.Flush()
			flushed = true
		}
	})
	accessLogged(t, next, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	require.True(t, isFlusher, "the wrapped writer must still be a Flusher")
	require.True(t, flushed)
}

// Wrapped from the outside, so a request the auth layer rejects still gets a line.
func TestAccessLogSeesARequestTheMuxRejects(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	lines := accessLogged(t, mux, httptest.NewRequest(http.MethodPost, "/objects", strings.NewReader(`{}`)))

	var access logLine
	for _, line := range lines {
		if _, ok := line["duration_ms"]; ok {
			access = line
		}
	}
	require.NotNil(t, access, "no access line among: %v", lines)
	require.EqualValues(t, http.StatusUnauthorized, access["status"])
	require.Equal(t, "WARN", access["level"])
}
