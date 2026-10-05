package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// One name can hold a different value per consumer, each under its own UUID
// (alrayyes/hush-hush#668).

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type variantFixture struct {
	mux   http.Handler
	s     *store.Store
	write string
	idABC string
	idD   string
}

func newVariantFixture(t *testing.T) variantFixture {
	t.Helper()

	mux, s := newTestMux(t)
	write := issueToken(t, s)

	create := func(value []byte, usedBy []string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "release_token", Value: value, UsedBy: usedBy}, write))

		return rec
	}

	idOf := func(rec *httptest.ResponseRecorder) string {
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

		var got hushhush.ObjectMetadata
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Regexp(t, uuidPattern, got.ID, "a create answers with the object's uuid")

		return got.ID
	}

	return variantFixture{
		mux: mux, s: s, write: write,
		idABC: idOf(create(sealedFixture, []string{"a", "b", "c"})),
		idD:   idOf(create(sealedFixtureNew, []string{"d"})),
	}
}

func (f variantFixture) do(t *testing.T, method, target, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}

	req := httptest.NewRequest(method, target, r)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)

	return rec
}

func (f variantFixture) consumerToken(t *testing.T, consumer string) string {
	t.Helper()

	_, token, err := f.s.CreateConsumerToken(t.Context(), consumer, "t", time.Hour)
	require.NoError(t, err)

	return token
}

func TestEachConsumerReadsItsOwnValueUnderTheSameName(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	for _, c := range []string{"a", "b", "c"} {
		rec := f.do(t, http.MethodGet, "/objects/release_token", f.consumerToken(t, c), nil)
		require.Equal(t, http.StatusOK, rec.Code, c)
		require.Equal(t, sealedFixture, rec.Body.Bytes(), c)
	}

	rec := f.do(t, http.MethodGet, "/objects/release_token", f.consumerToken(t, "d"), nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, sealedFixtureNew, rec.Body.Bytes())
}

func TestAConsumerWithNoVariantGetsNotFound(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rec := f.do(t, http.MethodGet, "/objects/release_token", f.consumerToken(t, "e"), nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"error":"unknown object"}`, rec.Body.String(), "the same answer as a name that doesn't exist")
}

func TestAWriteTokenMustSayWhichVariantWhenThereAreSeveral(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		rec := f.do(t, method, "/objects/release_token", f.write, nil)
		require.Equal(t, http.StatusConflict, rec.Code, method+" "+rec.Body.String())
		require.Contains(t, rec.Body.String(), "id", method)
	}

	put, err := json.Marshal(hushhush.UpdateObjectRequest{Value: sealedFixtureNew})
	require.NoError(t, err)
	rec := f.do(t, http.MethodPut, "/objects/release_token", f.write, put)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	require.Equal(t, http.StatusConflict, f.do(t, http.MethodGet, "/objects/release_token/used-by", f.write, nil).Code)
}

func TestAnIDSelectsOneVariant(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rec := f.do(t, http.MethodGet, "/objects/release_token?id="+f.idD, f.write, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, sealedFixtureNew, rec.Body.Bytes())

	rec = f.do(t, http.MethodGet, "/objects/release_token/used-by?id="+f.idD, f.write, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"used_by":["d"]}`, rec.Body.String())

	put, err := json.Marshal(hushhush.UpdateObjectRequest{Value: sealedFixture})
	require.NoError(t, err)
	rec = f.do(t, http.MethodPut, "/objects/release_token?id="+f.idD, f.write, put)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, f.idD, meta.ID)
	require.Equal(t, []string{"d"}, meta.UsedBy)

	require.Equal(t, http.StatusNoContent, f.do(t, http.MethodDelete, "/objects/release_token?id="+f.idD, f.write, nil).Code)

	rec = f.do(t, http.MethodGet, "/objects/release_token", f.write, nil)
	require.Equal(t, http.StatusOK, rec.Code, "one variant left, so no id is needed again")
	require.Equal(t, sealedFixture, rec.Body.Bytes())
}

