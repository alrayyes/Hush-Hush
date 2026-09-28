# Tasks

## 1. Store layer: consumer tokens

- [x] 1.1 Add `consumer_tokens` table to `internal/store/schema.sql` (`id`,
      `consumer`, `token_hash`, `description`, `created_at`, `expires_at`,
      `revoked_at`, `last_used_at`) and verify `go test ./internal/store/...`
      still passes against a fresh database (table creation is exercised
      implicitly by every store test opening one).
- [x] 1.2 Add a `ConsumerToken` struct and
      `CreateConsumerToken`/`ListConsumerTokens`/`RotateConsumerToken`/
      `RevokeConsumerToken`/`AuthenticateConsumerToken`/
      `UpdateConsumerTokenUsage` methods to `internal/store/tokens.go`,
      mirroring the existing `WriteToken` methods, and verify with unit
      tests covering: create returns the raw value once, list never
      returns a raw value, authenticate rejects unknown/expired/revoked
      tokens, rotate replaces the secret in place and rejects a
      revoked/expired id, revoke is a soft-delete that leaves the row
      listed.

## 2. API layer: consumer token endpoints

- [x] 2.1 Add `ConsumerTokenMetadata`/`ConsumerTokenWithValue` response
      types and `handleCreateConsumerToken`/`handleListConsumerTokens`/
      `handleRotateConsumerToken`/`handleRevokeConsumerToken` handlers to
      `internal/api/tokens.go`, matching the existing write-token
      handlers' structure and error handling.
- [x] 2.2 Register `POST /consumer-tokens`, `GET /consumer-tokens`,
      `POST /consumer-tokens/{id}/rotate`, `DELETE /consumer-tokens/{id}`
      in `internal/api/server.go`, session+CSRF gated like the existing
      `/tokens` routes, and add the six new store methods to the
      `objectStore` interface. Verify with handler tests covering create/
      list/rotate/revoke happy paths and the same error cases as 1.2's
      store tests, plus a 401 for an unauthenticated request to each
      endpoint.

## 3. Gate GET /objects/{slug}

- [x] 3.1 Add a `readAuth` helper to `internal/api/server.go` that tries
      write-bearer-token, then session, then consumer-token
      authentication in turn (design.md's "authorizes inside the
      handler" decision) and returns which kind matched, if any.
- [x] 3.2 Update `handleGetObject` (`internal/api/get.go`) to call
      `readAuth`, reject with 401 if nothing matched, and - for a
      matched consumer token - check the requested object's `used_by`
      includes that token's bound consumer before returning the
      ciphertext, responding 404 (not 403) when it doesn't. Update
      `UpdateConsumerTokenUsage`/`UpdateWriteTokenUsage` to record on a
      successful authenticated read the same way `requireWriteAccess`
      already does for writes.
- [x] 3.3 Extend `actorFrom` (`internal/api/server.go`) with the
      `consumer_token` actor type so an authenticated read's audit-log
      entry attributes to the real consumer token, not `"", ""`.
- [x] 3.4 Verify with handler tests covering: no credential → 401;
      expired/revoked credential → 401; write token or session reads any
      object; consumer token reads an object in its `used_by`; consumer
      token gets 404 (identical body/status to an unknown slug) for an
      object outside its scope; the audit log records `consumer_token`
      actor attribution for a successful scoped read.

## 4. OpenAPI spec and docs

- [x] 4.1 Update `api/openapi.yaml`: new schemas
      (`CreateConsumerTokenRequest`, `ConsumerTokenMetadata`,
      `ConsumerTokenWithValue`, `RotateConsumerTokenRequest`), the four
      new paths, and `GET /objects/{slug}`'s security requirement changed
      from none to bearer-or-session. Verify `bun run lint:api` passes.
- [x] 4.2 Record a new ADR in `docs/adr/` documenting this decision as
      partially superseding ADR 2 (mirroring how ADR 14 is recorded
      against ADR 3), and update `ARCHITECTURE.md`'s "Read path" bullet
      to match. Verify both read consistently with the shipped behaviour.
- [ ] 4.3 Run `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` clean, then open the pull request referencing
      alrayyes/hush-hush#438, and comment on alrayyes/hush-hush#440,
      #441, hush-hush-cli#133, and hush-hush-action#31 that the API has
      landed with a link to the merged `api/openapi.yaml`.
