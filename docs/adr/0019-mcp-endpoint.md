# 19. Serve an MCP endpoint from the existing binary, gated uniformly

## Status

Accepted

## Context

An MCP-capable agent doing infrastructure or deploy work regularly needs
to read or write a secret as one step in a larger task
(alrayyes/hush-hush#346). Today that means dropping out of the agent loop
to run `hush-hush-cli` by hand. The ticket asks for new transport onto
the same operations the CLI already wraps - `inject`, `get`, `update`,
`delete`, `list` - without loosening the service's actual security model.

Two shapes were on the table: embed an MCP server in the existing binary
behind a new route, or run it as a small sidecar process talking to the
existing HTTP API. Serving MCP itself means picking a protocol
implementation too - hand-rolled JSON-RPC 2.0, or an existing SDK.

## Decision

**Embed, don't add a sidecar.** This repo is architecturally a single Go
binary end to end ([ADR 10](0010-go-embed-single-binary.md)), with no
existing precedent for a second deployable process. A sidecar would need
its own auth story to reach the store; embedding keeps one binary, one
auth model, one thing to deploy.

**Use the official SDK**, `github.com/modelcontextprotocol/go-sdk`
(Anthropic/Google-maintained), pinned to `v1.8.0`. It provides
`mcp.NewServer`, the generic `mcp.AddTool[In, Out]` (JSON Schema inferred
from Go struct tags), and `mcp.NewStreamableHTTPHandler` for the HTTP
transport - no reason to hand-roll JSON-RPC framing, session handling, or
schema generation ourselves.

**One route, `POST /mcp`, gated uniformly across all five tools** by the
existing `requireWriteAccess(s, true, handleMCP(s, version))` - the same
bearer-token-or-session credential already checked on every other write
route. This is stricter than plain HTTP for two of the five operations:
`get` is unauthenticated over HTTP ([ADR 2](0002-unauthenticated-read-path-with-audit-log.md)),
and `list` accepts write-token-or-session the same way it does over HTTP
([ADR 7](0007-listing-gated-by-write-token.md)) rather than being
downgraded to match `get`. The ticket's own framing - "the agent needs a
real bearer token… same as any other consumer" - describes one
authenticated MCP session doing all five operations, not five
independently gated tools reproducing each HTTP route's own auth shape.
Splitting per-tool auth inside a single `POST /mcp` handler would also
need the request body parsed (to know which tool is being called) before
authentication could even run, which the current `requireWriteAccess`
middleware shape doesn't support without real rework.

**Stateless HTTP mode** (`StreamableHTTPOptions{Stateless: true}`), a
fresh `*mcp.Server` built per incoming HTTP request. The SDK's
`NewStreamableHTTPHandler` takes a `getServer func(*http.Request) *Server`
callback, so each tool handler closure is bound to that request's already
-verified actor (`actorFrom`, `callerFrom`, `sourceIPFrom` -
`internal/api/server.go`'s existing helpers) at construction time, rather
than relying on the SDK threading arbitrary `context.Value` entries from
the HTTP request into a tool handler's own `ctx`. A resumable, stateful
MCP session buys nothing for a simple CRUD tool call and isn't needed
here.

**No new audit-log actor type.** [ADR 16](0016-audit-log-actor-columns.md)
added `actor_type`/`actor_id` distinguishing `token` from `session`. MCP
is new transport onto the same two credentials, not a new credential
kind - every tool call records an audit entry the same way its HTTP
equivalent does, reusing `token`/`session` as-is. `get` becomes
attributed instead of blank now that it's authenticated over this
transport.

## Consequences

- An MCP client gets the same trust boundary as the CLI or any other
  bearer-token consumer, with no new unauthenticated surface.
- `get` and `list` behave differently depending on transport: unauthenticated
  (or write-token-gated, for `list`) over plain HTTP, always authenticated
  over MCP. That's a deliberate, visible divergence worth this record
  existing for - not an oversight to reconcile later.
- `api/openapi.yaml` documents `POST /mcp`'s existence and auth
  requirement, but not its JSON-RPC body - the real per-tool contract
  lives in the Go types `mcp.AddTool` infers schemas from, not in the
  OpenAPI document. A reader wanting the actual tool schemas needs a
  running server's own `tools/list`, not the spec.
- A resumable/stateful MCP session (multiple tool calls over one
  `Mcp-Session-Id`) isn't supported. Revisit only if a real need for it
  shows up - nothing here forecloses adding it.
