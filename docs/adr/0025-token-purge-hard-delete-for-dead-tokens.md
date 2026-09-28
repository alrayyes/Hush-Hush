# 25. Hard-delete a token, once it's already dead

## Status

Accepted. Adds to [ADR 17](0017-token-revocation-soft-delete.md), not a
reversal of it - a live token's revocation is still always a soft-delete;
this only adds a further-restricted action on top of that for a token
that's already revoked or expired.

## Context

ADR 17 moved revocation to a `revoked_at` soft-delete specifically so an
audit-log entry attributed to a token keeps resolving to its description
and owner after the token stops working. That's still the right default,
but it means a dead token's row now accumulates forever with no way to
actually remove it - an admin cleaning up a long list of expired
one-off tokens has no way to shrink it.
[Issue #439](https://github.com/alrayyes/hush-hush/issues/439) asked for a
real delete, scoped narrowly enough not to undo what ADR 17 already
settled.

## Decision

`PurgeWriteToken`/`PurgeConsumerToken` (store) and
`DELETE /tokens/{id}/purge`/`DELETE /consumer-tokens/{id}/purge` (HTTP,
session+CSRF gated like every other token-mutating endpoint) permanently
remove a token's row, restricted to a token that's already revoked or
past its `expires_at`. Attempting to purge a currently valid token is
rejected with `409`. An audit-log entry that already references a purged
token's id resolves to nothing for that id afterward - accepted
explicitly by the issue, not treated as a defect: `write_tokens`/
`consumer_tokens` have no foreign key from `audit_log` in the first
place (ADR 17's own reasoning), so a purge needs no query-layer change
for this to keep working.

A separate `/purge` path was chosen over a query parameter on the
existing `DELETE /tokens/{id}` (for example, `?hard=true`) - a query parameter
would make an already-destructive request's real effect depend on an
easy-to-miss parameter, where a distinct path keeps `DELETE
/tokens/{id}`'s existing contract (always a safe, idempotent-ish
soft-delete) completely unchanged and makes the two actions unambiguous
from the URL alone.

## Consequences

- A dead token can now actually be removed from the list, closing the
  gap ADR 17 left open on purpose.
- Nothing about how revoking works changes - `DELETE /tokens/{id}`
  still always soft-deletes, regardless of this addition.
- A purged token's audit history stops resolving to a description/owner,
  permanently. This is an explicit, caller-initiated choice per token,
  not something that happens on its own.
- No bulk or scheduled purge exists - each purge is one caller-initiated
  action against one token id, same granularity every other token
  endpoint already has.
