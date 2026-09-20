# Tasks

## 1. Store

- [ ] 1.1 Add a `last_used_at` column to `write_tokens` via
      `addColumnIfMissing` (same pattern as `owner`/`revoked_at`), add it
      to `WriteToken`, and add `UpdateWriteTokenUsage(ctx, id, usedAt
string) error`; verify with a store test asserting a fresh token has
      an empty `LastUsedAt` and it updates after
      `UpdateWriteTokenUsage`.

## 2. API

- [ ] 2.1 Call `UpdateWriteTokenUsage` from `requireWriteAccess`
      (`internal/api/server.go`) after a successful token authentication,
      and add `LastUsedAt` to `TokenMetadata`
      (`internal/api/tokens.go`); verify with a Go test that `GET /tokens`
      reflects a timestamp after an authenticated request and stays empty
      for an unused token.
- [ ] 2.2 Add `last_used_at` to `TokenMetadata` in `api/openapi.yaml`;
      verify `bun run lint:api` passes.

## 3. Web UI

- [ ] 3.1 Show each token's last-used time in the settings page's token
      list, matching the existing passkey list's presentation; verify with
      a component test covering both an unused and a used token.

## 4. Verification

- [ ] 4.1 `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` pass.
- [ ] 4.2 `bun run test`, `bun run check`, `bun run lint`, and
      `bun run format:check` pass in `cmd/hush-hush/web`.
- [ ] 4.3 Manually verified against the running binary: create a token,
      confirm it shows no last-used time, use it for a write, confirm the
      settings page now shows a last-used time for it.
