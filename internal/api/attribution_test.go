package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// Session-attributed writes (openspec/changes/web-ui/specs/auth/spec.md's
// "Session-attributed writes" requirement): a session-authenticated
// create/update/delete records the admin account as the audit log
// entry's verified actor, alongside - not instead of - any self-reported
// X-Caller header. A bearer-token-authenticated one keeps recording no
// actor for now (alrayyes/hush-hush#214 adds token attribution).

func TestSessionCreateIsAttributedToTheAdminAccount(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/objects",
		bytes.NewReader([]byte(`{"id":"attributed_create","value":"c2VhbGVkLWNpcGhlcnRleHQ="}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Caller", "homelab/vps-docker")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	entries, err := s.QueryAuditLog(t.Context(), store.AuditLogFilter{ObjectID: "attributed_create"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "session", entries[0].ActorType)
	require.Equal(t, "admin", entries[0].ActorID)
	require.Equal(t, "homelab/vps-docker", entries[0].Caller)
}

func TestSessionUpdateIsAttributedToTheAdminAccount(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedObject(t, s, "attributed_update")
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/objects/attributed_update",
		bytes.NewReader([]byte(`{"value":"bmV3LXNlYWxlZC12YWx1ZQ=="}`)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// seedObject's own create call already logged one entry for this
	// object id - the update this test cares about is the last one.
	entries, err := s.QueryAuditLog(t.Context(), store.AuditLogFilter{ObjectID: "attributed_update"})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	last := entries[len(entries)-1]
	require.Equal(t, store.AuditActionUpdate, last.Action)
	require.Equal(t, "session", last.ActorType)
	require.Equal(t, "admin", last.ActorID)
}

func TestSessionDeleteIsAttributedToTheAdminAccount(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	seedObject(t, s, "attributed_delete")
	sessionCookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/objects/attributed_delete", nil)
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	// seedObject's own create call already logged one entry for this
	// object id - the delete this test cares about is the last one.
	entries, err := s.QueryAuditLog(t.Context(), store.AuditLogFilter{ObjectID: "attributed_delete"})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	last := entries[len(entries)-1]
	require.Equal(t, store.AuditActionDelete, last.Action)
	require.Equal(t, "session", last.ActorType)
	require.Equal(t, "admin", last.ActorID)
}

func TestBearerTokenWriteHasNoActorYet(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "attributed_by_token", Value: []byte("sealed-ciphertext")}, issueToken(t, s))
	req.Header.Set("X-Caller", "ci-pipeline")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	entries, err := s.QueryAuditLog(t.Context(), store.AuditLogFilter{ObjectID: "attributed_by_token"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Empty(t, entries[0].ActorType)
	require.Empty(t, entries[0].ActorID)
	require.Equal(t, "ci-pipeline", entries[0].Caller)
}
