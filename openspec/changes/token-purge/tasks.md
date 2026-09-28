# Tasks

## 1. Store layer: purge

- [x] 1.1 Add `ErrTokenStillActive` sentinel and `PurgeWriteToken(ctx, id)
error` to `internal/store/tokens.go`: `ErrTokenNotFound` for an
      unknown id, `ErrTokenStillActive` for a currently valid one,
      otherwise deletes the row. Verify with tests covering: purging a
      revoked token removes it, purging an expired-but-never-revoked
      token removes it, purging an active token returns
      `ErrTokenStillActive` and leaves the row, purging an unknown id
      returns `ErrTokenNotFound`.
- [x] 1.2 Add the equivalent `PurgeConsumerToken(ctx, id) error` to
      `internal/store/consumer_tokens.go`, same cases, same tests.

## 2. API layer: purge endpoints

- [x] 2.1 Add `handlePurgeToken`/`handlePurgeConsumerToken` to
      `internal/api/tokens.go`: `ErrTokenNotFound` → 404,
      `ErrTokenStillActive` → 409 with a message naming that the token
      must be revoked or expired first, success → 204. Register `DELETE
/tokens/{id}/purge` and `DELETE /consumer-tokens/{id}/purge` in
      `internal/api/server.go`, session+CSRF gated like every other
      token-mutating endpoint, and add the two new store methods to the
      `objectStore` interface.
- [x] 2.2 Verify with handler tests covering: purging a revoked/expired
      token (either kind) → 204 and gone from the list; purging an active
      token → 409, still listed and still valid; purging an unknown id →
      404; an audit-log entry attributed to a since-purged token still
      returns from `QueryAuditLog` with its id present and
      description/owner absent; unauthenticated request → 401.

## 3. OpenAPI spec and docs

- [x] 3.1 Update `api/openapi.yaml`: the two new paths, a `409` response
      (new or reused `components.responses` entry) for the active-token
      case. Verify `bun run lint:api` passes.
- [x] 3.2 Record a new ADR in `docs/adr/` documenting purge as an
      addition alongside ADR 17's soft-delete, not a reversal of it.
      Verify it reads consistently with the shipped behavior.
- [ ] 3.3 Run `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` clean, then open the pull request referencing
      alrayyes/hush-hush#439, and comment on alrayyes/hush-hush#441 that
      the purge API has landed.
