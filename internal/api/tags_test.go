package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func postObject(t *testing.T, mux *http.ServeMux, token, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/objects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func putObject(t *testing.T, mux *http.ServeMux, token, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/objects/a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func listObjects(t *testing.T, mux *http.ServeMux, token, query string) []hushhush.ObjectMetadata {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/objects"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body []hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	return body
}

func TestCreateObjectWithTagsReturnsThemOnCreateAndList(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	rec := postObject(t, mux, token, `{"slug":"a","value":"`+sealedFixtureB64+`","tags":["prod","homelab"]}`)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, []string{"homelab", "prod"}, created.Tags)

	listed := listObjects(t, mux, token, "")
	require.Len(t, listed, 1)
	require.Equal(t, []string{"homelab", "prod"}, listed[0].Tags)
}

func TestListObjectsAlwaysCarriesTagsAsAnArray(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "a", sealedFixture, nil, "", ""))

	req := httptest.NewRequest(http.MethodGet, "/objects", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Contains(t, rec.Body.String(), `"tags":[]`)
}

func TestCreateObjectNormalisesTagsToLowercaseAndDedupes(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := postObject(t, mux, issueToken(t, s), `{"slug":"a","value":"`+sealedFixtureB64+`","tags":["Prod","prod"]}`)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, []string{"prod"}, created.Tags)
}

func TestCreateObjectRejectsInvalidTags(t *testing.T) {
	t.Parallel()

	tooMany := `["a","b","c","d","e","f","g","h","i","j","k"]`
	cases := map[string]string{
		"bad charset": `["has space"]`,
		"empty":       `[""]`,
		"too long":    `["` + strings.Repeat("a", 33) + `"]`,
		"too many":    tooMany,
	}

	for name, tags := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mux, s := newTestMux(t)

			rec := postObject(t, mux, issueToken(t, s), `{"slug":"a","value":"`+sealedFixtureB64+`","tags":`+tags+`}`)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), `"error"`)
		})
	}
}

func TestUpdateObjectTagsPreservedReplacedAndCleared(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "a", sealedFixture, nil, "", "", store.WithTags([]string{"prod"})))

	rec := putObject(t, mux, token, `{"value":"`+sealedFixtureNewB64+`"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []string{"prod"}, listObjects(t, mux, token, "")[0].Tags)

	rec = putObject(t, mux, token, `{"value":"`+sealedFixtureNewB64+`","tags":["homelab"]}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var meta hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, []string{"homelab"}, meta.Tags)

	rec = putObject(t, mux, token, `{"value":"`+sealedFixtureNewB64+`","tags":[]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, listObjects(t, mux, token, "")[0].Tags)
}

func TestUpdateObjectRejectsInvalidTags(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "a", sealedFixture, nil, "", ""))

	rec := putObject(t, mux, token, `{"value":"`+sealedFixtureNewB64+`","tags":["NO WAY"]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListObjectsFiltersByTagRepeatedMeansAll(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "both", sealedFixture, nil, "", "", store.WithTags([]string{"prod", "homelab"})))
	require.NoError(t, s.CreateObject(ctx, "prod_only", sealedFixture, nil, "", "", store.WithTags([]string{"prod"})))

	require.Len(t, listObjects(t, mux, token, "?tag=prod"), 2)

	both := listObjects(t, mux, token, "?tag=prod&tag=homelab")
	require.Len(t, both, 1)
	require.Equal(t, "both", both[0].Slug)

	require.Len(t, listObjects(t, mux, token, "?tag=PROD"), 2)
}

func TestListObjectsWithInvalidTagFilterIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/objects?tag=no%20way", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
