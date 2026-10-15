package api

import (
	"context"
	"errors"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	errMCPConsumerRequired = errors.New("consumer is required")
	errMCPUnknownToken     = errors.New("unknown token")
	errMCPTokenStillActive = errors.New("token must be revoked or expired before it can be purged")
)

// addMCPConsumerTokenTools registers the tools over consumer read tokens,
// which /consumer-tokens serves over HTTP. That route takes a write token or a
// session, as /mcp does, so a tool widens no access: a consumer token reads
// less than the write token that mints it.
func addMCPConsumerTokenTools(srv *mcp.Server, s objectStore) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_consumer_token",
		Description: "Issue a read token scoped to one consumer. The raw value is in this result and never again: store it now. ttl_seconds defaults to 90 days and is at most 365 days.",
	}, mcpCreateConsumerToken(s))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_consumer_tokens",
		Description: "List consumer read tokens' metadata (id, consumer, status, expiry, last use), never a raw value. One page of 50 unless limit says otherwise; total is the number of tokens in all.",
	}, mcpListConsumerTokens(s))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rotate_consumer_token",
		Description: "Replace a consumer token's secret and expiry. The new raw value is in this result and never again. Fails for an unknown, revoked or expired token.",
	}, mcpRotateConsumerToken(s))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "revoke_consumer_token",
		Description: "Invalidate a consumer token now. Revoking one that is already revoked, expired or unknown is not an error.",
	}, mcpRevokeConsumerToken(s))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "purge_consumer_token",
		Description: "Permanently remove a consumer token that is already revoked or expired. Fails for an active one.",
	}, mcpPurgeConsumerToken(s))
}

// mcpCreateConsumerTokenInput is the "create_consumer_token" tool's input -
// createConsumerTokenBody's fields.
type mcpCreateConsumerTokenInput struct {
	Consumer    string `json:"consumer" maxLength:"128" jsonschema:"the consumer the token reads as"`
	Description string `json:"description,omitempty" maxLength:"200" jsonschema:"what the token is for"`
	TTLSeconds  *int64 `json:"ttl_seconds,omitempty" jsonschema:"how long the token lives, in seconds; 90 days when left out"`
}

// mcpCreateConsumerToken is the "create_consumer_token" tool's handler - the
// same issue handleCreateConsumerToken (tokens.go) performs over HTTP.
func mcpCreateConsumerToken(s objectStore) mcp.ToolHandlerFor[mcpCreateConsumerTokenInput, ConsumerTokenWithValue] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpCreateConsumerTokenInput) (*mcp.CallToolResult, ConsumerTokenWithValue, error) {
		if err := checkLimits(in); err != nil {
			return nil, ConsumerTokenWithValue{}, err
		}

		if in.Consumer == "" {
			return nil, ConsumerTokenWithValue{}, errMCPConsumerRequired
		}

		ttl, err := mcpTokenTTL(in.TTLSeconds)
		if err != nil {
			return nil, ConsumerTokenWithValue{}, err
		}

		ct, value, err := s.CreateConsumerToken(ctx, in.Consumer, in.Description, ttl)
		if err != nil {
			return nil, ConsumerTokenWithValue{}, mcpInternalError(ctx, "create_consumer_token", err)
		}

		return nil, ConsumerTokenWithValue{ConsumerTokenMetadata: consumerTokenMetadataFromStore(ct), Value: value}, nil
	}
}

// mcpTokenTTL bounds a requested lifetime with the rule the routes apply.
func mcpTokenTTL(raw *int64) (time.Duration, error) {
	ttl, status, message := resolveTTL(raw)
	if status != 0 {
		return 0, errors.New(message) //nolint:err113 // the message is the point
	}

	return ttl, nil
}

// mcpListConsumerTokensInput is the "list_consumer_tokens" tool's input,
// mirroring GET /consumer-tokens's limit and offset.
type mcpListConsumerTokensInput struct {
	Limit  *int `json:"limit,omitempty" jsonschema:"how many tokens to return, 1 to 500; 50 when left out"`
	Offset *int `json:"offset,omitempty" jsonschema:"how many tokens to skip, for the next page"`
}

// mcpConsumerTokensPage is one page of tokens with the count in all, the
// X-Total-Count header's place in the HTTP answer.
type mcpConsumerTokensPage struct {
	Total  int                     `json:"total"`
	Tokens []ConsumerTokenMetadata `json:"tokens"`
}

