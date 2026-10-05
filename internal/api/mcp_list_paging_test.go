package api_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// The MCP list tool pages like the HTTP lists do (alrayyes/hush-hush#685,
// ADR 34): at most one page, 50 by default, and it says how many there are in
// all. The structured result stays an array, so a client that reads it keeps
// working; the total is in the result's text.

// seedObjects stores n objects and returns a function that calls the list tool.
func seedObjects(t *testing.T, n int) func(args map[string]any) mcpCallToolResult {
	t.Helper()

	m, s := newTestMux(t)
	tok := issueToken(t, s)

	for i := range n {
		seedObject(t, s, fmt.Sprintf("mcp_page_%03d", i))
	}

	return func(args map[string]any) mcpCallToolResult {
		return callTool(t, m, tok, "list", args)
	}
}

func listedSlugs(t *testing.T, r mcpCallToolResult) []string {
	t.Helper()

	var out []struct {
		Slug string `json:"slug"`
	}
	require.NoError(t, json.Unmarshal(r.StructuredContent, &out))

	slugs := make([]string, len(out))
	for i, o := range out {
		slugs[i] = o.Slug
	}

	return slugs
}

func resultText(r mcpCallToolResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		b.WriteString(c.Text)
	}

	return b.String()
}

func TestMCPListWithNoPagingIsTheFirstPageAndSaysTheTotal(t *testing.T) {
	t.Parallel()

	call := seedObjects(t, 60)
	r := call(map[string]any{})

	require.False(t, r.IsError, "content: %+v", r.Content)
	require.Len(t, listedSlugs(t, r), hushhush.DefaultListPageSize)
	require.Contains(t, resultText(r), "of 60", "the text says there are more than this page")
}

func TestMCPListTakesALimitAndAnOffset(t *testing.T) {
	t.Parallel()

	call := seedObjects(t, 60)

	r := call(map[string]any{"limit": 10, "offset": 55})
	require.False(t, r.IsError, "content: %+v", r.Content)
	require.Equal(t, []string{"mcp_page_055", "mcp_page_056", "mcp_page_057", "mcp_page_058", "mcp_page_059"}, listedSlugs(t, r))
	require.Contains(t, resultText(r), "of 60")
}

func TestMCPListOutsideTheLimitsIsAToolErrorNamingThem(t *testing.T) {
	t.Parallel()

	call := seedObjects(t, 1)

	for _, args := range []map[string]any{{"limit": 0}, {"limit": 501}, {"offset": -1}} {
		r := call(args)
		require.True(t, r.IsError, "%v should be refused", args)
		require.Regexp(t, `limit|offset`, resultText(r))
	}
}

func TestMCPListToolStatesItsLimits(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	resp := mcpCall(t, mux, issueToken(t, s), mcpToolsListBody)

	var tools struct {
		Tools []struct {
			Name        string `json:"name"`
			InputSchema struct {
				Properties map[string]struct {
					Description string `json:"description"`
				} `json:"properties"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &tools))

	for _, tool := range tools.Tools {
		if tool.Name != "list" {
			continue
		}

		require.Contains(t, tool.InputSchema.Properties["limit"].Description, "1 to 500")
		require.Contains(t, tool.InputSchema.Properties["limit"].Description, "50")
		require.Contains(t, tool.InputSchema.Properties["offset"].Description, "skip")

		return
	}

	t.Fatal("no list tool")
}
