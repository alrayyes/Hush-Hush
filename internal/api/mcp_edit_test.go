package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestMCPEditChangesTheDescriptionAndLeavesTheValueAlone(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_edit_target")

	result := callTool(t, mux, token, "edit", map[string]any{
		"slug": "mcp_edit_target", "description": "now with a label",
	})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var out struct {
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal(result.StructuredContent, &out))
	require.Equal(t, "mcp_edit_target", out.Slug)
	require.Equal(t, "now with a label", out.Description)

	obj, err := s.GetObject(context.Background(), "mcp_edit_target")
	require.NoError(t, err)
	require.Equal(t, sealedFixture, obj.Value)
	require.Equal(t, "now with a label", obj.Description)
}

func TestMCPEditRenamesAndReplacesTags(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_edit_old")

	result := callTool(t, mux, token, "edit", map[string]any{
		"slug": "mcp_edit_old", "new_slug": "mcp_edit_new", "tags": []string{"homelab"},
	})
	require.False(t, result.IsError, "content: %+v", result.Content)

	obj, err := s.GetObject(context.Background(), "mcp_edit_new")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab"}, obj.Tags)

	_, err = s.GetObject(context.Background(), "mcp_edit_old")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMCPEditRecordsAnUpdateAuditEntry(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_edit_audited")

	result := callTool(t, mux, token, "edit", map[string]any{"slug": "mcp_edit_audited", "description": "x"})
	require.False(t, result.IsError, "content: %+v", result.Content)

	entries, err := s.QueryAuditLog(context.Background(), store.AuditLogFilter{ObjectID: "mcp_edit_audited"})
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	require.Equal(t, store.AuditActionUpdate, entries[len(entries)-1].Action)
	require.Equal(t, "token", entries[len(entries)-1].ActorType)
}

func TestMCPEditWithNothingToChangeIsAToolError(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_edit_nothing")

	result := callTool(t, mux, token, "edit", map[string]any{"slug": "mcp_edit_nothing"})
	require.True(t, result.IsError)
}

func TestMCPEditRefusesAnInvalidNewSlugAnUnknownObjectAndATakenName(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_edit_a")
	seedObject(t, s, "mcp_edit_b")

	for name, args := range map[string]map[string]any{
		"invalid new slug": {"slug": "mcp_edit_a", "new_slug": "no spaces"},
		"unknown object":   {"slug": "mcp_edit_missing", "description": "x"},
		"name taken":       {"slug": "mcp_edit_a", "new_slug": "mcp_edit_b"},
	} {
		require.True(t, callTool(t, mux, token, "edit", args).IsError, name)
	}
}
