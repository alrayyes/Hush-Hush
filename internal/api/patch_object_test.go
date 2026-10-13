package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// PATCH /objects/{slug} edits an object's metadata - description, tags and
// slug - without touching its sealed value.

func patchObjectRequest(t *testing.T, target, body, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPatch, target, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func patchObject(t *testing.T, mux http.Handler, target, body, token string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, patchObjectRequest(t, target, body, token))

	return rec
}

func seedPatchable(t *testing.T, s *store.Store) {
	t.Helper()

	require.NoError(t, s.CreateObject(context.Background(), "webhook", []byte("sealed"), []string{"homelab/vps-docker"}, "old note", "", store.WithTags([]string{"prod"})))
}

func TestPatchObjectChangesTheDescriptionAndLeavesTheRestAlone(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)
	before, err := s.GetObject(t.Context(), "webhook")
	require.NoError(t, err)

	rec := patchObject(t, mux, "/objects/webhook", `{"description":"new note"}`, issueToken(t, s))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "new note", got.Description)
	require.Equal(t, "webhook", got.Slug)
	require.Equal(t, []string{"prod"}, got.Tags)
	require.Equal(t, []string{"homelab/vps-docker"}, got.UsedBy)

	after, err := s.GetObject(t.Context(), "webhook")
	require.NoError(t, err)
	require.Equal(t, before.Value, after.Value, "the sealed value is not touched")
	require.Equal(t, before.ID, after.ID)
}

func TestPatchObjectWithAnEmptyDescriptionClearsIt(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)

	rec := patchObject(t, mux, "/objects/webhook", `{"description":""}`, issueToken(t, s))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got, err := s.GetObject(t.Context(), "webhook")
	require.NoError(t, err)
	require.Empty(t, got.Description)
}

func TestPatchObjectReplacesTheTagsAndAnEmptyListClearsThem(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)
	token := issueToken(t, s)

	rec := patchObject(t, mux, "/objects/webhook", `{"tags":["Staging","web"]}`, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"staging", "web"}, got.Tags)

	rec = patchObject(t, mux, "/objects/webhook", `{"tags":[]}`, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Empty(t, got.Tags)
}

func TestPatchObjectWithAnInvalidTagIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)

	rec := patchObject(t, mux, "/objects/webhook", `{"tags":["Not A Tag!"]}`, issueToken(t, s))

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestPatchObjectRenamesTheSlugAndKeepsItsConsumers(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)
	token := issueToken(t, s)

	rec := patchObject(t, mux, "/objects/webhook", `{"slug":"deploy_webhook"}`, token)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "deploy_webhook", got.Slug)
	require.Equal(t, []string{"homelab/vps-docker"}, got.UsedBy)

	_, err := s.GetObject(t.Context(), "webhook")
	require.ErrorIs(t, err, store.ErrNotFound, "the old slug is gone")

	moved, err := s.GetObject(t.Context(), "deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []byte("sealed"), moved.Value)
}

func TestPatchObjectRenameMovesEveryVariantOfTheName(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rec := patchObject(t, f.mux, "/objects/release_token?id="+f.idABC, `{"slug":"deploy_token"}`, f.write)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	_, err := f.s.GetObject(t.Context(), "release_token", store.WithID(f.idD))
	require.ErrorIs(t, err, store.ErrNotFound)

	for _, id := range []string{f.idABC, f.idD} {
		moved, err := f.s.GetObject(t.Context(), "deploy_token", store.WithID(id))
		require.NoError(t, err, id)
		require.Equal(t, id, moved.ID)
	}
}

func TestPatchObjectRenameToATakenSlugIsAConflictAndChangesNothing(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)
	require.NoError(t, s.CreateObject(t.Context(), "taken", []byte("other"), nil, "", ""))

	rec := patchObject(t, mux, "/objects/webhook", `{"slug":"taken","description":"changed"}`, issueToken(t, s))

	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	got, err := s.GetObject(t.Context(), "webhook")
	require.NoError(t, err)
	require.Equal(t, "old note", got.Description, "a refused rename applies none of the request")
}

func TestPatchObjectRenameToAMalformedSlugIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)
	token := issueToken(t, s)

	for _, slug := range []string{"Has Space", "-leading", "a/b", "dot.ted", ""} {
		body, err := json.Marshal(map[string]string{"slug": slug})
		require.NoError(t, err)

		rec := patchObject(t, mux, "/objects/webhook", string(body), token)

		require.Equal(t, http.StatusBadRequest, rec.Code, slug)
	}

	_, err := s.GetObject(t.Context(), "webhook")
	require.NoError(t, err)
}

func TestPatchObjectWithNothingToChangeIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)

	rec := patchObject(t, mux, "/objects/webhook", `{}`, issueToken(t, s))

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestPatchObjectOfAnUnknownNameIsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := patchObject(t, mux, "/objects/nothing_here", `{"description":"x"}`, issueToken(t, s))

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestPatchObjectOfANameWithSeveralVariantsNeedsAnID(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	rec := patchObject(t, f.mux, "/objects/release_token", `{"description":"x"}`, f.write)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	rec = patchObject(t, f.mux, "/objects/release_token?id="+f.idD, `{"description":"only d"}`, f.write)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	abc, err := f.s.GetObject(t.Context(), "release_token", store.WithID(f.idABC))
	require.NoError(t, err)
	require.Empty(t, abc.Description, "a description change reaches the named variant only")
}

func TestPatchObjectIsAuditedAsAnUpdate(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)

	rec := patchObject(t, mux, "/objects/webhook", `{"slug":"deploy_webhook"}`, issueToken(t, s))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	entries, err := s.QueryAuditLog(t.Context(), store.AuditLogFilter{ObjectID: "deploy_webhook"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, store.AuditActionUpdate, entries[0].Action)
	require.NotEmpty(t, entries[0].VariantID)
}

func TestPatchObjectWithoutACredentialIsUnauthorized(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)

	rec := patchObject(t, mux, "/objects/webhook", `{"description":"x"}`, "")

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPatchObjectRenameToAnUppercaseSlugIsAccepted(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedPatchable(t, s)
	token := issueToken(t, s)

	rec := patchObject(t, mux, "/objects/webhook", `{"slug":"CF_DNS_API_TOKEN_SECRET"}`, token)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	_, err := s.GetObject(t.Context(), "CF_DNS_API_TOKEN_SECRET")
	require.NoError(t, err)
}
