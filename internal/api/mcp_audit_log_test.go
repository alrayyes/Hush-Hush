package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

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

	mux, s := newTestMux(t)
	ctx := context.Background()

	for _, e := range []struct{ id, action string }{{"a", "create"}, {"b", "create"}, {"a", "read"}} {
		require.NoError(t, s.RecordAuditLog(ctx, e.id, store.AuditAction(e.action), "ci", "203.0.113.1", "", ""))
	}

	result := callTool(t, mux, issueToken(t, s), "audit_log", args)

	var entries []mcpAuditEntry
	if !result.IsError {
		require.NoError(t, json.Unmarshal(result.StructuredContent, &entries))
	}

	return entries, result
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
