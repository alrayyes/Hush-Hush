# 23. Rotate a write token in place, not by revoke-then-reissue

## Status

Accepted

## Context

Before this change, replacing a write token's secret - because it's
leaking soon, or the caller just wants a fresh one on a schedule - meant
two separate calls with no connection between them: issue a brand-new
token, then separately revoke the old one. The new token got its own id
and description, so anything that already referred to the old token by id
(an audit log entry, a deployment's own record of "which token this is")
had to be updated by hand, or was simply left pointing at a now-revoked
id. [Issue #423](https://github.com/alrayyes/hush-hush/issues/423) asked
for a real rotate operation with a caller-chosen TTL instead.

[ADR 17](0017-token-revocation-soft-delete.md) already settled that a
token's id, description, and owner have to stay resolvable after the
token itself stops working, for audit-log attribution. Rotation needs the
same property while the token is still alive: replacing its secret
shouldn't force a new identity on it.

## Decision

`RotateWriteToken(id, ttl)` replaces the `token_hash` and `expires_at`
columns of the existing `write_tokens` row in place - `id`, `description`,
`owner`, and `created_at` are untouched, and `last_used_at` is cleared
back to unset, since the new secret hasn't authenticated anything yet. The
old secret stops authenticating immediately, since only the new secret's
hash is stored afterward.

Rotating a revoked or already-expired token is rejected with the same
`ErrTokenNotFound` a revoke or usage-update against an unknown id
returns, rather than silently reviving it - a rotate response promises
the caller a working new secret, and there's no valid token identity to
hand one to. Reviving a dead token's id would also mean an audit-log
entry from its earlier life and one from after the revival share an id
with two different meanings, which is exactly the ambiguity ADR 17 avoids
for revoke.

Exposed the same way create/list/revoke already are: `RotateWriteToken` in
the store, `POST /tokens/{id}/rotate` over HTTP (session and CSRF gated,
same as revoke), and `hush-hush token rotate <id> --ttl <duration>` on the
CLI.

## Consequences

- An audit-log entry recorded before a rotation still resolves to the
  same token id and description afterward - rotation never breaks
  attribution, the same property revocation already preserves.
- A caller has to already know the token's id to rotate it - there's no
  "rotate by description" lookup, matching how revoke already works.
- Rotating clears `last_used_at`, so "last used" now means "last used
  since the current secret was issued," not "since this id was first
  created." Treated as correct: the two facts answer different questions,
  and the previous secret's own last-used timestamp is meaningless once
  it can no longer authenticate anything.
- No separate "rotation history" is kept - only the current secret's hash
  is ever stored, consistent with a token's raw value never being
  recoverable after issuance either.
