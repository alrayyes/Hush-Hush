package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

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

// mcpSelector holds a tool's input to its field limits and turns its optional
// id into the store option that picks a variant.
func mcpSelector(in any, id string) ([]store.ObjectOption, error) {
	if err := checkLimits(in); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, nil
	}

	if !objectIDPattern.MatchString(id) {
		return nil, errInvalidObjectID
	}

	return []store.ObjectOption{store.WithID(id)}, nil
}

// mcpLookupError is what a tool answers when a lookup or write by name fails:
// the same three outcomes the HTTP handlers map to 404 and 409, or an internal
// error it logs.
func mcpLookupError(ctx context.Context, tool string, err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return errMCPUnknownObject
	case errors.Is(err, store.ErrAmbiguousSlug):
		return errAmbiguousName
	case errors.Is(err, store.ErrVariantConflict):
		return errVariantConflict
	default:
		return mcpInternalError(ctx, tool, err)
	}
}

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
	ID   string `json:"id,omitempty" maxLength:"36" jsonschema:"the object's UUID, to pick one variant when the slug has several"`
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
	ID     string    `json:"id,omitempty" maxLength:"36" jsonschema:"the object's UUID, to pick one variant when the slug has several"`
	Value  string    `json:"value" jsonschema:"the new sealed ciphertext, base64-encoded"`
	UsedBy *[]string `json:"used_by,omitempty" maxItems:"100" maxLength:"128" jsonschema:"replaces the object's recorded consumers; omit to leave them unchanged"`
	Tags   *[]string `json:"tags,omitempty" jsonschema:"replaces the object's tags; an empty list clears them; omit to leave them unchanged"`
}

// mcpDeleteInput is the "delete" tool's input - just the object's slug.
type mcpDeleteInput struct {
	Slug string `json:"slug" jsonschema:"the object's slug"`
	ID   string `json:"id,omitempty" maxLength:"36" jsonschema:"the object's UUID, to pick one variant when the slug has several"`
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
	// Limit and Offset page the result like GET /objects does. They're pointers
	// so a client that sends neither gets the first page, and one that sends 0
	// is refused rather than read as absent.
	Limit  *int `json:"limit,omitempty" jsonschema:"how many objects to return, 1 to 500; 50 when left out"`
	Offset *int `json:"offset,omitempty" jsonschema:"how many objects to skip; add the number you got to read the next page"`
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
		Description: "List stored objects' metadata (id, slug, used_by, description) - never the sealed value. One page of 50 unless limit says otherwise; the result's text says how many there are in all.",
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

		var id string

		err = s.CreateObject(ctx, in.Slug, value, in.UsedBy, in.Description, ownerID, store.WithTags(tags), store.IntoID(&id))
		switch {
		case err == nil:
		case errors.Is(err, store.ErrAlreadyExists):
			return nil, ObjectMetadata{}, errMCPObjectAlreadyExists
		default:
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "inject", err)
		}

		if err := s.RecordAuditLog(ctx, in.Slug, store.AuditActionCreate, caller, sourceIP, actorType, actorID, store.AuditVariant(id)); err != nil {
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "inject", err)
		}

		return nil, ObjectMetadata{ID: id, Slug: in.Slug, UsedBy: in.UsedBy, Tags: tags, Description: in.Description}, nil
	}
}

// mcpGet is the "get" tool's handler - the same fetch handleGetObject
// (get.go) performs over HTTP, except authenticated (this endpoint's
// uniform gating, docs/adr/0019-mcp-endpoint.md), so the resulting audit
// entry carries a verified actor instead of the empty one an unauthenticated
// HTTP GET records.
func mcpGet(s objectStore, caller, sourceIP, actorType, actorID string) mcp.ToolHandlerFor[mcpGetInput, mcpGetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpGetInput) (*mcp.CallToolResult, mcpGetOutput, error) {
		opts, err := mcpSelector(in, in.ID)
		if err != nil {
			return nil, mcpGetOutput{}, err
		}

		obj, err := s.GetObject(ctx, in.Slug, opts...)
		if err != nil {
			return nil, mcpGetOutput{}, mcpLookupError(ctx, "get", err)
		}

		if err := s.RecordAuditLog(ctx, in.Slug, store.AuditActionRead, caller, sourceIP, actorType, actorID, store.AuditVariant(obj.ID)); err != nil {
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

		obj, err := mcpUpdateObject(ctx, s, in, value, opts)
		if err != nil {
			return nil, ObjectMetadata{}, err
		}

		if err := s.RecordAuditLog(ctx, in.Slug, store.AuditActionUpdate, caller, sourceIP, actorType, actorID, store.AuditVariant(obj.ID)); err != nil {
			return nil, ObjectMetadata{}, mcpInternalError(ctx, "update", err)
		}

		return nil, ObjectMetadata{ID: obj.ID, Slug: obj.Slug, UsedBy: obj.UsedBy, Tags: tagsOrEmpty(obj.Tags), Description: obj.Description}, nil
	}
}

