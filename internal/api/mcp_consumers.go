package api

import (
	"context"
	"errors"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	errMCPConsumerNameRequired = errors.New("name is required")
	errMCPConsumerExists       = errors.New("consumer already exists")
	errMCPUnknownConsumer      = errors.New("unknown consumer")
)

// addMCPConsumerTools registers the tools over the consumer directory, which
// GET and POST /consumers serve over HTTP.
func addMCPConsumerTools(srv *mcp.Server, s objectStore) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_consumers",
		Description: "List the consumers (repos or hosts) that secrets are shared with, each with how many secrets name it and its registered age public key if it has one. One page of 20 unless page_size says otherwise (at most 100); total is the number of matches in all.",
	}, mcpListConsumers(s))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_consumer",
		Description: "Add a consumer to the directory before any secret names it. Fails if it already exists.",
	}, mcpAddConsumer(s))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_consumer",
		Description: "Rename a consumer, register or replace its age public key, or both. The consumer's read tokens follow a rename. A new_name that is already a consumer merges the two, and tokens of both then read as the merged name. Send at least one of new_name and public_key; a key can be replaced but not cleared.",
	}, mcpUpdateConsumer(s))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_consumer",
		Description: "Remove a consumer from every secret that names it, and revoke its read tokens. The secrets themselves stay, even if this leaves one with no consumers.",
	}, mcpDeleteConsumer(s))
}

// mcpListConsumersInput is the "list_consumers" tool's input, mirroring GET
// /consumers's q, page and page_size. Page and PageSize are pointers so a
// client that sends neither gets the first page, and one that sends 0 is
// refused rather than read as absent.
type mcpListConsumersInput struct {
	Query    string `json:"query,omitempty" maxLength:"128" jsonschema:"only consumers whose name contains this text, case-insensitive"`
	Page     *int   `json:"page,omitempty" jsonschema:"which page to return, from 1; 1 when left out"`
	PageSize *int   `json:"page_size,omitempty" jsonschema:"how many consumers per page, 1 to 100; 20 when left out"`
}

// mcpListConsumers is the "list_consumers" tool's handler - the paged listing
// handleListConsumers (consumers.go) returns over HTTP when given a filter.
// Always that richer shape, since a tool has no older caller to keep the
// plain array for.
func mcpListConsumers(s objectStore) mcp.ToolHandlerFor[mcpListConsumersInput, ConsumersPage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpListConsumersInput) (*mcp.CallToolResult, ConsumersPage, error) {
		if err := checkLimits(in); err != nil {
			return nil, ConsumersPage{}, err
		}

		filter, err := consumerFilter(in.Query, optionalInt(in.Page), optionalInt(in.PageSize))
		if err != nil {
			return nil, ConsumersPage{}, err
		}

		page, err := s.ListConsumersPage(ctx, filter)
		if err != nil {
			return nil, ConsumersPage{}, mcpInternalError(ctx, "list_consumers", err)
		}

		return nil, consumersPageFrom(page), nil
	}
}

// mcpAddConsumerInput is the "add_consumer" tool's input - AddConsumerRequest.
type mcpAddConsumerInput struct {
	Name string `json:"name" maxLength:"128" jsonschema:"the consumer's name, a repo or host"`
}

// mcpAddConsumer is the "add_consumer" tool's handler - the same insert
// handleAddConsumer (consumers.go) performs over HTTP.
func mcpAddConsumer(s objectStore) mcp.ToolHandlerFor[mcpAddConsumerInput, ConsumerEntry] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpAddConsumerInput) (*mcp.CallToolResult, ConsumerEntry, error) {
		if err := checkLimits(in); err != nil {
			return nil, ConsumerEntry{}, err
		}

		if in.Name == "" {
			return nil, ConsumerEntry{}, errMCPConsumerNameRequired
		}

		switch err := s.AddConsumer(ctx, in.Name); {
		case err == nil:
		case errors.Is(err, store.ErrConsumerAlreadyExists):
			return nil, ConsumerEntry{}, errMCPConsumerExists
		default:
			return nil, ConsumerEntry{}, mcpInternalError(ctx, "add_consumer", err)
		}

		return nil, ConsumerEntry{Name: in.Name, SecretCount: 0}, nil
	}
}

// mcpUpdateConsumerInput is the "update_consumer" tool's input -
// UpdateConsumerRequest's fields plus the name the HTTP PATCH takes from its
// path. new_name is the rename, since name already names the consumer.
type mcpUpdateConsumerInput struct {
	Name      string `json:"name" maxLength:"128" jsonschema:"the consumer's current name"`
	NewName   string `json:"new_name,omitempty" maxLength:"128" jsonschema:"renames the consumer; omit to keep the name"`
	PublicKey string `json:"public_key,omitempty" maxLength:"512" jsonschema:"the consumer's age public key (age1...); omit to leave it unchanged"`
}

// mcpUpdateConsumer is the "update_consumer" tool's handler - the same change
// handleUpdateConsumer (consumers.go) performs over HTTP.
func mcpUpdateConsumer(s objectStore) mcp.ToolHandlerFor[mcpUpdateConsumerInput, ConsumerEntry] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpUpdateConsumerInput) (*mcp.CallToolResult, ConsumerEntry, error) {
		if err := checkLimits(in); err != nil {
			return nil, ConsumerEntry{}, err
		}

		req := UpdateConsumerRequest{Name: in.NewName, PublicKey: in.PublicKey}
		// Checked before anything is applied, so a bad key also skips the rename.
		if msg := validateUpdateConsumer(req); msg != "" {
			return nil, ConsumerEntry{}, errors.New(msg) //nolint:err113 // the message is the point
		}

		entry, err := updateConsumer(ctx, s, in.Name, req)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrUnknownConsumer):
			return nil, ConsumerEntry{}, errMCPUnknownConsumer
		case errors.Is(err, store.ErrVariantConflict):
			return nil, ConsumerEntry{}, conflictErr(err, errVariantConflict)
		default:
			return nil, ConsumerEntry{}, mcpInternalError(ctx, "update_consumer", err)
		}

		return nil, ConsumerEntry{Name: entry.Name, SecretCount: entry.SecretCount, PublicKey: entry.PublicKey}, nil
	}
}

// mcpDeleteConsumerInput is the "delete_consumer" tool's input - just the name.
type mcpDeleteConsumerInput struct {
	Name string `json:"name" maxLength:"128" jsonschema:"the consumer's name"`
}

// mcpDeleteConsumer is the "delete_consumer" tool's handler - the same removal
// handleDeleteConsumer (consumers.go) performs over HTTP.
func mcpDeleteConsumer(s objectStore) mcp.ToolHandlerFor[mcpDeleteConsumerInput, mcpDeleteOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpDeleteConsumerInput) (*mcp.CallToolResult, mcpDeleteOutput, error) {
		if err := checkLimits(in); err != nil {
			return nil, mcpDeleteOutput{}, err
		}

		switch err := s.DeleteConsumer(ctx, in.Name); {
		case err == nil:
		case errors.Is(err, store.ErrUnknownConsumer):
			return nil, mcpDeleteOutput{}, errMCPUnknownConsumer
		default:
			return nil, mcpDeleteOutput{}, mcpInternalError(ctx, "delete_consumer", err)
		}

		return nil, mcpDeleteOutput{Deleted: true}, nil
	}
}
