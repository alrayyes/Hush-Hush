# Design

## Context

`TokenMetadata` and `ConsumerTokenMetadata` (`cmd/hush-hush/web/src/lib/api.ts`)
both carry `revoked: boolean` and `expires_at: string` but no
server-computed "is this token dead" field - the server never needed one
until now, since revoke/rotate/list didn't care about expiry, only
revoked status. Both tables' Status column today is
`token.revoked ? 'Revoked' : 'Active'`, which already mislabels an
expired-but-unrevoked token as active - a latent bug this change's own
acceptance criteria exposes rather than one it introduces.

See `proposal.md` for why this change exists.

## Goals / Non-Goals

**Goals:**

- One shared way to compute a token's real status (active/expired/revoked)
  from `revoked` + `expires_at`, used by both tables' Status column and
  both tables' Actions-column gating - not two copies of the same
  comparison.
- Fix the existing Status column's mislabelling as part of landing this,
  rather than shipping a purge button next to a label that contradicts it.

**Non-Goals:**

- A live-updating countdown or "expires in 3 days" style display -
  `formatTimestamp` already renders `expires_at` as an absolute timestamp,
  and that's unaffected here.

## Decisions

**Add `isTokenDead(token: { revoked: boolean; expires_at: string }):
boolean` to `src/lib/datetime.ts`**, next to `formatTimestamp` - that
file is already the one place time-display/comparison logic lives, and
both `TokenMetadata` and `ConsumerTokenMetadata` structurally satisfy the
same minimal shape, so one function serves both tables without importing
either type into `datetime.ts`.

- Computes `revoked || new Date(expires_at).getTime() <= Date.now()`.
- The Status column then reads `token.revoked ? 'Revoked' : isTokenDead(token)
? 'Expired' : 'Active'` - revoked checked first since a token can be both
  revoked and past its expiry, and "Revoked" is the more informative label
  of the two in that case (it names an admin action; "Expired" is just
  time passing).
- Rotate/Revoke show when `!isTokenDead(token)`, and "Delete permanently"
  shows when `isTokenDead(token)` - mutually exclusive, matching the
  spec's "never both" requirement.

**No new dialog component** - the purge confirmation reuses the existing
`AlertDialog` pattern already used for revoke (secrets' delete, consumers'
delete, bearer tokens' revoke), with its own state
(`purgeOpen`/`purgeId`/`purgeError` and the consumer-token equivalents),
mirroring the naming convention `+page.svelte` already uses for its other
four token-action dialogs.

## Risks / Trade-offs

- `isTokenDead` computes against the browser's own clock, not the
  server's - a client clock skewed forward could show "Expired" (and
  offer purge) a little early, or skewed back could show "Active" for a
  server-side-already-expired token a little longer. Purging is still
  gated server-side by `TokenStillActive`, so a client-side misjudgment
  can't actually violate the "never purge an active token" invariant -
  worst case is the UI's guess disagreeing with the button being
  clickable for a moment, not a real safety issue.
