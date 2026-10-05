package api

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Sentinels rather than errors constructed inline (go-lint.md's err113) -
// each is a tool-level error a caller sees verbatim, so the message is
// also the whole point of the value.
var (
	errMCPInternal             = errors.New("internal error")
	errMCPSlugAndValueRequired = errors.New("slug and value are required")
	errMCPValueRequired        = errors.New("value is required")
	errMCPValueNotBase64       = errors.New("value must be base64-encoded")
	errMCPObjectAlreadyExists  = errors.New("object already exists")
	errMCPUnknownObject        = errors.New("unknown object")
)

// checkMCPInput holds a tool's input to its field limits, then turns its
// base64 text into the sealed value, refusing text that isn't base64 and a
// value over MaxValueBytes.
func checkMCPInput(in any, text string) ([]byte, error) {
	if err := checkLimits(in); err != nil {
		return nil, err
	}

	value, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, errMCPValueNotBase64
	}

	if len(value) > MaxValueBytes {
		return nil, errValueTooLarge
	}

	return value, nil
}

// mcpInjectInput is the "inject" tool's input - CreateObjectRequest's
// fields, except Value is base64 text rather than []byte.
//
// CreateObjectRequest/UpdateObjectRequest aren't reused directly here the
// way ObjectMetadata is for tool output: the MCP SDK infers a tool's JSON
// Schema straight from the Go type (google/jsonschema-go), which has no
// special case for []byte the way encoding/json's own marshaling does. A
// []byte field infers as a JSON array of integers, not the base64 string
// it actually decodes from on the wire, and the SDK validates a call's
// arguments against that inferred schema before this handler ever sees
// them - a real base64 argument fails that validation outright. A plain
// string field, decoded by hand, sidesteps the mismatch.
type mcpInjectInput struct {
	Slug        string   `json:"slug" maxLength:"128" jsonschema:"the object's new caller-facing slug"`
	Value       string   `json:"value" jsonschema:"the sealed (age) ciphertext, base64-encoded"`
	UsedBy      []string `json:"used_by,omitempty" maxItems:"100" maxLength:"128" jsonschema:"consumers to record against the object"`
	Description string   `json:"description,omitempty" maxLength:"1000" jsonschema:"a human-readable note about the object"`
	Tags        []string `json:"tags,omitempty" jsonschema:"labels for grouping, lowercase a-z 0-9 . _ / -, at most 10"`
}

// mcpGetInput is the "get" tool's input - just the object's slug, the same
// single value api/openapi.yaml's getObject documents.
type mcpGetInput struct {
	Slug string `json:"slug" jsonschema:"the object's slug"`
}

// mcpGetOutput is the "get" tool's output - the sealed ciphertext exactly
// as stored (ADR 0006: one value, never a bundled file), base64-encoded
// for the same reason mcpInjectInput's Value is.
type mcpGetOutput struct {
	Value string `json:"value" jsonschema:"the object's sealed ciphertext, base64-encoded"`
}

// mcpUpdateInput is the "update" tool's input - UpdateObjectRequest's
// fields plus the slug (the HTTP PUT takes it from the URL path instead),
// Value base64-encoded for the same reason mcpInjectInput's is.
type mcpUpdateInput struct {
	Slug   string    `json:"slug" maxLength:"128" jsonschema:"the object's slug"`
	Value  string    `json:"value" jsonschema:"the new sealed ciphertext, base64-encoded"`
	UsedBy *[]string `json:"used_by,omitempty" maxItems:"100" maxLength:"128" jsonschema:"replaces the object's recorded consumers; omit to leave them unchanged"`
	Tags   *[]string `json:"tags,omitempty" jsonschema:"replaces the object's tags; an empty list clears them; omit to leave them unchanged"`
}

// mcpDeleteInput is the "delete" tool's input - just the object's slug.
type mcpDeleteInput struct {
	Slug string `json:"slug" jsonschema:"the object's slug"`
}

// mcpDeleteOutput confirms a delete happened, since the HTTP DELETE's 204
// has nothing for a tool call to echo back.
type mcpDeleteOutput struct {
	Deleted bool `json:"deleted"`
}

// mcpListInput is the "list" tool's input, mirroring GET /objects's
// used_by query parameter.
type mcpListInput struct {
	UsedBy string `json:"used_by,omitempty" jsonschema:"restrict to objects whose recorded used_by lineage includes this consumer"`
	Tag    string `json:"tag,omitempty" jsonschema:"restrict to objects carrying this tag"`
}

