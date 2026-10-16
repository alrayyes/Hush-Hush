package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"filippo.io/age"
	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestListConsumersReturnsEachDistinctNameOnce(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "a", []byte("v"), []string{"homelab/vps-docker", "homelab/mattermost"}, "", ""))
	require.NoError(t, s.CreateObject(context.Background(), "b", []byte("v"), []string{"homelab/mattermost"}, "", ""))

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, []string{"homelab/mattermost", "homelab/vps-docker"}, body)
}

func TestListConsumersReturnsEmptyArrayWhenNoneExist(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[]`, rec.Body.String())
}

func TestListConsumersWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListConsumersWithASessionSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func seedConsumersFixture(t *testing.T, s *store.Store) {
	t.Helper()

	require.NoError(t, s.CreateObject(context.Background(), "a", []byte("v"), []string{"homelab/vps-docker", "homelab/mattermost"}, "", ""))
	require.NoError(t, s.CreateObject(context.Background(), "b", []byte("v"), []string{"homelab/mattermost"}, "", ""))
	require.NoError(t, s.CreateObject(context.Background(), "c", []byte("v"), []string{"work/ci-runner"}, "", ""))
}

func TestListConsumersWithQFilterReturnsAPaginatedResponse(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers?q=homelab", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumersPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 2, body.Total)
	require.Equal(t, []hushhush.ConsumerEntry{
		{Name: "homelab/mattermost", SecretCount: 2},
		{Name: "homelab/vps-docker", SecretCount: 1},
	}, body.Consumers)
}

func TestListConsumersWithPageAndPageSizeReturnsOnePage(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers?page=2&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumersPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 3, body.Total)
	require.Equal(t, []hushhush.ConsumerEntry{{Name: "work/ci-runner", SecretCount: 1}}, body.Consumers)
}

// The most-used consumer sorts last by name, so a listing ordered by anything
// but name puts different consumers on page 1.
func TestListConsumersPageIsSortedByNameWhateverTheCounts(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"zeta", "mu", "alpha"}, "", ""))
	require.NoError(t, s.CreateObject(ctx, "b", []byte("v"), []string{"zeta", "mu", "beta"}, "", ""))
	require.NoError(t, s.CreateObject(ctx, "c", []byte("v"), []string{"zeta"}, "", ""))

	req := httptest.NewRequest(http.MethodGet, "/consumers?page=1&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumersPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 4, body.Total)
	require.Equal(t, []hushhush.ConsumerEntry{
		{Name: "alpha", SecretCount: 1},
		{Name: "beta", SecretCount: 1},
	}, body.Consumers)
}

func TestListConsumersNoParametersDefaultsToTheUnpaginatedListing(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, []string{"homelab/mattermost", "homelab/vps-docker", "work/ci-runner"}, body)
}

func TestListConsumersWithInvalidPageIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers?page=0", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListConsumersWithInvalidPageSizeIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/consumers?page_size=101", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func renameConsumerRequest(t *testing.T, name, newName, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPatch, "/consumers/"+name,
		bytes.NewReader([]byte(`{"name":"`+newName+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestRenameConsumerUpdatesEveryObjectAndReturnsTheNewEntry(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renameConsumerRequest(t, "homelab/vps-docker", "homelab/vps-docker-2", issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, hushhush.ConsumerEntry{Name: "homelab/vps-docker-2", SecretCount: 1}, body)

	obj, err := s.GetObject(context.Background(), "a")
	require.NoError(t, err)
	require.Contains(t, obj.UsedBy, "homelab/vps-docker-2")
	require.NotContains(t, obj.UsedBy, "homelab/vps-docker")
}

func TestRenameConsumerWithASlashInThePathSucceeds(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	// The consumer name itself contains a "/" - the route has to treat
	// everything after /consumers/ as the name rather than a path
	// segment boundary (api/openapi.yaml's consumerName parameter).
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renameConsumerRequest(t, "homelab/mattermost", "homelab/mattermost-2", issueToken(t, s)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestRenameConsumerUnknownNameReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renameConsumerRequest(t, "nonexistent", "new", issueToken(t, s)))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRenameConsumerWithoutNameIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodPatch, "/consumers/homelab/vps-docker", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRenameConsumerWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renameConsumerRequest(t, "homelab/vps-docker", "new", ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionRenamesConsumerWithItsCSRFToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := renameConsumerRequest(t, "homelab/vps-docker", "new", "")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestSessionRenamesConsumerWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)

	req := renameConsumerRequest(t, "homelab/vps-docker", "new", "")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func deleteConsumerRequest(t *testing.T, name, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, "/consumers/"+name, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestDeleteConsumerRemovesItFromEveryObject(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteConsumerRequest(t, "homelab/mattermost", issueToken(t, s)))

	require.Equal(t, http.StatusNoContent, rec.Code)

	consumers, err := s.ListConsumers(context.Background())
	require.NoError(t, err)
	require.NotContains(t, consumers, "homelab/mattermost")

	obj, err := s.GetObject(context.Background(), "a")
	require.NoError(t, err)
	require.NotContains(t, obj.UsedBy, "homelab/mattermost")
}

func TestDeleteConsumerUnknownNameReturnsNotFound(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteConsumerRequest(t, "nonexistent", issueToken(t, s)))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteConsumerWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deleteConsumerRequest(t, "homelab/mattermost", ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionDeletesConsumerWithItsCSRFToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := deleteConsumerRequest(t, "homelab/mattermost", "")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

func TestSessionDeletesConsumerWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)

	req := deleteConsumerRequest(t, "homelab/mattermost", "")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func addConsumerRequest(t *testing.T, name, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/consumers",
		bytes.NewReader([]byte(`{"name":"`+name+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestAddConsumerCreatesEntryWithZeroSecrets(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, addConsumerRequest(t, "homelab/new-device", issueToken(t, s)))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var body hushhush.ConsumerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, hushhush.ConsumerEntry{Name: "homelab/new-device", SecretCount: 0}, body)

	consumers, err := s.ListConsumers(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/new-device"}, consumers)
}

func TestAddConsumerDuplicateNameReturnsConflict(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, addConsumerRequest(t, "homelab/mattermost", issueToken(t, s)))

	require.Equal(t, http.StatusConflict, rec.Code)

	// The page shows this text as it is (frontend.md: the server's own
	// message), so it is part of the contract.
	var body hushhush.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "consumer already exists", body.Error)
}

func TestAddConsumerWithoutNameIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/consumers", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAddConsumerWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, addConsumerRequest(t, "homelab/new-device", ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionAddsConsumerWithItsCSRFToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := addConsumerRequest(t, "homelab/new-device", "")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestSessionAddsConsumerWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)

	req := addConsumerRequest(t, "homelab/new-device", "")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func updateConsumerRequest(t *testing.T, name, body, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPatch, "/consumers/"+name, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

// TestRegisterConsumerPublicKeyReturnsItAlongsideNameAndCount covers
// specs/consumers/spec.md's "Registering a consumer's public key"
// scenario: registering a key for a consumer already recorded via
// used_by (the common case - AddConsumer itself would reject that name
// as already existing), then a subsequent GET /consumers returns it
// alongside name and count.
func TestRegisterConsumerPublicKeyReturnsItAlongsideNameAndCount(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	token := issueToken(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateConsumerRequest(t, "homelab/vps-docker", `{"public_key":"age1xl8amwv0vaxfk3gkrsgfv8l9nwkfuyqf448jwauhkn4vsw9jfqzq5290c0"}`, token))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, hushhush.ConsumerEntry{Name: "homelab/vps-docker", SecretCount: 1, PublicKey: "age1xl8amwv0vaxfk3gkrsgfv8l9nwkfuyqf448jwauhkn4vsw9jfqzq5290c0"}, body)

	req := httptest.NewRequest(http.MethodGet, "/consumers?q=homelab/vps-docker", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, req)

	require.Equal(t, http.StatusOK, listRec.Code, listRec.Body.String())

	var page hushhush.ConsumersPage
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &page))
	require.Equal(t, []hushhush.ConsumerEntry{
		{Name: "homelab/vps-docker", SecretCount: 1, PublicKey: "age1xl8amwv0vaxfk3gkrsgfv8l9nwkfuyqf448jwauhkn4vsw9jfqzq5290c0"},
	}, page.Consumers)
}

