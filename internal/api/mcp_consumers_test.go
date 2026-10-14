package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

type mcpConsumersPage struct {
	Consumers []struct {
		Name        string `json:"name"`
		SecretCount int    `json:"secret_count"`
	} `json:"consumers"`
	Total int `json:"total"`
}

func TestMCPListConsumersCountsTheSecretsEachOneIsUsedBy(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "mcp_consumers_a", []byte("v"), []string{"ci", "homelab"}, "", ""))
	require.NoError(t, s.CreateObject(context.Background(), "mcp_consumers_b", []byte("v"), []string{"ci"}, "", ""))

	result := callTool(t, mux, token, "list_consumers", map[string]any{})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var page mcpConsumersPage
	require.NoError(t, json.Unmarshal(result.StructuredContent, &page))
	require.Equal(t, 2, page.Total)
	require.Equal(t, "ci", page.Consumers[0].Name)
	require.Equal(t, 2, page.Consumers[0].SecretCount)
}

func TestMCPListConsumersNarrowsByNameAndPages(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	for _, name := range []string{"alpha", "alphabet", "beta"} {
		require.False(t, callTool(t, mux, token, "add_consumer", map[string]any{"name": name}).IsError)
	}

	result := callTool(t, mux, token, "list_consumers", map[string]any{"query": "ALPHA", "page_size": 1})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var page mcpConsumersPage
	require.NoError(t, json.Unmarshal(result.StructuredContent, &page))
	require.Equal(t, 2, page.Total, "total counts every match, not just the page")
	require.Len(t, page.Consumers, 1)
}

func TestMCPListConsumersRefusesAnInvalidPageSize(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	result := callTool(t, mux, issueToken(t, s), "list_consumers", map[string]any{"page_size": 0})
	require.True(t, result.IsError)
}

func TestMCPAddConsumerRegistersANameWithNoSecrets(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	result := callTool(t, mux, token, "add_consumer", map[string]any{"name": "homelab/new-host"})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.JSONEq(t, `{"name":"homelab/new-host","secret_count":0}`, string(result.StructuredContent))

	names, err := s.ListConsumers(context.Background())
	require.NoError(t, err)
	require.Contains(t, names, "homelab/new-host")
}

func TestMCPAddConsumerRefusesADuplicateAndAnEmptyName(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.False(t, callTool(t, mux, token, "add_consumer", map[string]any{"name": "dup"}).IsError)

	dup := callTool(t, mux, token, "add_consumer", map[string]any{"name": "dup"})
	require.True(t, dup.IsError)
	require.Contains(t, dup.Content[0].Text, "already exists")

	empty := callTool(t, mux, token, "add_consumer", map[string]any{"name": ""})
	require.True(t, empty.IsError)
	require.Contains(t, empty.Content[0].Text, "required")
}

func TestMCPListConsumersReturnsTwentyToAPageUnlessAskedOtherwise(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	for i := range 21 {
		require.NoError(t, s.AddConsumer(context.Background(), fmt.Sprintf("consumer-%02d", i)))
	}

	result := callTool(t, mux, token, "list_consumers", map[string]any{})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var page mcpConsumersPage
	require.NoError(t, json.Unmarshal(result.StructuredContent, &page))
	require.Len(t, page.Consumers, 20)
	require.Equal(t, 21, page.Total)
}

func TestMCPListConsumersRefusesPageZeroAndAQueryOverTheLimit(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	require.True(t, callTool(t, mux, token, "list_consumers", map[string]any{"page": 0}).IsError)
	require.True(t, callTool(t, mux, token, "list_consumers", map[string]any{"query": strings.Repeat("a", 129)}).IsError)
}

func TestMCPAddConsumerRefusesANameOverTheLimit(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	result := callTool(t, mux, issueToken(t, s), "add_consumer", map[string]any{"name": strings.Repeat("a", 129)})
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "128")
}

const agePublicKey = "age1xl8amwv0vaxfk3gkrsgfv8l9nwkfuyqf448jwauhkn4vsw9jfqzq5290c0"

func TestMCPUpdateConsumerRenamesAndRegistersAKey(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "mcp_uc_target", []byte("v"), []string{"old-name"}, "", ""))

	result := callTool(t, mux, token, "update_consumer", map[string]any{
		"name": "old-name", "new_name": "new-name", "public_key": agePublicKey,
	})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var out struct {
		Name        string `json:"name"`
		SecretCount int    `json:"secret_count"`
		PublicKey   string `json:"public_key"`
	}
	require.NoError(t, json.Unmarshal(result.StructuredContent, &out))
	require.Equal(t, "new-name", out.Name)
	require.Equal(t, 1, out.SecretCount)
	require.Equal(t, agePublicKey, out.PublicKey)

	obj, err := s.GetObject(context.Background(), "mcp_uc_target")
	require.NoError(t, err)
	require.Equal(t, []string{"new-name"}, obj.UsedBy)
}

