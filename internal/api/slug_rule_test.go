package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// A secret's name follows one rule on create and on rename
// (alrayyes/hush-hush#859): letters of either case, digits, '_' and '-',
// starting with a letter or digit, up to 128 characters. Names are
// case-sensitive, so two that differ only by case are two secrets.

func TestCreatingASecretWithAnUppercaseNameIsAccepted(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "CF_DNS_API_TOKEN_SECRET", Value: sealedFixture}, token))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	_, err := s.GetObject(t.Context(), "CF_DNS_API_TOKEN_SECRET")
	require.NoError(t, err)
}

func TestNamesThatDifferOnlyByCaseAreTwoSecrets(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	for _, slug := range []string{"Api_Token", "api_token"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: slug, Value: sealedFixture}, token))
		require.Equal(t, http.StatusCreated, rec.Code, slug+" "+rec.Body.String())
	}
}

func TestCreatingASecretWithAMalformedNameIsRejectedWithTheRule(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	for _, slug := range []string{"Has Space", " lead", "-leading", "_leading", "a/b", "dot.ted", "new\nline", "ünï"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: slug, Value: sealedFixture}, token))

		require.Equal(t, http.StatusBadRequest, rec.Code, "%q: %s", slug, rec.Body.String())
		require.Contains(t, rec.Body.String(), "letters", "the 400 names the rule")
	}
}

func TestANameOver128CharactersIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: strings.Repeat("a", 129), Value: sealedFixture}, token))

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}