// TestRegisterConsumerPublicKeyForANameNeverSeenBefore proves registering
// a key doesn't require the consumer to already exist in either the
// consumers table or any object's used_by list - specs/consumers/spec.md
// doesn't restrict registration that way, and a name only ever gains a
// consumers row this way otherwise.
func TestRegisterConsumerPublicKeyForANameNeverSeenBefore(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateConsumerRequest(t, "homelab/pre-registered", `{"public_key":"age1rl48fde34aq8m4hsa73wzraa2enevmzqexxd6rfn25vewxvuspas2cs0l4"}`, token))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, hushhush.ConsumerEntry{Name: "homelab/pre-registered", SecretCount: 0, PublicKey: "age1rl48fde34aq8m4hsa73wzraa2enevmzqexxd6rfn25vewxvuspas2cs0l4"}, body)
}

// TestConsumerWithNoRegisteredPublicKeyOmitsTheField covers
// specs/consumers/spec.md's "Consumer with no registered public key"
// scenario: the field is absent from the response entirely, not present
// with an error or a null.
func TestConsumerWithNoRegisteredPublicKeyOmitsTheField(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)

	req := httptest.NewRequest(http.MethodGet, "/consumers?q=homelab/vps-docker", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, s))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "public_key")
}

func TestUpdateConsumerRenameAndPublicKeyTogether(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	token := issueToken(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateConsumerRequest(t, "homelab/vps-docker",
		`{"name":"homelab/vps-docker-2","public_key":"age12vgujjyk2cc59whutam7d7xxgd4avmq09jrtyhw6g46dg5txdc6qmenvhy"}`, token))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.ConsumerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, hushhush.ConsumerEntry{Name: "homelab/vps-docker-2", SecretCount: 1, PublicKey: "age12vgujjyk2cc59whutam7d7xxgd4avmq09jrtyhw6g46dg5txdc6qmenvhy"}, body)
}

