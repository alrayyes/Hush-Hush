package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// mcpRequest builds a raw JSON-RPC 2.0 request against POST /mcp - the MCP
// endpoint speaks its own protocol, not one of this package's own request
// types, so these are hand-built envelopes rather than marshaled structs.
func mcpRequest(t *testing.T, body string, token string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

// mcpToolsListBody is a minimal MCP initialize + tools/list exchange -
// every case below needs at least an authenticated tools/list to prove the
// endpoint is up and gated correctly.
const mcpToolsListBody = `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`

func TestMCPWithoutBearerTokenOrSessionIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := mcpRequest(t, mcpToolsListBody, "")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMCPWithWrongBearerTokenIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := mcpRequest(t, mcpToolsListBody, "wrong-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// mcpJSONRPCResponse is enough of the JSON-RPC 2.0 envelope shape for these
// tests to read a result or an error out of it, without depending on the
// SDK's own (unexported) message types.
type mcpJSONRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// mcpCall issues one JSON-RPC call against the real mux with a valid
// bearer token and decodes its single JSON response line - the endpoint is
// stateless, so every call is its own independent HTTP round trip.
func mcpCall(t *testing.T, mux http.Handler, token, body string) mcpJSONRPCResponse {
	t.Helper()

	req := mcpRequest(t, body, token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var resp mcpJSONRPCResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	return resp
}

func TestMCPToolsListReturnsTheFiveTools(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	resp := mcpCall(t, mux, issueToken(t, s), mcpToolsListBody)

	require.Nil(t, resp.Error)

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &result))

	names := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		names[i] = tool.Name
	}

	require.ElementsMatch(t, []string{"inject", "get", "update", "delete", "list"}, names)
}

// mcpCallToolBody builds a tools/call JSON-RPC request for the given tool
// and argument map.
func mcpCallToolBody(t *testing.T, tool string, args map[string]any) string {
	t.Helper()

	params := map[string]any{"name": tool, "arguments": args}
	b, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params,
	})
	require.NoError(t, err)

	return string(b)
}

// mcpCallToolResult is the CallToolResult shape these tests care about -
// structuredContent for a successful call's typed output, isError/content
// for a tool-level failure.
type mcpCallToolResult struct {
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
	Content           []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func callTool(t *testing.T, mux http.Handler, token, tool string, args map[string]any) mcpCallToolResult {
	t.Helper()

	resp := mcpCall(t, mux, token, mcpCallToolBody(t, tool, args))
	require.Nil(t, resp.Error, "unexpected protocol-level error")

	var result mcpCallToolResult
	require.NoError(t, json.Unmarshal(resp.Result, &result))

	return result
}

func TestMCPInjectCreatesAnObjectAndRecordsAnAuditEntry(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	result := callTool(t, mux, token, "inject", map[string]any{
		"id":      "mcp_inject_target",
		"value":   []byte("sealed-ciphertext"),
		"used_by": []string{"homelab/vps-docker"},
	})
	require.False(t, result.IsError, "content: %+v", result.Content)

	obj, err := s.GetObject(context.Background(), "mcp_inject_target")
	require.NoError(t, err)
	require.Equal(t, []byte("sealed-ciphertext"), obj.Value)

	entries, err := s.QueryAuditLog(context.Background(), store.AuditLogFilter{ObjectID: "mcp_inject_target"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, store.AuditActionCreate, entries[0].Action)
	require.Equal(t, "token", entries[0].ActorType)
}

func TestMCPInjectDuplicateIDIsAToolError(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	args := map[string]any{"id": "mcp_dup", "value": []byte("v1")}
	first := callTool(t, mux, token, "inject", args)
	require.False(t, first.IsError)

	second := callTool(t, mux, token, "inject", map[string]any{"id": "mcp_dup", "value": []byte("v2")})
	require.True(t, second.IsError)
}

func TestMCPGetReturnsTheSealedValueAndRecordsAnAuditEntry(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_get_target")

	result := callTool(t, mux, token, "get", map[string]any{"id": "mcp_get_target"})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var out struct {
		Value []byte `json:"value"`
	}
	require.NoError(t, json.Unmarshal(result.StructuredContent, &out))
	require.Equal(t, []byte("sealed-ciphertext"), out.Value)

	entries, err := s.QueryAuditLog(context.Background(), store.AuditLogFilter{ObjectID: "mcp_get_target"})
	require.NoError(t, err)
	// seedObject's own create call already logs one entry; the get tool
	// call above should be the second.
	require.Len(t, entries, 2)
	require.Equal(t, store.AuditActionRead, entries[1].Action)
	require.Equal(t, "token", entries[1].ActorType, "get is authenticated over MCP even though it isn't over plain HTTP")
}

func TestMCPGetUnknownObjectIsAToolError(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	result := callTool(t, mux, issueToken(t, s), "get", map[string]any{"id": "does-not-exist"})
	require.True(t, result.IsError)
}

func TestMCPUpdateReplacesTheValue(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_update_target")

	result := callTool(t, mux, token, "update", map[string]any{
		"id": "mcp_update_target", "value": []byte("new-sealed-value"),
	})
	require.False(t, result.IsError, "content: %+v", result.Content)

	obj, err := s.GetObject(context.Background(), "mcp_update_target")
	require.NoError(t, err)
	require.Equal(t, []byte("new-sealed-value"), obj.Value)
}

func TestMCPDeleteRemovesTheObject(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_delete_target")

	result := callTool(t, mux, token, "delete", map[string]any{"id": "mcp_delete_target"})
	require.False(t, result.IsError, "content: %+v", result.Content)

	_, err := s.GetObject(context.Background(), "mcp_delete_target")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMCPListReturnsEveryObjectsMetadata(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	seedObject(t, s, "mcp_list_a")
	seedObject(t, s, "mcp_list_b")

	result := callTool(t, mux, token, "list", map[string]any{})
	require.False(t, result.IsError, "content: %+v", result.Content)

	var out []struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(result.StructuredContent, &out))

	ids := make([]string, len(out))
	for i, o := range out {
		ids[i] = o.ID
	}

	require.Subset(t, ids, []string{"mcp_list_a", "mcp_list_b"})
}
