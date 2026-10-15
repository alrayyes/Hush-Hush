package api_test

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// Every operation in api/openapi.yaml is reachable through an MCP tool, or is
// on a written list with a reason (rules/mcp.md). A new endpoint with no tool
// fails here, not in a later audit.
//
// mcpOperationTool maps an operation to the tool that does it.
// mcpExcludedOperations are deliberately not agent-reachable.
// mcpPendingOperations are the gap alrayyes/hush-hush#970 closes one resource
// at a time: a slice moves its operations into mcpOperationTool, and the
// last one deletes this list.
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
}

var mcpExcludedOperations = map[string]string{
	"health":             "liveness probe for the orchestrator, not a task an agent does",
	"ready":              "readiness probe for the orchestrator, not a task an agent does",
	"mcp":                "the MCP transport itself",
	"beginRegistration":  "passkey ceremony: needs the owner's authenticator in a browser",
	"finishRegistration": "passkey ceremony: needs the owner's authenticator in a browser",
	"beginLogin":         "passkey ceremony: needs the owner's authenticator in a browser",
	"finishLogin":        "passkey ceremony: needs the owner's authenticator in a browser",
	"logout":             "needs a browser session and its CSRF token, which a bearer-token MCP call has neither of",
}

var mcpPendingOperations = map[string]string{
	"getAuthStatus":       "#970",
	"getOwnerIdentity":    "#970",
	"listCredentials":     "#970",
	"renameCredential":    "#970",
	"deleteCredential":    "#970",
	"createToken":         "#970",
	"listTokens":          "#970",
	"revokeToken":         "#970",
	"rotateToken":         "#970",
	"purgeToken":          "#970",
	"createConsumerToken": "#970",
	"listConsumerTokens":  "#970",
	"revokeConsumerToken": "#970",
	"rotateConsumerToken": "#970",
	"purgeConsumerToken":  "#970",
}

func specOperationIDs(t *testing.T) []string {
	t.Helper()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	var ids []string
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			ids = append(ids, op.OperationID)
		}
	}
	sort.Strings(ids)

	return ids
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

	var unaccounted []string
	for _, id := range specOperationIDs(t) {
		_, mapped := mcpOperationTool[id]
		_, excluded := mcpExcludedOperations[id]
		_, pending := mcpPendingOperations[id]
		if !mapped && !excluded && !pending {
			unaccounted = append(unaccounted, id)
		}
	}

	require.Empty(t, unaccounted,
		"add an MCP tool for each (mcpOperationTool) or a reason to leave it out (mcpExcludedOperations)")
}

func TestNoListedOperationIsNotInTheSpec(t *testing.T) {
	t.Parallel()

	inSpec := map[string]bool{}
	for _, id := range specOperationIDs(t) {
		inSpec[id] = true
	}

	var stale []string
	for _, list := range []map[string]string{mcpOperationTool, mcpExcludedOperations, mcpPendingOperations} {
		for id := range list {
			if !inSpec[id] {
				stale = append(stale, id)
			}
		}
	}

	require.Empty(t, stale, "removed from the spec, so remove from the parity lists")
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

func TestAnOperationIsOnExactlyOneList(t *testing.T) {
	t.Parallel()

	var onSeveral []string
	for _, id := range specOperationIDs(t) {
		_, mapped := mcpOperationTool[id]
		_, excluded := mcpExcludedOperations[id]
		_, pending := mcpPendingOperations[id]

		count := 0
		for _, on := range []bool{mapped, excluded, pending} {
			if on {
				count++
			}
		}
		if count > 1 {
			onSeveral = append(onSeveral, id)
		}
	}

	require.Empty(t, onSeveral, "drop it from mcpPendingOperations once it has a tool or a reason; an operation is mapped or excluded, not both")
}