// mcpListConsumerTokens is the "list_consumer_tokens" tool's handler - the
// same listing and bounds handleListConsumerTokens (tokens.go) uses over HTTP.
func mcpListConsumerTokens(s objectStore) mcp.ToolHandlerFor[mcpListConsumerTokensInput, mcpConsumerTokensPage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpListConsumerTokensInput) (*mcp.CallToolResult, mcpConsumerTokensPage, error) {
		limit, offset, err := pageBounds(optionalInt(in.Limit), optionalInt(in.Offset))
		if err != nil {
			return nil, mcpConsumerTokensPage{}, err
		}

		tokens, err := s.ListConsumerTokens(ctx)
		if err != nil {
			return nil, mcpConsumerTokensPage{}, mcpInternalError(ctx, "list_consumer_tokens", err)
		}

		total := len(tokens)
		out := make([]ConsumerTokenMetadata, 0, min(limit, total))
		for _, t := range tokens[min(offset, total):min(offset+limit, total)] {
			out = append(out, consumerTokenMetadataFromStore(t))
		}

		return nil, mcpConsumerTokensPage{Total: total, Tokens: out}, nil
	}
}

// mcpConsumerTokenIDInput names the token a tool acts on.
type mcpConsumerTokenIDInput struct {
	ID string `json:"id" maxLength:"128" jsonschema:"the token's id, from create_consumer_token or list_consumer_tokens"`
}

// mcpRotateConsumerTokenInput is the "rotate_consumer_token" tool's input.
type mcpRotateConsumerTokenInput struct {
	ID         string `json:"id" maxLength:"128" jsonschema:"the token's id"`
	TTLSeconds *int64 `json:"ttl_seconds,omitempty" jsonschema:"the new lifetime in seconds; 90 days when left out"`
}

// mcpRotateConsumerToken is the "rotate_consumer_token" tool's handler - the
// same replacement handleRotateConsumerToken (tokens.go) performs over HTTP.
func mcpRotateConsumerToken(s objectStore) mcp.ToolHandlerFor[mcpRotateConsumerTokenInput, ConsumerTokenWithValue] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpRotateConsumerTokenInput) (*mcp.CallToolResult, ConsumerTokenWithValue, error) {
		if err := checkLimits(in); err != nil {
			return nil, ConsumerTokenWithValue{}, err
		}

		ttl, err := mcpTokenTTL(in.TTLSeconds)
		if err != nil {
			return nil, ConsumerTokenWithValue{}, err
		}

		ct, value, err := s.RotateConsumerToken(ctx, in.ID, ttl)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrTokenNotFound):
			return nil, ConsumerTokenWithValue{}, errMCPUnknownToken
		default:
			return nil, ConsumerTokenWithValue{}, mcpInternalError(ctx, "rotate_consumer_token", err)
		}

		return nil, ConsumerTokenWithValue{ConsumerTokenMetadata: consumerTokenMetadataFromStore(ct), Value: value}, nil
	}
}

// mcpRevokeConsumerToken is the "revoke_consumer_token" tool's handler - the
// same invalidation handleRevokeConsumerToken (tokens.go) performs over HTTP,
// including its no-error answer for an unknown id.
func mcpRevokeConsumerToken(s objectStore) mcp.ToolHandlerFor[mcpConsumerTokenIDInput, mcpRevokeOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpConsumerTokenIDInput) (*mcp.CallToolResult, mcpRevokeOutput, error) {
		if err := checkLimits(in); err != nil {
			return nil, mcpRevokeOutput{}, err
		}

		if err := s.RevokeConsumerToken(ctx, in.ID); err != nil && !errors.Is(err, store.ErrTokenNotFound) {
			return nil, mcpRevokeOutput{}, mcpInternalError(ctx, "revoke_consumer_token", err)
		}

		return nil, mcpRevokeOutput{Revoked: true}, nil
	}
}

// mcpRevokeOutput confirms a revoke, which the HTTP 204 has nothing to echo for.
type mcpRevokeOutput struct {
	Revoked bool `json:"revoked"`
}

// mcpPurgeConsumerToken is the "purge_consumer_token" tool's handler - the
// same removal handlePurgeConsumerToken (tokens.go) performs over HTTP.
func mcpPurgeConsumerToken(s objectStore) mcp.ToolHandlerFor[mcpConsumerTokenIDInput, mcpDeleteOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpConsumerTokenIDInput) (*mcp.CallToolResult, mcpDeleteOutput, error) {
		if err := checkLimits(in); err != nil {
			return nil, mcpDeleteOutput{}, err
		}

		switch err := s.PurgeConsumerToken(ctx, in.ID); {
		case err == nil:
		case errors.Is(err, store.ErrTokenNotFound):
			return nil, mcpDeleteOutput{}, errMCPUnknownToken
		case errors.Is(err, store.ErrTokenStillActive):
			return nil, mcpDeleteOutput{}, errMCPTokenStillActive
		default:
			return nil, mcpDeleteOutput{}, mcpInternalError(ctx, "purge_consumer_token", err)
		}

		return nil, mcpDeleteOutput{Deleted: true}, nil
	}
}
