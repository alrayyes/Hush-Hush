# 26. Let a write bearer token manage consumer read tokens

## Status

Accepted. Adds to [ADR 24](0024-consumer-scoped-read-tokens.md), not a
reversal of it - ADR 24 settled what a consumer read token is and what it
authorizes; this only widens who can create and manage one over HTTP.

## Context

Every `/consumer-tokens*` operation (create, list, revoke, rotate, purge)
shipped session-only, the same as the pre-existing `/tokens*` write-token
endpoints they were modelled on. That left no way for a caller with no
browser - a CLI, a script, a CI job - to mint or manage a consumer token
over HTTP at all: `hush-hush-cli` explicitly scoped this out of its own
consumer-read-token support for exactly this reason
([hush-hush-cli#133](https://github.com/alrayyes/hush-hush-cli/issues/133)),
and [issue #467](https://github.com/alrayyes/hush-hush/issues/467) asked
for the gap closed so it eventually could.

## Decision

`POST /consumer-tokens`, `GET /consumer-tokens`, `DELETE
/consumer-tokens/{id}`, `POST /consumer-tokens/{id}/rotate`, and `DELETE
/consumer-tokens/{id}/purge` now accept a valid write bearer token as an
alternative to an admin session, the same `requireWriteAccess` gate
`/objects` and `/consumers` already use rather than a separate
session-only path.

This is a narrowing delegation, not an escalation: a write bearer token
already reads and writes any object, unrestricted. A consumer token it
mints is strictly less powerful than the write token that minted it - read
only, and only for the one consumer it names. That's the opposite of
`/tokens` (write-token management), which stays session-only on purpose -
minting a _write_ token from a write token would be the actual escalation,
and is also the bootstrap problem `bearerAuth`'s own spec description
already calls out as the reason write tokens are issued by direct store
access instead of over HTTP at all.

A session-authenticated mutating call still needs its CSRF token
(`csrfTokenOptional` in `api/openapi.yaml`, matching how `POST /objects`
already documents the same either-credential shape); a bearer-token-
authenticated one doesn't, since there's no session to have one.

## Consequences

- A CLI, script, or CI job holding only a write bearer token can now
  mint, list, rotate, revoke, and purge consumer read tokens without ever
  establishing a session - unblocking `hush-hush-cli#133`'s own follow-up.
- `/tokens` (write-token management) is unchanged and stays session-only;
  this decision doesn't touch it.
- A write token's blast radius, if leaked, now also includes "can mint a
  consumer token for any consumer name" - already implied by "can read or
  write any object," since a consumer token only ever narrows that same
  access, never extends it.
- Audit attribution for a bearer-authenticated consumer-token action
  records the write token's id (`actorFrom`'s existing `"token"` actor
  type), the same as any other bearer-authenticated write action - no new
  attribution mechanism needed.
