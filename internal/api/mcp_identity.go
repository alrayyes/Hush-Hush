package api

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addMCPOwnerIdentityTool registers the tool over the owner's escrowed
// identity, which GET /auth/identity serves over HTTP.
func addMCPOwnerIdentityTool(srv *mcp.Server, s objectStore) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "owner_identity",
		Description: "Get the owner's escrowed age public key, empty until the owner has registered. Add it as a sealing recipient to keep a copy the owner can read. Takes no arguments.",
	}, mcpOwnerIdentity(s))
}

// mcpOwnerIdentityInput is empty: GET /auth/identity takes no parameters.
type mcpOwnerIdentityInput struct{}

// mcpOwnerIdentity is the "owner_identity" tool's handler - the same lookup
// handleAuthIdentity (status.go) performs over HTTP. The route takes a write
// token or a session, as does /mcp, so no access is widened.
func mcpOwnerIdentity(s objectStore) mcp.ToolHandlerFor[mcpOwnerIdentityInput, OwnerIdentity] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ mcpOwnerIdentityInput) (*mcp.CallToolResult, OwnerIdentity, error) {
		publicKey, err := s.CurrentUserPublicKey(ctx)
		if err != nil {
			return nil, OwnerIdentity{}, mcpInternalError(ctx, "owner_identity", err)
		}

		return nil, OwnerIdentity{PublicKey: publicKey}, nil
	}
}
