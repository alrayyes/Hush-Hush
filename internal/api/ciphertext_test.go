package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// sealedValue returns a real age ciphertext, sealed to one fresh X25519
// recipient: the smallest value the server should accept. Anything a test
// stores through the API has to be one of these now (alrayyes/hush-hush#538),
// since the server refuses a value that isn't a well-formed age file.
func sealedValue(t *testing.T) []byte {
	t.Helper()

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	var out bytes.Buffer
	w, err := age.Encrypt(&out, identity.Recipient())
	require.NoError(t, err)
	_, err = w.Write([]byte("plaintext the server never sees"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	return out.Bytes()
}

// Two distinct valid ciphertexts, generated once, for tests that only need
// "a value the server accepts" and, where a test replaces one value with
// another, a second that differs from it. Shared and read-only: never
// modify them.
var (
	sealedFixture       = mustSeal()
	sealedFixtureNew    = mustSeal()
	sealedFixtureB64    = base64.StdEncoding.EncodeToString(sealedFixture)
	sealedFixtureNewB64 = base64.StdEncoding.EncodeToString(sealedFixtureNew)
)

func mustSeal() []byte {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		panic(err)
	}

	var out bytes.Buffer
	w, err := age.Encrypt(&out, identity.Recipient())
	if err != nil {
		panic(err)
	}
	if _, err := w.Write([]byte("plaintext the server never sees")); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}

	return out.Bytes()
}

// noRecipientAgeFile is a syntactically valid age header with zero recipient
// stanzas: it announces the age format and ends the header, but nobody could
// ever decrypt it.
func noRecipientAgeFile() []byte {
	return []byte("age-encryption.org/v1\n--- " + strings.Repeat("A", 43) + "\n" + strings.Repeat("x", 40))
}

func rawRequest(t *testing.T, method, path, body, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	return req
}

func TestCreateObjectAcceptsAWellFormedAgeCiphertext(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "well_formed", Value: sealedValue(t)}, issueToken(t, s)))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestWriteRejectsAValueThatIsNotAnAgeFile(t *testing.T) {
	t.Parallel()

	notAge := []byte("this is plaintext, not an age file")

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		mux, s := newTestMux(t)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "not_age", Value: notAge}, issueToken(t, s)))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())

		_, err := s.GetObject(t.Context(), "not_age")
		require.Error(t, err, "a rejected value must store nothing")
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		mux, s := newTestMux(t)
		original := sealedValue(t)
		require.NoError(t, s.CreateObject(t.Context(), "to_update", original, nil, "", ""))

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, updateRequest(t, "to_update", notAge, issueToken(t, s)))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())

		obj, err := s.GetObject(t.Context(), "to_update")
		require.NoError(t, err)
		require.Equal(t, original, obj.Value, "a rejected update must leave the stored value alone")
	})
}

func TestWriteRejectsAnAgeHeaderWithNoRecipients(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		mux, s := newTestMux(t)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "no_recipients", Value: noRecipientAgeFile()}, issueToken(t, s)))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "recipient", "the message should name the problem")
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		mux, s := newTestMux(t)
		require.NoError(t, s.CreateObject(t.Context(), "to_update", sealedValue(t), nil, "", ""))

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, updateRequest(t, "to_update", noRecipientAgeFile(), issueToken(t, s)))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "recipient")
	})
}

// An invalid base64 value is also a 422, not the 400 a body that doesn't
// parse at all gets: the body is well-formed JSON, the value in it isn't an
// age file.
func TestWriteRejectsAValueThatIsNotBase64(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		mux, s := newTestMux(t)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, rawRequest(t, http.MethodPost, "/objects", `{"slug":"bad_b64","value":"!!!not base64!!!"}`, issueToken(t, s)))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		mux, s := newTestMux(t)
		require.NoError(t, s.CreateObject(t.Context(), "to_update", sealedValue(t), nil, "", ""))

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, rawRequest(t, http.MethodPut, "/objects/to_update", `{"value":"!!!not base64!!!"}`, issueToken(t, s)))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	})
}

// A body that isn't JSON at all is still the 400 it always was.
func TestWriteStillRejectsAMalformedBodyWithA400(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, rawRequest(t, http.MethodPost, "/objects", `{not json`, issueToken(t, s)))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body hushhush.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
}
