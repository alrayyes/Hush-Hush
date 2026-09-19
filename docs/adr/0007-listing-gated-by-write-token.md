# 7. Gate listing objects behind the write bearer token, unlike every other read

## Status

Accepted

## Context

Every other read path (`get`, `used_by`, the audit log) is deliberately
unauthenticated ([ADR 2](0002-unauthenticated-read-path-with-audit-log.md)),
because the caller already needs to hold an id to use them and
confidentiality rests on holding a matching private key, not on gating
metadata access. Listing doesn't fit that shape: it's the one operation
that lets a caller discover ids it doesn't already have.

## Decision

Listing objects requires the write bearer token, the same credential
gating create/update/delete, unlike every other read path.

Leaving it unauthenticated, consistent with `get`/`used_by`/the audit log,
was considered and rejected: those all need an id the caller already
holds, so the flat-read-path reasoning doesn't extend to a caller
discovering ids in the first place. Enumerating every stored object is a
capability none of the existing unauthenticated reads grant, so it's gated
the same as a write instead.

## Consequences

- A caller with only read access (no bearer token) can fetch a specific
  object it already knows the id for, but can't discover what other
  objects exist.
- This is the one asymmetry in an otherwise flat-read-path design, and it's
  deliberate rather than an inconsistency to "fix" later.
- Any future unauthenticated listing feature needs to re-justify itself
  against this decision, not just add a new unauthenticated endpoint next
  to it.
