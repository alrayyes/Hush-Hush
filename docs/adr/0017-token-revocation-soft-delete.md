# 17. Revoke write tokens with a `revoked_at` timestamp, not a `DELETE`

## Status

Accepted

## Context

Token revocation originally worked by `DELETE FROM write_tokens`. Once the
audit log started attributing writes to individual tokens ([ADR
16](0016-audit-log-actor-columns.md)), a hard delete became a problem: an
old audit entry pointing at a deleted token's id resolves to nothing a
reviewer can explain during an incident.

## Decision

Token revocation moves to a `revoked_at` timestamp column.
`ValidateWriteToken` gains a `revoked_at IS NULL` check alongside its
existing expiry check. The CLI's own `token revoke` command's interface is
unchanged - only what happens underneath it.

A revoked token that's actually deleted takes its description and owner
with it, exactly the gap this change's audit-log-per-token requirement
exists to close.

## Consequences

- Revoked tokens accumulate in the table rather than being removed;
  nothing in this design prunes them, since they're the record an audit
  review needs.
- No foreign key from `audit_log` to `write_tokens` - consistent with the
  audit log having no foreign key to objects either, so an entry survives
  regardless of what happens to the token or account it references.
- A revoked token's id, description, and owner stay resolvable indefinitely
  for audit purposes, at the cost of the table never shrinking on its own.