func TestAnIDFromAnotherNameIsNotFound(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "other", Value: sealedFixture}, f.write))
	require.Equal(t, http.StatusCreated, rec.Code)

	var other hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &other))

	require.Equal(t, http.StatusNotFound, f.do(t, http.MethodGet, "/objects/release_token?id="+other.ID, f.write, nil).Code)
	require.Equal(t, http.StatusBadRequest, f.do(t, http.MethodGet, "/objects/release_token?id=not-a-uuid", f.write, nil).Code)
}

func TestCreatingAVariantForAConsumerThatHasOneIsAConflict(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	for _, usedBy := range [][]string{{"e", "a"}, nil} {
		rec := httptest.NewRecorder()
		f.mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "release_token", Value: sealedFixture, UsedBy: usedBy}, f.write))
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	}

	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "release_token", Value: sealedFixture, UsedBy: []string{"e"}}, f.write))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestAnUpdateOrRenameThatPutsAConsumerInTwoVariantsIsAConflict(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	moved := []string{"d", "a"}
	put, err := json.Marshal(hushhush.UpdateObjectRequest{Value: sealedFixture, UsedBy: &moved})
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, f.do(t, http.MethodPut, "/objects/release_token?id="+f.idD, f.write, put).Code)

	rename, err := json.Marshal(hushhush.UpdateConsumerRequest{Name: "d"})
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, f.do(t, http.MethodPatch, "/consumers/a", f.write, rename).Code)

	d := f.do(t, http.MethodGet, "/objects/release_token?id="+f.idD, f.write, nil)
	require.Equal(t, sealedFixtureNew, d.Body.Bytes(), "nothing changed")
}

func TestTheListCarriesEveryVariantWithItsID(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rec := f.do(t, http.MethodGet, "/objects", f.write, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var list []hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list, 2)

	ids := map[string]bool{}
	for _, o := range list {
		require.Equal(t, "release_token", o.Slug)
		ids[o.ID] = true
	}

	require.Equal(t, map[string]bool{f.idABC: true, f.idD: true}, ids)
}

func TestMCPToolsTakeAnIDForAVariant(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	result := callTool(t, f.mux, f.write, "get", map[string]any{"slug": "release_token"})
	require.True(t, result.IsError, "several variants and no id")

	result = callTool(t, f.mux, f.write, "get", map[string]any{"slug": "release_token", "id": f.idD})
	require.False(t, result.IsError, "content: %+v", result.Content)

	result = callTool(t, f.mux, f.write, "delete", map[string]any{"slug": "release_token", "id": f.idD})
	require.False(t, result.IsError, "content: %+v", result.Content)

	_, err := f.s.GetObject(t.Context(), "release_token", store.WithID(f.idD))
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestSpecDocumentsTheIDAndTheSelector(t *testing.T) {
	t.Parallel()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	id := doc.Components.Schemas["ObjectMetadata"].Value.Properties["id"]
	require.NotNil(t, id, "ObjectMetadata carries the object's uuid")
	require.Equal(t, "uuid", id.Value.Format)

	for path, methods := range map[string][]string{
		"/objects/{slug}":         {http.MethodGet, http.MethodPut, http.MethodDelete},
		"/objects/{slug}/used-by": {http.MethodGet},
	} {
		for _, method := range methods {
			op := doc.Paths.Value(path).GetOperation(method)

			var found bool

			for _, p := range op.Parameters {
				if p.Value.Name == "id" && p.Value.In == "query" {
					found = true

					require.Equal(t, "uuid", p.Value.Schema.Value.Format, method+" "+path)
				}
			}

			require.True(t, found, "%s %s takes an id", method, path)
			require.NotNil(t, op.Responses.Value("409"), "%s %s can be a 409 when the name has several variants", method, path)
		}
	}
}
