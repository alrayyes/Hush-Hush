# Tasks

## 1. Spec

- [x] 1.1 Add `POST /mcp` to `api/openapi.yaml` - method, auth requirement
      (`bearerAuth`/`cookieAuth`, same as the other write routes), a generic
      `application/json` body noting the real schema is MCP's JSON-RPC 2.0
      envelope; verify with `bun run lint:api`.
- [x] 1.2 Record the decision in `docs/adr/0019-mcp-endpoint.md`: embedded
      vs. sidecar, SDK choice + pinned version, uniform gating across all
      five tools, stateless per-request server construction, no new actor
      type.

## 2. API

- [ ] 2.1 Add `github.com/modelcontextprotocol/go-sdk` to `go.mod`,
      pinned to an exact version.
- [ ] 2.2 Implement `internal/api/mcp.go`: `handleMCP`, one handler
      function per tool (`inject`/`get`/`update`/`delete`/`list`), reusing
      `CreateObjectRequest`/`UpdateObjectRequest`/`ObjectMetadata` where
      they already fit; wire `POST /mcp` into `NewMux`
      (`internal/api/server.go`) behind `requireWriteAccess`.
- [ ] 2.3 Failing tests first in `internal/api/mcp_test.go`: unauthenticated
      `POST /mcp` rejected; `tools/list` returns the five tools; each tool
      round-trips through the store and records the expected audit entry;
      not-found/already-exists map to a tool error (`isError: true`), not a
      protocol error.
- [ ] 2.4 Add a `tools/list` contract case to
      `internal/api/openapi_test.go` so the existing contract test still
      proves the route matches the spec.

## 3. Docs

- [ ] 3.1 `README.md`: new subsection under `## Usage`, next to
      `### Start the server`, documenting the endpoint - how to point an
      MCP client at it, that it needs a bearer token, the five tools.

## 4. Verification

- [ ] 4.1 `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` pass.
- [ ] 4.2 `bun run lint:api` and `bun run format:check` pass.
- [ ] 4.3 Manually verified against the running binary: issue a token,
      point an MCP client (for example, MCP Inspector) at `POST /mcp` with it,
      confirm `tools/list` and a round-trip `inject`/`get`/`delete` work
      end to end.