// mcpUpdateObject replaces the variant of in's slug that in.ID (or the slug
// alone) names, and reads it back by id, since the update may change which
// consumers name it.
func mcpUpdateObject(ctx context.Context, s objectStore, in mcpUpdateInput, value []byte, opts []store.ObjectOption) (store.Object, error) {
	selector, err := mcpSelector(in, in.ID)
	if err != nil {
		return store.Object{}, err
	}

	var id string

	opts = append(append(opts, selector...), store.IntoID(&id))
	if err := s.UpdateObject(ctx, in.Slug, value, uniqueConsumersPtr(in.UsedBy), opts...); err != nil {
		return store.Object{}, mcpLookupError(ctx, "update", err)
	}

	obj, err := s.GetObject(ctx, in.Slug, store.WithID(id))
	if err != nil {
		return store.Object{}, mcpInternalError(ctx, "update", err)
	}

	return obj, nil
}

// mcpDelete is the "delete" tool's handler - the same removal
// handleDeleteObject (delete.go) performs over HTTP.
func mcpDelete(s objectStore, caller, sourceIP, actorType, actorID string) mcp.ToolHandlerFor[mcpDeleteInput, mcpDeleteOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpDeleteInput) (*mcp.CallToolResult, mcpDeleteOutput, error) {
		opts, err := mcpSelector(in, in.ID)
		if err != nil {
			return nil, mcpDeleteOutput{}, err
		}

		var id string

		if err := s.DeleteObject(ctx, in.Slug, append(opts, store.IntoID(&id))...); err != nil {
			return nil, mcpDeleteOutput{}, mcpLookupError(ctx, "delete", err)
		}

		if err := s.RecordAuditLog(ctx, in.Slug, store.AuditActionDelete, caller, sourceIP, actorType, actorID, store.AuditVariant(id)); err != nil {
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

		limit, offset, err := pageBounds(optionalInt(in.Limit), optionalInt(in.Offset))
		if err != nil {
			return nil, nil, err
		}

		objs, err := s.ListObjects(ctx, filter)
		if err != nil {
			return nil, nil, mcpInternalError(ctx, "list", err)
		}

		total := len(objs)
		objs = objs[min(offset, total):min(offset+limit, total)]

		metadata := make([]ObjectMetadata, len(objs))
		for i, obj := range objs {
			metadata[i] = ObjectMetadata{ID: obj.ID, Slug: obj.Slug, UsedBy: obj.UsedBy, Tags: tagsOrEmpty(obj.Tags), Description: obj.Description}
		}

		return pageSummary(len(metadata), total, offset), metadata, nil
	}
}

// optionalInt is a paging input as the query-string form pageBounds reads: ""
// for one that wasn't given.
func optionalInt(v *int) string {
	if v == nil {
		return ""
	}

	return strconv.Itoa(*v)
}

// pageSummary is the text a list result carries beside its array, so a client
// can tell a full list from a page of one: the structured result stays a plain
// array, as the HTTP lists do, and the total has nowhere else to go.
func pageSummary(shown, total, offset int) *mcp.CallToolResult {
	text := fmt.Sprintf("Showing %d of %d objects, from offset %d.", shown, total, offset)
	if next := offset + shown; next < total {
		text += fmt.Sprintf(" Pass offset=%d for the next page.", next)
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
