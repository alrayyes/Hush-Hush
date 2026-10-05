package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// sealedValueOfSize returns a real age ciphertext exactly size bytes long.
// age's overhead depends on the header and on one 16-byte tag per 64 KiB
// chunk, so it starts from a guess and corrects by the shortfall.
func sealedValueOfSize(t *testing.T, size int) []byte {
	t.Helper()

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	plain := size
	for range 5 {
		var out bytes.Buffer
		w, err := age.Encrypt(&out, identity.Recipient())
		require.NoError(t, err)
		_, err = w.Write(make([]byte, plain))
		require.NoError(t, err)
		require.NoError(t, w.Close())

		if out.Len() == size {
			return out.Bytes()
		}

		plain += size - out.Len()
	}

	t.Fatalf("could not seal a value of exactly %d bytes", size)

	return nil
}

func requireBodyTooLarge(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())

	var got hushhush.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), rec.Body.String())
	require.NotEmpty(t, got.Error)
}

// The four unauthenticated POST routes are the ones anyone who can reach the
// server can call, so they are the ones that must not read without a bound
// (alrayyes/hush-hush#617).
func TestOversizedBodyIsRefusedOnEveryUnauthenticatedPost(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("a", hushhush.MaxBodyBytes+1)

	for _, path := range []string{
		"/auth/register/begin", "/auth/register/finish", "/auth/login/begin", "/auth/login/finish",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			mux, _ := newTestMux(t)
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"x":"`+big+`"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			requireBodyTooLarge(t, rec)
		})
	}
}

// countingReader is a body of unknown length (so no Content-Length can refuse
// it up front) that records how much the server pulled from it.
type countingReader struct{ read int }

func (c *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}

	c.read += len(p)

	return len(p), nil
}

func TestServerStopsReadingAnUnboundedBodyAtTheLimit(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	body := &countingReader{}

	req := httptest.NewRequest(http.MethodPost, "/objects", io.MultiReader(strings.NewReader(`{"slug":"a","value":"`), body))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	requireBodyTooLarge(t, rec)
	// The reader hands out whatever buffer it is given, so allow one buffer
	// of slack over the limit, not an unbounded stream.
	require.LessOrEqual(t, body.read, hushhush.MaxBodyBytes+64*1024)
}

func TestValueOverTheMaximumIsRefusedOnCreateAndUpdate(t *testing.T) {
	t.Parallel()

	over := sealedValueOfSize(t, hushhush.MaxValueBytes+1)

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		mux, s := newTestMux(t)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "big", Value: over}, issueToken(t, s)))

		requireBodyTooLarge(t, rec)
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		mux, s := newTestMux(t)
		token := issueToken(t, s)

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "big", Value: sealedFixture}, token))
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, updateRequest(t, "big", over, token))

		requireBodyTooLarge(t, rec)
	})
}

func TestValueAtTheMaximumIsAccepted(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{
		Slug: "max", Value: sealedValueOfSize(t, hushhush.MaxValueBytes),
	}, issueToken(t, s)))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestMCPInjectRefusesAValueOverTheMaximum(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	result := callTool(t, mux, issueToken(t, s), "inject", map[string]any{
		"slug":  "mcp_big",
		"value": sealedValueOfSize(t, hushhush.MaxValueBytes+1),
	})

	require.True(t, result.IsError, "content: %+v", result.Content)
}
