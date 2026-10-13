package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// A refused consumer says which consumer clashed and with which secret
// (alrayyes/hush-hush#851), so the web UI can show the message as it is.

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	return body.Error
}

func TestACreateClashNamesTheConsumerAndTheSecretItIsIn(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "release_token", Value: sealedFixture, UsedBy: []string{"e", "a"}}, f.write))

	require.Equal(t, fmt.Sprintf(`consumer "a" is already in the secret named "release_token" (id %s)`, f.idABC), errorOf(t, rec))
}

func TestACreateWithNoConsumerForATakenNameSaysSo(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "release_token", Value: sealedFixture}, f.write))

	require.Equal(t, `a secret named "release_token" already exists and this one names no consumer`, errorOf(t, rec))
}

func TestAnUpdateClashNamesTheConsumerAndTheSecretItIsIn(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	moved := []string{"d", "a"}
	put, err := json.Marshal(hushhush.UpdateObjectRequest{Value: sealedFixture, UsedBy: &moved})
	require.NoError(t, err)

	rec := f.do(t, http.MethodPut, "/objects/release_token?id="+f.idD, f.write, put)

	require.Equal(t, fmt.Sprintf(`consumer "a" is already in the secret named "release_token" (id %s)`, f.idABC), errorOf(t, rec))
}

func TestAConsumerRenameClashNamesTheConsumerAndTheSecretItIsIn(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rename, err := json.Marshal(hushhush.UpdateConsumerRequest{Name: "d"})
	require.NoError(t, err)

	rec := f.do(t, http.MethodPatch, "/consumers/a", f.write, rename)

	require.Equal(t, fmt.Sprintf(`consumer "d" is already in the secret named "release_token" (id %s)`, f.idD), errorOf(t, rec))
}
