# Proposal

## Why

`DELETE /tokens/{id}` (and its consumer-token equivalent) only
soft-deletes, deliberately, per
[ADR 17](../../../docs/adr/0017-token-revocation-soft-delete.md): a hard
delete would break an existing audit-log entry's attribution back to that
token's description and owner. Dead tokens accumulate in the list forever
with no way to actually remove them, which is the gap
[alrayyes/hush-hush#439](https://github.com/alrayyes/hush-hush/issues/439)
asks to close - a hard-delete action, but only for tokens that are already
dead (revoked or expired), so nothing still authenticating anything can be
purged out from under an in-flight audit review.

## What Changes

- A new purge action for both write (bearer) tokens and consumer read
  tokens, restricted to a token that's already revoked or past its
  `expires_at`.
- Purging an active (unrevoked, unexpired) token is rejected with `409`,
  naming that it must be revoked or expired first.
- A purged token's row is actually removed. An audit-log entry that already
  references its id resolves to nothing for that id afterward (no
  description/owner) - the accepted tradeoff this issue explicitly signs
  off on, not a bug to guard against.
- New HTTP endpoints: `DELETE /tokens/{id}/purge` and
  `DELETE /consumer-tokens/{id}/purge`.
- A new ADR recording this as an addition alongside ADR 17's soft-delete,
  not a reversal of it.

## Capabilities

Same situation as
[alrayyes/hush-hush#438](https://github.com/alrayyes/hush-hush/issues/438)'s
own change: `openspec list --specs` still reports nothing, so there's no
canonical `tokens` spec to diff a delta against yet
(`openspec/changes/consumer-read-tokens/proposal.md` has the fuller note on
why - unarchived history across this whole repo, not specific to this
change). This delta is `ADDED`, scoped to only the purge behavior this
change introduces.

### New Capabilities

- `tokens`: adds a purge action (hard-delete restricted to dead tokens) for
  both write and consumer tokens, alongside their existing create/list/
  rotate/revoke requirements. (Reuses the existing path name; see note
  above.)

### Modified Capabilities

(none - see note above)

## Impact

- `internal/store/tokens.go`, `internal/store/consumer_tokens.go`: new
  `PurgeWriteToken`/`PurgeConsumerToken` store methods.
- `internal/api/tokens.go`, `internal/api/server.go`: new handlers and
  routes.
- `api/openapi.yaml`: two new endpoints, a new `409` response for an
  active-token purge attempt.
- `docs/adr/`: one new ADR.
- No change to `GET /objects/{slug}`'s auth requirements or to how a
  token is revoked - this only adds a further-restricted delete on top of
  what #438/#446 already shipped.
