# Design

## Context

`RevokeWriteToken`/`RevokeConsumerToken` (`internal/store/tokens.go`,
`internal/store/consumer_tokens.go`) already soft-delete via a
`revoked_at` timestamp (ADR 17). `RotateWriteToken`/`RotateConsumerToken`
already reject a dead token the same way this change needs to reject a
_live_ one - `RotateWriteToken`'s `WHERE id = ? AND revoked_at IS NULL
AND expires_at > ?` is close to the inverse of the guard this change
needs.

## Goals / Non-Goals

**Goals:**

- A real `DELETE FROM write_tokens`/`consumer_tokens` for a token that's
  already dead.
- Symmetric behavior across both token kinds - same restriction, same
  status codes.

**Non-Goals:**

- Changing how revoking already soft-deletes - ADR 17 still holds for
  a live token's revocation.
- Any UI work - that's `alrayyes/hush-hush#441`, blocked on this issue.
- Bulk/scheduled purge (for example, "auto-purge anything revoked >90 days ago") -
  not asked for; this is a caller-initiated action per token.

## Decisions

**A separate `/purge` sub-path (`DELETE /tokens/{id}/purge`,
`DELETE /consumer-tokens/{id}/purge`), not a query parameter on the
existing `DELETE /tokens/{id}`.** A query parameter (`?hard=true`) would
make an already-destructive DELETE request's meaning depend on an easy-to-
miss parameter; a distinct path makes the two actions - revoke vs. purge -
each unambiguous from the URL alone, and keeps `DELETE /tokens/{id}`'s
existing contract (always a safe, idempotent-ish soft-delete) untouched.

**Store method returns a sentinel error for "still active," distinct from
`ErrTokenNotFound`.** `PurgeWriteToken`/`PurgeConsumerToken` need to
distinguish three outcomes the handler maps to three different responses:
unknown id (404, matching every other token endpoint's precedent), active
token (409), successful purge (204). A single `WHERE id = ? AND
(revoked_at IS NOT NULL OR expires_at <= ?)` delete can't tell "no such
row" apart from "row exists but is still active" from `RowsAffected`
alone, so the store method does a `SELECT` first to decide which sentinel
to return, then the `DELETE`.

```go
var ErrTokenStillActive = errors.New("token is still active")

func (s *Store) PurgeWriteToken(ctx context.Context, id string) error {
    // SELECT revoked_at, expires_at WHERE id = ? - ErrTokenNotFound if no row,
    // ErrTokenStillActive if revoked_at IS NULL AND expires_at > now,
    // otherwise DELETE FROM write_tokens WHERE id = ?
}
```

Alternative considered: let the caller revoke, then purge only once revoked,
treating "expired but never revoked" as not eligible to purge. Rejected - the
issue's own acceptance criteria explicitly cover the expired-but-never-
revoked case, and an admin cleaning up old tokens shouldn't have to revoke
something that already stopped working on its own just to become eligible
to remove it.

## Risks / Trade-offs

- [A purged token's audit-log entries lose their description/owner
  forever] → Explicitly accepted by the issue itself; `QueryAuditLog`
  already tolerates an unresolvable actor id (no foreign key to
  `write_tokens`/`consumer_tokens` in the first place), so no query-layer
  change is needed for this to work.
- [Purge and revoke both mutate the same row; a race between them] → Not a
  new risk - `RevokeWriteToken`'s existing `WHERE ... AND revoked_at IS
NULL` and this change's own active-check both already tolerate a
  concurrent revoke deciding which one "wins" the row's final state, the
  same way rotate already does.

## Migration Plan

No schema change - `write_tokens`/`consumer_tokens` already have every
column this needs (`revoked_at`, `expires_at`). No backfill.
