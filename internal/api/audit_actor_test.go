package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// audit-log/spec.md's "Revoked tokens stay attributable" requirement:
// a token's audit trail keeps resolving to its real description and
// owner even after it's revoked, since GET /tokens' own soft-delete
// (alrayyes/hush-hush#204) keeps a revoked token listed rather than
// removed.
func TestRevokedTokensAuditEntryStaysAttributable(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	wt, token, err := s.CreateWriteToken(t.Context(), "revoked but attributable", time.Hour, "")
	require.NoError(t, err)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "revoked_actor", Value: []byte("sealed-ciphertext")}, token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	require.NoError(t, s.RevokeWriteToken(t.Context(), wt.ID))

	entries, err := s.QueryAuditLog(t.Context(), store.AuditLogFilter{Actor: wt.ID})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	tokens, err := s.ListWriteTokens(t.Context())
	require.NoError(t, err)
	var resolved *hushhush.TokenMetadata
	for _, tok := range tokens {
		if tok.ID == entries[0].ActorID {
			m := hushhush.TokenMetadata{ID: tok.ID, Description: tok.Description, Owner: tok.Owner, Revoked: tok.Revoked}
			resolved = &m
		}
	}
	require.NotNil(t, resolved)
	require.Equal(t, "revoked but attributable", resolved.Description)
	require.True(t, resolved.Revoked)
}

func TestQueryAuditLogFiltersByActorOverHTTP(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	wt, token, err := s.CreateWriteToken(t.Context(), "filtered", time.Hour, "")
	require.NoError(t, err)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "actor_filter_target", Value: []byte("v")}, token)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	other := issueToken(t, s)
	otherReq := createRequest(t, hushhush.CreateObjectRequest{ID: "actor_filter_other", Value: []byte("v")}, other)
	mux.ServeHTTP(httptest.NewRecorder(), otherReq)

	getReq := httptest.NewRequest(http.MethodGet, "/audit-log?actor="+wt.ID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, getReq)
	require.Equal(t, http.StatusOK, rec.Code)

	var entries []hushhush.AuditLogEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "actor_filter_target", entries[0].ObjectID)
	require.Equal(t, wt.ID, entries[0].ActorID)
}
