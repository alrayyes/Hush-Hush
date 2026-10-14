package api

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addMCPAuditLogTool registers the tool over the audit log, which GET
// /audit-log serves over HTTP.
func addMCPAuditLogTool(srv *mcp.Server, s objectStore) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "audit_log",
		Description: "Query the audit log: every create, read, update and delete of a secret, with when, who and from where. Filters combine with AND. One page of 50, oldest first, unless limit and order say otherwise; pass after (or before, newest first) with the last id you got to read the next page.",
	}, mcpAuditLog(s))
}

// mcpAuditLogInput is the "audit_log" tool's input, mirroring GET /audit-log's
// query parameters. The cursors and limit are pointers so one that is left
// out is told apart from a 0, which is refused.
type mcpAuditLogInput struct {
	ObjectID string `json:"object_id,omitempty" maxLength:"128" jsonschema:"only entries for this object's slug or id"`
	Caller   string `json:"caller,omitempty" maxLength:"128" jsonschema:"only entries recorded with this self-reported caller"`
	Actor    string `json:"actor,omitempty" maxLength:"128" jsonschema:"only entries by this verified actor (a token id), or none for unauthenticated reads"`
	From     string `json:"from,omitempty" maxLength:"64" jsonschema:"only entries at or after this RFC 3339 time"`
	To       string `json:"to,omitempty" maxLength:"64" jsonschema:"only entries at or before this RFC 3339 time"`
	After    *int   `json:"after,omitempty" jsonschema:"only entries with an id above this, for paging forward"`
	Before   *int   `json:"before,omitempty" jsonschema:"only entries with an id below this, for paging back"`
	Order    string `json:"order,omitempty" jsonschema:"asc (oldest first, the default) or desc"`
	Limit    *int   `json:"limit,omitempty" jsonschema:"how many entries to return, 1 to 500; 50 when left out"`
}

// query is the input as the query string GET /audit-log reads, so the tool
// and the route parse and bound the filter in one place.
func (in mcpAuditLogInput) query() url.Values {
	q := url.Values{}

	for name, value := range map[string]string{
		"object_id": in.ObjectID, "caller": in.Caller, "actor": in.Actor,
		"from": in.From, "to": in.To, "order": in.Order,
		"after": optionalInt(in.After), "before": optionalInt(in.Before), "limit": optionalInt(in.Limit),
	} {
		if value != "" {
			q.Set(name, value)
		}
	}

	return q
}

// mcpAuditLog is the "audit_log" tool's handler - the same query
// handleQueryAuditLog (audit_log.go) performs over HTTP. Reading the log
// writes no entry, matching the route.
func mcpAuditLog(s objectStore) mcp.ToolHandlerFor[mcpAuditLogInput, []AuditLogEntry] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpAuditLogInput) (*mcp.CallToolResult, []AuditLogEntry, error) {
		if err := checkLimits(in); err != nil {
			return nil, nil, err
		}

		filter, err := auditLogFilterFrom(in.query())
		if err != nil {
			return nil, nil, err
		}

		rows, err := s.QueryAuditLog(ctx, filter)
		if err != nil {
			return nil, nil, mcpInternalError(ctx, "audit_log", err)
		}

		return nil, auditLogEntriesFrom(rows), nil
	}
}