func TestMCPUpdateConsumerRefusesNothingToChangeABadKeyAndAnUnknownName(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.AddConsumer(context.Background(), "known"))

	for name, tc := range map[string]struct {
		args map[string]any
		want string
	}{
		"nothing to change": {map[string]any{"name": "known"}, "required"},
		"bad key":           {map[string]any{"name": "known", "public_key": "not-a-key"}, "age"},
		"unknown consumer":  {map[string]any{"name": "ghost", "new_name": "other"}, "unknown consumer"},
	} {
		result := callTool(t, mux, token, "update_consumer", tc.args)
		require.True(t, result.IsError, name)
		require.Contains(t, result.Content[0].Text, tc.want, name)
	}
}

func TestMCPDeleteConsumerStripsItFromEverySecretButKeepsThem(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "mcp_dc_target", []byte("v"), []string{"gone", "stays"}, "", ""))

	result := callTool(t, mux, token, "delete_consumer", map[string]any{"name": "gone"})
	require.False(t, result.IsError, "content: %+v", result.Content)
	require.JSONEq(t, `{"deleted":true}`, string(result.StructuredContent))

	obj, err := s.GetObject(context.Background(), "mcp_dc_target")
	require.NoError(t, err)
	require.Equal(t, []string{"stays"}, obj.UsedBy)
}

func TestMCPDeleteConsumerOfAnUnknownNameIsAToolError(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	result := callTool(t, mux, issueToken(t, s), "delete_consumer", map[string]any{"name": "ghost"})
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "unknown consumer")
}

func TestMCPConsumerToolsRefuseANameOverTheLimit(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	long := strings.Repeat("a", 129)

	require.True(t, callTool(t, mux, token, "update_consumer", map[string]any{"name": long, "new_name": "x"}).IsError)
	require.True(t, callTool(t, mux, token, "update_consumer", map[string]any{"name": "x", "new_name": long}).IsError)
	require.True(t, callTool(t, mux, token, "delete_consumer", map[string]any{"name": long}).IsError)
}

func TestMCPUpdateConsumerMergesIntoAnExistingNameAndKeepsTheSecrets(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "mcp_merge_a", []byte("v"), []string{"from"}, "", ""))
	require.NoError(t, s.CreateObject(context.Background(), "mcp_merge_b", []byte("v"), []string{"into"}, "", ""))

	result := callTool(t, mux, token, "update_consumer", map[string]any{"name": "from", "new_name": "into"})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var out struct {
		Name        string `json:"name"`
		SecretCount int    `json:"secret_count"`
	}
	require.NoError(t, json.Unmarshal(result.StructuredContent, &out))
	require.Equal(t, "into", out.Name)
	require.Equal(t, 2, out.SecretCount, "both secrets now name the merged consumer")
}

func TestMCPUpdateConsumerRefusesARenameThatWouldMergeTwoVariantsOfOneName(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	// d is the only consumer of one variant of release_token and a, b, c the
	// consumers of the other: merging d into a would give both variants a.
	result := callTool(t, f.mux, f.write, "update_consumer", map[string]any{"name": "d", "new_name": "a"})
	require.True(t, result.IsError)

	obj, err := f.s.GetObject(context.Background(), "release_token", store.WithID(f.idD))
	require.NoError(t, err)
	require.Equal(t, []string{"d"}, obj.UsedBy, "a refused rename changes nothing")
}

func TestMCPUpdateConsumerWithABadKeyAppliesNoRename(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	require.NoError(t, s.CreateObject(context.Background(), "mcp_badkey", []byte("v"), []string{"keep-me"}, "", ""))

	result := callTool(t, mux, token, "update_consumer", map[string]any{"name": "keep-me", "new_name": "renamed", "public_key": "not-a-key"})
	require.True(t, result.IsError)

	obj, err := s.GetObject(context.Background(), "mcp_badkey")
	require.NoError(t, err)
	require.Equal(t, []string{"keep-me"}, obj.UsedBy)
}

func TestMCPConsumerToolsAcceptANameAtTheLimitAndRefuseAKeyOverIt(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	atLimit := strings.Repeat("a", 128)

	require.False(t, callTool(t, mux, token, "add_consumer", map[string]any{"name": atLimit}).IsError)
	require.False(t, callTool(t, mux, token, "update_consumer", map[string]any{"name": atLimit, "new_name": strings.Repeat("b", 128)}).IsError)
	require.True(t, callTool(t, mux, token, "update_consumer", map[string]any{"name": "x", "public_key": strings.Repeat("a", 513)}).IsError)
}
