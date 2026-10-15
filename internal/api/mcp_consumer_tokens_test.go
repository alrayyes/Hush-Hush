package api_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type mcpConsumerToken struct {
	ID       string `json:"id"`
	Consumer string `json:"consumer"`
	Status   string `json:"status"`
	Value    string `json:"value"`
}

type mcpConsumerTokenList struct {
	Total  int                `json:"total"`
	Tokens []mcpConsumerToken `json:"tokens"`
}

// One pass through the five tools: each is called once, and the page answers
// the shape the others need (an id to act on, a status to read). What each
// operation does is the HTTP routes' tests.
func TestMCPConsumerTokenLifecycle(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	bearer := issueToken(t, s)

	call := func(tool string, args map[string]any) mcpCallToolResult {
		t.Helper()

		result := callTool(t, mux, bearer, tool, args)
		require.False(t, result.IsError, "%s: %+v", tool, result.Content)

		return result
	}

	var created mcpConsumerToken
	require.NoError(t, json.Unmarshal(call("create_consumer_token", map[string]any{"consumer": "ci", "description": "deploy"}).StructuredContent, &created))
	require.NotEmpty(t, created.Value, "the raw value is returned once, on create")

	var listed mcpConsumerTokenList
	require.NoError(t, json.Unmarshal(call("list_consumer_tokens", map[string]any{}).StructuredContent, &listed))
	require.Equal(t, 1, listed.Total)
	require.Equal(t, created.ID, listed.Tokens[0].ID)

	var raw struct {
		Tokens []map[string]any `json:"tokens"`
	}
	require.NoError(t, json.Unmarshal(call("list_consumer_tokens", map[string]any{}).StructuredContent, &raw))
	require.NotContains(t, raw.Tokens[0], "value", "a listing never carries a raw value")

	var rotated mcpConsumerToken
	require.NoError(t, json.Unmarshal(call("rotate_consumer_token", map[string]any{"id": created.ID}).StructuredContent, &rotated))
	require.NotEqual(t, created.Value, rotated.Value)

	call("revoke_consumer_token", map[string]any{"id": created.ID})
	call("purge_consumer_token", map[string]any{"id": created.ID})

	require.NoError(t, json.Unmarshal(call("list_consumer_tokens", map[string]any{}).StructuredContent, &listed))
	require.Equal(t, 0, listed.Total)
}

func TestMCPPurgeOfAnActiveConsumerTokenIsAToolError(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	bearer := issueToken(t, s)

	var created mcpConsumerToken
	require.NoError(t, json.Unmarshal(callTool(t, mux, bearer, "create_consumer_token", map[string]any{"consumer": "ci"}).StructuredContent, &created))

	result := callTool(t, mux, bearer, "purge_consumer_token", map[string]any{"id": created.ID})
	require.True(t, result.IsError, "a token has to be revoked or expired first, as over HTTP")
}
