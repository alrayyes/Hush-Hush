package api_test

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// Every operation in api/openapi.yaml is reachable through an MCP tool, or the
// spec says why it is not (rules/mcp.md). A new endpoint with no tool fails
// here, not in a later audit.
//
// mcpOperationTool maps an operation to the tool that does it. An operation
// an agent should not reach carries x-mcp-excluded in the spec, with the
// reason as its value: the spec is where a reader of the API looks, so the
// exclusion lives beside the operation instead of in a second list.
//
// This checks names, not behaviour: it can't tell that two tools were swapped
// between their operations. What each tool does is the tool tests' job.

var mcpOperationTool = map[string]string{
	"listObjects":                "list",
	"createObject":               "inject",
	"getObject":                  "get",
	"updateObject":               "update",
	"patchObject":                "edit",
	"deleteObject":               "delete",
	"getObjectUsedBy":            "used_by",
	"listConsumers":              "list_consumers",
	"addConsumer":                "add_consumer",
	"updateConsumer":             "update_consumer",
	"deleteConsumer":             "delete_consumer",
	"queryAuditLog":              "audit_log",
	"queryAuditLogFilterOptions": "audit_log_filter_options",
	"getOwnerIdentity":           "owner_identity",
	"createConsumerToken":        "create_consumer_token",
	"listConsumerTokens":         "list_consumer_tokens",
	"revokeConsumerToken":        "revoke_consumer_token",
	"rotateConsumerToken":        "rotate_consumer_token",
	"purgeConsumerToken":         "purge_consumer_token",
}

// specOperations is every operation ID in the spec, with the reason it is
// left out of MCP where the spec gives one.
func specOperations(t *testing.T) (ids []string, excluded map[string]string) {
	t.Helper()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	excluded = map[string]string{}
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			ids = append(ids, op.OperationID)

			if reason, ok := op.Extensions["x-mcp-excluded"]; ok {
				text, isText := reason.(string)
				require.True(t, isText && text != "", "%s: x-mcp-excluded needs a reason", op.OperationID)
				excluded[op.OperationID] = text
			}
		}
	}
	sort.Strings(ids)

	return ids, excluded
}

func listedMCPTools(t *testing.T) []string {
	t.Helper()

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

	return names
}

func TestEveryOperationHasAnMCPToolOrAWrittenReason(t *testing.T) {
	t.Parallel()

	ids, excluded := specOperations(t)

	var unaccounted []string
	for _, id := range ids {
		_, mapped := mcpOperationTool[id]
		_, left := excluded[id]
		if !mapped && !left {
			unaccounted = append(unaccounted, id)
		}
	}

	require.Empty(t, unaccounted,
		"add an MCP tool for each (mcpOperationTool) or x-mcp-excluded with a reason in the spec")
}

func TestAnOperationIsMappedOrExcludedNotBoth(t *testing.T) {
	t.Parallel()

	_, excluded := specOperations(t)

	var both []string
	for id := range excluded {
		if _, mapped := mcpOperationTool[id]; mapped {
			both = append(both, id)
		}
	}

	require.Empty(t, both, "it has a tool, so drop x-mcp-excluded; or it is excluded, so drop the tool")
}

func TestNoMappedOperationIsMissingFromTheSpec(t *testing.T) {
	t.Parallel()

	ids, _ := specOperations(t)
	inSpec := map[string]bool{}
	for _, id := range ids {
		inSpec[id] = true
	}

	var stale []string
	for id := range mcpOperationTool {
		if !inSpec[id] {
			stale = append(stale, id)
		}
	}

	require.Empty(t, stale, "removed from the spec, so remove from mcpOperationTool")
}

func TestEveryMappedOperationNamesARegisteredTool(t *testing.T) {
	t.Parallel()

	registered := map[string]bool{}
	for _, name := range listedMCPTools(t) {
		registered[name] = true
	}

	var missing []string
	for id, tool := range mcpOperationTool {
		if !registered[tool] {
			missing = append(missing, id+" -> "+tool)
		}
	}

	require.Empty(t, missing)
}

func TestEveryRegisteredToolIsMappedFromAnOperation(t *testing.T) {
	t.Parallel()

	mapped := map[string]bool{}
	for _, tool := range mcpOperationTool {
		mapped[tool] = true
	}

	var orphans []string
	for _, name := range listedMCPTools(t) {
		if !mapped[name] {
			orphans = append(orphans, name)
		}
	}

	require.Empty(t, orphans, "a tool that does no listed operation has no place in the parity map")
}