// handleMCP serves the MCP endpoint (alrayyes/hush-hush#346) - inject/get/
// update/delete/list tools mirroring hush-hush-cli's own operations, over
// the same bearer-token-or-session gate requireWriteAccess already applies
// to every other write route (docs/adr/0019-mcp-endpoint.md). Gating every
// tool uniformly, including get and list, is stricter than those
// operations' plain HTTP auth - get is unauthenticated by ADR 0002 - but
// the ticket's own framing is one authenticated MCP session doing all five
// operations, not per-tool auth rules.
//
// A fresh *mcp.Server is built per HTTP request (Stateless mode - no
// long-lived MCP session to track for a simple CRUD tool call), with each
// tool handler closing over the actor/caller/source-IP already computed
// from r, exactly as every HTTP handler in this package already does via
// actorFrom/callerFrom/sourceIPFrom - not relying on the SDK threading
// context values into a tool handler's own ctx.
func handleMCP(s objectStore, version string) http.HandlerFunc {
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return newMCPServer(s, version, r)
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
		// Every tool here is a fast, synchronous CRUD call with nothing to
		// stream - a plain JSON response keeps the wire format the single
		// shape api/openapi.yaml documents, rather than an SSE event the
		// client would have to unwrap for the same one message.
		JSONResponse: true,
	})

	return handler.ServeHTTP
}

// newMCPServer builds the *mcp.Server for one HTTP request, registering
// the five tools with handlers bound to that request's already-verified
// actor - requireWriteAccess has already run by the time this is called,
// so r carries the same context values actorFrom/callerFrom read for
// every other handler.
func newMCPServer(s objectStore, version string, r *http.Request) *mcp.Server {
	actorType, actorID := actorFrom(r)
	caller := callerFrom(r)
	sourceIP := sourceIPFrom(r)

	srv := mcp.NewServer(&mcp.Implementation{Name: "hush-hush", Version: version}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "inject",
		Description: "Store a new sealed secret object under a slug. value must already be sealed (age) ciphertext - this server never decrypts it.",
	}, mcpInject(s, caller, sourceIP, actorType, actorID))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get",
		Description: "Fetch an object's sealed ciphertext exactly as stored - a single value, never a bundled file (ADR 0006).",
	}, mcpGet(s, caller, sourceIP, actorType, actorID))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update",
		Description: "Replace an object's sealed value. slug and description stay fixed; used_by is left unchanged unless given.",
	}, mcpUpdate(s, caller, sourceIP, actorType, actorID))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete",
		Description: "Permanently remove an object. A subsequent get for the same slug fails.",
	}, mcpDelete(s, caller, sourceIP, actorType, actorID))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list",
		Description: "List every stored object's metadata (slug, used_by, description) - never the sealed value.",
	}, mcpList(s))

	return srv
}

// mcpInternalError logs err the same way writeInternalError does for an
// HTTP handler, and returns the same generic message a caller sees - no
// internal detail leaks into a tool result an agent might act on or relay.
func mcpInternalError(ctx context.Context, tool string, err error) error {
	slog.ErrorContext(ctx, "internal error", "tool", tool, "error", err)

	return errMCPInternal
}

// mcpInject is the "inject" tool's handler - the same create operation
// handleCreateObject (create.go) performs over HTTP.
func mcpInject(s objectStore, caller, sourceIP, actorType, actorID string) mcp.ToolHandlerFor[mcpInjectInput, ObjectMetadata] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpInjectInput) (*mcp.CallToolResult, ObjectMetadata, error) {
		if in.Slug == "" || in.Value == "" {
			return nil, ObjectMetadata{}, errMCPSlugAndValueRequired
		}

		value, err := checkMCPInput(in, in.Value)
		if err != nil {
			return nil, ObjectMetadata{}, err
		}

		if err := validateAgeCiphertext(value); err != nil {
			return nil, ObjectMetadata{}, err
		}

		tags, err := createTags(in.Tags)
		if err != nil {
			return nil, ObjectMetadata{}, err
		}

		ownerID, err := s.CurrentUserID(ctx)
		if err != nil {
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "inject", err)
		}

		in.UsedBy = uniqueConsumers(in.UsedBy)

		err = s.CreateObject(ctx, in.Slug, value, in.UsedBy, in.Description, ownerID, store.WithTags(tags))
		switch {
		case err == nil:
		case errors.Is(err, store.ErrAlreadyExists):
			return nil, ObjectMetadata{}, errMCPObjectAlreadyExists
		default:
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "inject", err)
		}

		if err := s.RecordAuditLog(ctx, in.Slug, store.AuditActionCreate, caller, sourceIP, actorType, actorID); err != nil {
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "inject", err)
		}

		return nil, ObjectMetadata{Slug: in.Slug, UsedBy: in.UsedBy, Tags: tags, Description: in.Description}, nil
	}
}