func TestSessionRegistersConsumerPublicKeyWithItsCSRFToken(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := updateConsumerRequest(t, "homelab/vps-docker", `{"public_key":"age1ak6pxq5vahftyf04ctc4fqfesaksvc0lua6yf3zdlwwp9hs7zursuhpl9w"}`, "")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestSessionRegistersConsumerPublicKeyWithoutCSRFTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedConsumersFixture(t, s)
	sessionCookie := seedSession(t, s)

	req := updateConsumerRequest(t, "homelab/vps-docker", `{"public_key":"age1ak6pxq5vahftyf04ctc4fqfesaksvc0lua6yf3zdlwwp9hs7zursuhpl9w"}`, "")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// A public key is stored as the consumer's sealing recipient, so one that
// isn't a valid age recipient is refused rather than found out when the
// first secret is sealed to it.
func TestUpdateConsumerRejectsAPublicKeyThatIsNotAnAgeRecipient(t *testing.T) {
	t.Parallel()

	secretKey, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	for name, key := range map[string]string{
		"random text":     "not-a-key",
		"truncated":       "age1xl8amwv0vaxfk3gkrsgfv8l9nwkfuyqf448jwauhkn4vsw9jf",
		"bad checksum":    "age1xl8amwv0vaxfk3gkrsgfv8l9nwkfuyqf448jwauhkn4vsw9jfqzq5290c1",
		"a private key":   secretKey.String(),
		"whitespace only": "   ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mux, s := newTestMux(t)
			token := issueToken(t, s)
			require.NoError(t, s.CreateObject(context.Background(), "keyed_secret", []byte("v"), []string{"homelab/vps-docker"}, "", ""))

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, updateConsumerRequest(t, "homelab/vps-docker", fmt.Sprintf(`{"public_key":%q}`, key), token))

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.NotContains(t, rec.Body.String(), key)
			// The dialog shows this text as it is, so it is part of the contract.
			require.Contains(t, rec.Body.String(), "public_key must be an age public key")

			page, err := s.ListConsumersPage(t.Context(), store.ConsumerFilter{Page: 1, PageSize: 10})
			require.NoError(t, err)
			require.Empty(t, page.Consumers[0].PublicKey)
		})
	}
}

func TestUpdateConsumerRejectingTheKeyAlsoSkipsTheRename(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "keyed_secret", []byte("v"), []string{"homelab/vps-docker"}, "", ""))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, updateConsumerRequest(t, "homelab/vps-docker", `{"name":"homelab/renamed","public_key":"not-a-key"}`, token))

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	page, err := s.ListConsumersPage(t.Context(), store.ConsumerFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, "homelab/vps-docker", page.Consumers[0].Name)
}
