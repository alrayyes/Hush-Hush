package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMCPUsedByReturnsTheRecordedConsumers(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "mcp_used_by_target", []byte("v"), []string{"homelab/vps-docker", "ci"}, "", ""))

	result := callTool(t, mux, token, "used_by", map[string]any{"slug": "mcp_used_by_target"})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var out struct {
		UsedBy []string `json:"used_by"`
	}
	require.NoError(t, json.Unmarshal(result.StructuredContent, &out))
	require.ElementsMatch(t, []string{"homelab/vps-docker", "ci"}, out.UsedBy)
}

func TestMCPUsedByOfAnObjectWithNoConsumersIsAnEmptyList(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_used_by_none")

	result := callTool(t, mux, token, "used_by", map[string]any{"slug": "mcp_used_by_none"})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.JSONEq(t, `{"used_by":[]}`, string(result.StructuredContent))
}

func TestMCPUsedByOfAnUnknownObjectIsAToolError(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	result := callTool(t, mux, issueToken(t, s), "used_by", map[string]any{"slug": "mcp_used_by_missing"})
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "unknown object")
}
