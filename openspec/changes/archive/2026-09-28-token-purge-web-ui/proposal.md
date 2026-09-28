# Proposal

## Why

The backend half of token purging (#439) landed `DELETE /tokens/{id}/purge`
and `DELETE /consumer-tokens/{id}/purge` in `api/openapi.yaml` - a
hard-delete for a token that's already dead (revoked or past its expiry),
so the settings page's token tables don't grow forever with entries
nobody can ever remove (`docs/adr/0017-token-revocation-soft-delete.md`'s
soft-delete stays the default; purge is a further, restricted action on
top of it). There's no way to trigger it from the web UI yet. Filed as
`alrayyes/hush-hush#441`.

## What Changes

- Add a "Delete permanently" action to both the Bearer tokens and
  Consumer tokens tables, shown only for a token that's revoked or past
  its `expires_at` - never for an active one.
- Both `TokenMetadata` and `ConsumerTokenMetadata` carry `revoked` and
  `expires_at` but no server-computed "is this dead" field, so the
  client computes it. The existing Status column
  (`token.revoked ? 'Revoked' : 'Active'`) is wrong today for an
  expired-but-unrevoked token - fixed here to a real three-state
  Active/Expired/Revoked status, since showing "Active" next to a
  "Delete permanently" button would be self-contradictory.
- A confirmation dialog naming the real, permanent consequence: audit-log
  entries referencing the purged token become unresolvable.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `tokens`: adds a purge action to the web UI, alongside the existing
  bearer/consumer-token create/list/rotate/revoke management
  (`openspec list --specs` still reports nothing to diff against - same
  situation #440's own proposal already documented - so this is declared
  as an addition here).

## Impact

- `cmd/hush-hush/web/src/lib/api.ts`: new `purgeToken`/
  `purgeConsumerToken` functions.
- `cmd/hush-hush/web/src/lib/datetime.ts`: a new expiry-check helper.
- `cmd/hush-hush/web/src/routes/(app)/settings/+page.svelte`: both
  tables' Status column and Actions column, plus the new confirmation
  dialogs and their state.
- `cmd/hush-hush/web/e2e/journey.spec.ts`: new coverage plus an axe-core
  scan of the confirmation dialog.
