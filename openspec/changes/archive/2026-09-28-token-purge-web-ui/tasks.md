# Tasks

## 1. Expiry helper and API client

- [x] 1.1 Add `isTokenDead` to `src/lib/datetime.ts` per `design.md`;
      verify with a unit test in `datetime.spec.ts` covering: revoked +
      not expired (dead), not revoked + expired (dead), not revoked +
      not expired (alive), revoked + expired (dead).
- [x] 1.2 Add `purgeToken(id)`/`purgeConsumerToken(id)` to
      `src/lib/api.ts` (`DELETE /tokens/{id}/purge` /
      `DELETE /consumer-tokens/{id}/purge`, 204 with an empty body),
      mirroring
      `revokeToken`/`revokeConsumerToken`'s shape; verify with
      `bun run check` that the calls type-check against
      `api/openapi.yaml`'s `purgeToken`/`purgeConsumerToken` operations.

## 2. Settings page: status and actions

- [x] 2.1 Replace both tables' `token.revoked ? 'Revoked' : 'Active'`
      Status column with the three-state
      `token.revoked ? 'Revoked' : isTokenDead(token) ? 'Expired' : 'Active'`;
      verified via the e2e test in task group 4, which seeds an
      already-expired token directly through the API (a 1-second TTL,
      the lowest value `CreateTokenRequest.ttl_seconds` allows) and
      confirms it reads "Expired".
- [x] 2.2 Gate both tables' Actions column: Rotate/Revoke when
      `!isTokenDead(token)`, "Delete permanently" when `isTokenDead(token)` - never both; styled with Tailwind utility classes only, no new
      `app.css` rules; verified against the real running app (screenshots)
      that an active token shows only Rotate/Revoke and a dead one shows
      only Delete permanently.
- [x] 2.3 Add the purge confirmation `AlertDialog` for both token types
      (own state each, mirroring the existing revoke dialogs' naming),
      with the required warning text naming audit-log entries becoming
      unresolvable; verified against the real running app (screenshots)
      that the token disappears from its list immediately on confirm,
      with no page reload.

## 3. Tests

- [x] 3.1 Extend `e2e/journey.spec.ts` with a flow that seeds an
      already-dead token (revoke one, or set a short TTL and wait past
      it), confirms only "Delete permanently" is offered (not
      Rotate/Revoke), purges it, and confirms it's gone from the list -
      for both the bearer-token and consumer-token tables - plus an
      axe-core scan of the open confirmation dialog; passes against a
      fresh `bun run build`, twice, to rule out flakiness from the
      timing-sensitive expiry wait.

## 4. Verification

- [x] 4.1 `bun run lint:tailwind`, `bun run check`, `bun run lint`,
      `bun run test`, and (after a fresh `bun run build`)
      `bun run test:e2e` all pass in `cmd/hush-hush/web`.
- [x] 4.2 Manually verified against the running binary: an active
      token shows no purge action; a revoked token's purge removes it
      immediately. The 409-on-clock-skew path isn't separately tested
      (hard to simulate deterministically without mocking the browser
      clock) - `confirmPurge`/`confirmPurgeConsumerToken` reuse the exact
      same try/catch + `apiErrorMessage` pattern every other action on
      this page already uses for surfacing a server error, so this rides
      on that already-proven path rather than needing its own test
      (design.md's own risk note: a client-side misjudgment can't violate
      the "never purge an active token" invariant, since the server
      still enforces it).
