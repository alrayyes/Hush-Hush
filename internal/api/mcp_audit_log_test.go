package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

type mcpAuditEntry struct {
	ID       int64  `json:"id"`
	ObjectID string `json:"object_id"`
	Action   string `json:"action"`
}

func mcpAuditLog(t *testing.T, args map[string]any) ([]mcpAuditEntry, mcpCallToolResult) {
	t.Helper()

	return mcpAuditLogSeeded(t, func(s *store.Store) {
		ctx := context.Background()

		for _, e := range []struct{ id, action string }{{"a", "create"}, {"b", "create"}, {"a", "read"}} {
			require.NoError(t, s.RecordAuditLog(ctx, e.id, store.AuditAction(e.action), "ci", "203.0.113.1", "", ""))
		}
	}, args)
}

// mcpAuditLogSeeded runs the tool against a store the caller has filled.
func mcpAuditLogSeeded(t *testing.T, seed func(*store.Store), args map[string]any) ([]mcpAuditEntry, mcpCallToolResult) {
	t.Helper()

	mux, s := newTestMux(t)
	seed(s)

	result := callTool(t, mux, issueToken(t, s), "audit_log", args)

	var entries []mcpAuditEntry
	if !result.IsError {
		require.NoError(t, json.Unmarshal(result.StructuredContent, &entries))
	}

	return entries, result
}

func seedAuditRows(n int) func(*store.Store) {
	return func(s *store.Store) {
		for range n {
			if err := s.RecordAuditLog(context.Background(), "bulk", store.AuditActionRead, "ci", "203.0.113.1", "", ""); err != nil {
				panic(err)
			}
		}
	}
}

func TestMCPAuditLogReturnsEntriesOldestFirst(t *testing.T) {
	t.Parallel()

	entries, result := mcpAuditLog(t, map[string]any{})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, entries, 3)
	require.Equal(t, []string{"a", "b", "a"}, []string{entries[0].ObjectID, entries[1].ObjectID, entries[2].ObjectID})
}

func TestMCPAuditLogFiltersByObjectAndCaller(t *testing.T) {
	t.Parallel()

	entries, result := mcpAuditLog(t, map[string]any{"object_id": "a", "caller": "ci"})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, entries, 2)
	require.Equal(t, "read", entries[1].Action)
}

func TestMCPAuditLogPagesNewestFirstWithALimitAndACursor(t *testing.T) {
	t.Parallel()

	first, result := mcpAuditLog(t, map[string]any{"order": "desc", "limit": 2})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, first, 2)
	require.Equal(t, "read", first[0].Action, "newest first")
	require.Greater(t, first[0].ID, first[1].ID)
}

func TestMCPAuditLogAfterAEntryReturnsOnlyLaterOnes(t *testing.T) {
	t.Parallel()

	all, _ := mcpAuditLog(t, map[string]any{})
	require.Len(t, all, 3)

	later, result := mcpAuditLog(t, map[string]any{"after": all[0].ID})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, later, 2)
}

func TestMCPAuditLogRefusesABadFilter(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		args map[string]any
		want string
	}{
		"from not RFC 3339": {map[string]any{"from": "yesterday"}, "RFC 3339"},
		"unknown order":     {map[string]any{"order": "sideways"}, "asc or desc"},
		"limit zero":        {map[string]any{"limit": 0}, "limit"},
		"limit over 500":    {map[string]any{"limit": 501}, "limit"},
		"caller over 128":   {map[string]any{"caller": strings.Repeat("a", 129)}, "128"},
	} {
		_, result := mcpAuditLog(t, tc.args)
		require.True(t, result.IsError, name)
		require.Contains(t, result.Content[0].Text, tc.want, name)
	}
}

func TestMCPAuditLogReturnsFiftyEntriesWhenNoLimitIsGiven(t *testing.T) {
	t.Parallel()

	entries, result := mcpAuditLogSeeded(t, seedAuditRows(60), map[string]any{})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, entries, 50)
}

func TestMCPAuditLogAcceptsALimitOf500AndRefuses501(t *testing.T) {
	t.Parallel()

	entries, result := mcpAuditLogSeeded(t, seedAuditRows(60), map[string]any{"limit": 500})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, entries, 60)

	_, result = mcpAuditLogSeeded(t, seedAuditRows(1), map[string]any{"limit": 501})
	require.True(t, result.IsError)
}

func TestMCPAuditLogReadsTheNextPageBackWithBefore(t *testing.T) {
	t.Parallel()

	seed := seedAuditRows(5)

	first, result := mcpAuditLogSeeded(t, seed, map[string]any{"order": "desc", "limit": 2})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, first, 2)

	next, result := mcpAuditLogSeeded(t, seed, map[string]any{"order": "desc", "limit": 2, "before": first[1].ID})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, next, 2)
	require.Less(t, next[0].ID, first[1].ID, "the next page starts below the cursor")
}

func TestMCPAuditLogFiltersByActor(t *testing.T) {
	t.Parallel()

	seed := func(s *store.Store) {
		ctx := context.Background()
		require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionRead, "", "203.0.113.1", "token", "tok-1"))
		require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionRead, "", "203.0.113.1", "", ""))
	}

	byToken, result := mcpAuditLogSeeded(t, seed, map[string]any{"actor": "tok-1"})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, byToken, 1)
	require.Equal(t, "a", byToken[0].ObjectID)

	unauthenticated, result := mcpAuditLogSeeded(t, seed, map[string]any{"actor": "none"})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, unauthenticated, 1)
	require.Equal(t, "b", unauthenticated[0].ObjectID)
}

func TestMCPAuditLogFiltersByTime(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)

	fromFuture, result := mcpAuditLogSeeded(t, seedAuditRows(2), map[string]any{"from": future})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Empty(t, fromFuture, "nothing was recorded after an hour from now")

	toPast, result := mcpAuditLogSeeded(t, seedAuditRows(2), map[string]any{"to": past})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Empty(t, toPast, "nothing was recorded before an hour ago")

	inWindow, result := mcpAuditLogSeeded(t, seedAuditRows(2), map[string]any{"from": past, "to": future})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.Len(t, inWindow, 2)
}

func TestMCPAuditLogWithoutATokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, mcpRequest(t, mcpCallToolBody(t, "audit_log", map[string]any{}), ""))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
