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
)

// addMCPConsumerTools registers the tools over the consumer directory, which
// GET and POST /consumers serve over HTTP.
func addMCPConsumerTools(srv *mcp.Server, s objectStore) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_consumers",
		Description: "List the consumers (repos or hosts) that secrets are shared with, each with how many secrets name it and its registered age public key if it has one. One page of 50 unless page_size says otherwise; total is the number of matches in all.",
	}, mcpListConsumers(s))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_consumer",
		Description: "Add a consumer to the directory before any secret names it. Fails if it already exists.",
	}, mcpAddConsumer(s))
}

// mcpListConsumersInput is the "list_consumers" tool's input, mirroring GET
// /consumers's q, page and page_size. Page and PageSize are pointers so a
// client that sends neither gets the first page, and one that sends 0 is
// refused rather than read as absent.
type mcpListConsumersInput struct {
	Query    string `json:"query,omitempty" jsonschema:"only consumers whose name contains this text, case-insensitive"`
	Page     *int   `json:"page,omitempty" jsonschema:"which page to return, from 1; 1 when left out"`
	PageSize *int   `json:"page_size,omitempty" jsonschema:"how many consumers per page; 50 when left out"`
}

// mcpListConsumers is the "list_consumers" tool's handler - the paged listing
// handleListConsumers (consumers.go) returns over HTTP when given a filter.
// Always that richer shape, since a tool has no older caller to keep the
// plain array for.
func mcpListConsumers(s objectStore) mcp.ToolHandlerFor[mcpListConsumersInput, ConsumersPage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpListConsumersInput) (*mcp.CallToolResult, ConsumersPage, error) {
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
