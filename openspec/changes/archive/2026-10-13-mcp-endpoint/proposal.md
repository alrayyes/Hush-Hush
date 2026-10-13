# Proposal

## Why

Hush-Hush's write and read paths are only reachable today by a human
running `hush-hush-cli` by hand, or by another service wired up as a
direct HTTP consumer. There's no way for an MCP-capable agent to inject,
fetch, rotate, or list secrets on a user's behalf as part of a larger
task - doing that today means dropping out of the agent loop to run the
CLI manually (alrayyes/hush-hush#346).

## What Changes

- A new `POST /mcp` endpoint, embedded in the existing binary, serving
  the [MCP](https://modelcontextprotocol.io) Streamable HTTP transport via
  the official `github.com/modelcontextprotocol/go-sdk`.
- Five tools mirroring `hush-hush-cli`'s own operations one-to-one:
  `inject`, `get`, `update`, `delete`, `list`.
- Gated by the same bearer-token-or-session credential every other write
  route already accepts (`requireWriteAccess`), applied uniformly to all
  five tools - including `get` and `list`, which are stricter than their
  plain-HTTP equivalents (`GET /objects/{id}` is unauthenticated by ADR
  0002). One authenticated MCP session does all five operations, matching
  the ticket's framing: "the agent needs a real bearer token… same as
  any other consumer."
- Every tool call is audit logged the same way its HTTP equivalent is,
  attributed to the same verified actor (`token`/`session`, ADR 0016) - no
  new actor type.
- Stateless HTTP mode: a fresh MCP server is constructed per incoming
  request rather than kept alive across a resumable session, since a
  CRUD tool call doesn't need one.

## Capabilities

### New Capabilities

- `mcp`: an MCP endpoint mirroring the existing object operations.

### Modified Capabilities

_None._

## Impact

- `api/openapi.yaml`: adds `POST /mcp` (this PR) - a generic
  `application/json` request/response, since OpenAPI can't usefully
  describe JSON-RPC's dynamic per-tool dispatch; the real contract is
  each tool's own JSON Schema, generated from Go types at compile time.
- `docs/adr/0019-mcp-endpoint.md`: records the transport, SDK, and
  uniform-gating decisions (this PR).
- `internal/api/mcp.go`, `internal/api/server.go`, `go.mod`/`go.sum`,
  `internal/api/openapi_test.go`, `README.md`: the implementation,
  landing in a follow-up PR once this one merges (alrayyes/hush-hush#346
  precedent: PR #283 then #289 for the consumer rename/delete endpoints).
