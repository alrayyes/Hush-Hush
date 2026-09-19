# 16. Track a verified actor separately from the self-reported caller label

## Status

Accepted. Extends [ADR 8](0008-unauthenticated-caller-header.md) rather
than replacing it.

## Context

Once sessions and tokens became real, verifiable identities (rather than
the v1 world of one shared write token and an optional self-reported
`X-Caller` header), the audit log needed to represent "who actually
authenticated this request" - and the existing `caller` field was never
meant to carry that weight.

## Decision

New `actor_type` (`token`/`session`, nullable) and `actor_id` columns on
`audit_log`, kept separate from the existing `caller` column rather than
overwriting it.

Researched against verified-vs-self-reported audit field conventions
(Pangea, Cloudflare Audit Logs v2, evlog.dev). Collapsing a verified
identity and an unverified, self-reported one into a single field is the
antipattern those sources call out directly - a caller can already put
anything in `X-Caller` ([ADR 8](0008-unauthenticated-caller-header.md)),
and losing the distinction once a real actor exists would make the whole
field less trustworthy, not more.

This revises `auth/spec.md`'s originally planned "session overwrites
caller" behaviour (written before this requirement existed) to attribute
via the new `actor_type`/`actor_id` pair instead. No foreign key to
`write_tokens`, the same reasoning already used for the audit log having
no foreign key to objects: the entry has to survive the token or account
state it references changing - see [ADR
17](0017-token-revocation-soft-delete.md).

## Consequences

- The audit log now carries two distinct kinds of identity per entry: an
  unverified, self-reported `caller`, and a verified `actor_type`/`actor_id`
  pair when the request was authenticated by a token or session.
- A reviewer investigating an incident can tell the difference between "the
  caller claimed to be X" and "the request was actually authenticated as
  X" - the whole point of keeping them separate.
- Any future credential type that authenticates a request needs to populate
  `actor_type`/`actor_id` the same way, not invent a third field.