// mcpGet is the "get" tool's handler - the same fetch handleGetObject
// (get.go) performs over HTTP, except authenticated (this endpoint's
// uniform gating, docs/adr/0019-mcp-endpoint.md), so the resulting audit
// entry carries a verified actor instead of the empty one an unauthenticated
// HTTP GET records.
func mcpGet(s objectStore, caller, sourceIP, actorType, actorID string) mcp.ToolHandlerFor[mcpGetInput, mcpGetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpGetInput) (*mcp.CallToolResult, mcpGetOutput, error) {
		obj, err := s.GetObject(ctx, in.Slug)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrNotFound):
			return nil, mcpGetOutput{}, errMCPUnknownObject
		default:
			return nil, mcpGetOutput{}, mcpInternalError(ctx, "get", err)
		}

		if err := s.RecordAuditLog(ctx, in.Slug, store.AuditActionRead, caller, sourceIP, actorType, actorID); err != nil {
			return nil, mcpGetOutput{}, mcpInternalError(ctx, "get", err)
		}

		return nil, mcpGetOutput{Value: base64.StdEncoding.EncodeToString(obj.Value)}, nil
	}
}

// mcpUpdate is the "update" tool's handler - the same rotate
// handleUpdateObject (update.go) performs over HTTP.
func mcpUpdate(s objectStore, caller, sourceIP, actorType, actorID string) mcp.ToolHandlerFor[mcpUpdateInput, ObjectMetadata] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpUpdateInput) (*mcp.CallToolResult, ObjectMetadata, error) {
		if in.Value == "" {
			return nil, ObjectMetadata{}, errMCPValueRequired
		}

		value, err := checkMCPInput(in, in.Value)
		if err != nil {
			return nil, ObjectMetadata{}, err
		}

		if err := validateAgeCiphertext(value); err != nil {
			return nil, ObjectMetadata{}, err
		}

		opts, err := updateTagOptions(in.Tags)
		if err != nil {
			return nil, ObjectMetadata{}, err
		}

		err = s.UpdateObject(ctx, in.Slug, value, uniqueConsumersPtr(in.UsedBy), opts...)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrNotFound):
			return nil, ObjectMetadata{}, errMCPUnknownObject
		default:
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "update", err)
		}

		obj, err := s.GetObject(ctx, in.Slug)
		if err != nil {
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "update", err)
		}

		if err := s.RecordAuditLog(ctx, in.Slug, store.AuditActionUpdate, caller, sourceIP, actorType, actorID); err != nil {
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "update", err)
		}

		return nil, ObjectMetadata{Slug: obj.Slug, UsedBy: obj.UsedBy, Tags: tagsOrEmpty(obj.Tags), Description: obj.Description}, nil
	}
}

// mcpDelete is the "delete" tool's handler - the same removal
// handleDeleteObject (delete.go) performs over HTTP.
func mcpDelete(s objectStore, caller, sourceIP, actorType, actorID string) mcp.ToolHandlerFor[mcpDeleteInput, mcpDeleteOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpDeleteInput) (*mcp.CallToolResult, mcpDeleteOutput, error) {
		err := s.DeleteObject(ctx, in.Slug)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrNotFound):
			return nil, mcpDeleteOutput{}, errMCPUnknownObject
		default:
			return nil, mcpDeleteOutput{}, mcpInternalError(ctx, "delete", err)
		}

		if err := s.RecordAuditLog(ctx, in.Slug, store.AuditActionDelete, caller, sourceIP, actorType, actorID); err != nil {
			return nil, mcpDeleteOutput{}, mcpInternalError(ctx, "delete", err)
		}

		return nil, mcpDeleteOutput{Deleted: true}, nil
	}
}

// mcpList is the "list" tool's handler - the same enumeration
// handleListObjects (list.go) performs over HTTP. No audit entry, matching
// handleListObjects: listing itself isn't logged, only reads/writes of a
// specific object are.
func mcpList(s objectStore) mcp.ToolHandlerFor[mcpListInput, []ObjectMetadata] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpListInput) (*mcp.CallToolResult, []ObjectMetadata, error) {
		filter := store.ObjectFilter{UsedBy: in.UsedBy}

		if in.Tag != "" {
			tag, err := normaliseTag(in.Tag)
			if err != nil {
				return nil, nil, err
			}

			filter.Tags = []string{tag}
		}

		objs, err := s.ListObjects(ctx, filter)
		if err != nil {
			return nil, nil, mcpInternalError(ctx, "list", err)
		}

		metadata := make([]ObjectMetadata, len(objs))
		for i, obj := range objs {
			metadata[i] = ObjectMetadata{Slug: obj.Slug, UsedBy: obj.UsedBy, Tags: tagsOrEmpty(obj.Tags), Description: obj.Description}
		}

		return nil, metadata, nil
	}
}
