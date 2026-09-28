# 24. Gate `GET /objects/{slug}` with a credential; add consumer-scoped read tokens

## Status

Accepted. Partially supersedes [ADR 2](0002-unauthenticated-read-path-with-audit-log.md),
which left the read path flat-unauthenticated - that ADR's confidentiality
reasoning (an age private key, not a server-side check, is the real
boundary) still holds; this adds an access-control layer on top of it
rather than replacing it.

## Context

ADR 2 explicitly named per-consumer read authorization as deferred scope,
not something the flat-unauthenticated design closed off. Ryan asked for
that gap closed: a consumer should authenticate its own reads, so a
leaked credential exposes only that consumer's own objects rather than
every object in the store ([alrayyes/hush-hush#438](https://github.com/alrayyes/hush-hush/issues/438)).

## Decision

`GET /objects/{slug}` now requires one of: a valid write bearer token, a
valid admin session, or a valid consumer read token whose bound consumer
appears in that object's `used_by` list.

Consumer read tokens are a new, separate credential (`consumer_tokens`
table), modelled on the existing write bearer tokens - hashed at rest, a
random-hex id split from its secret, TTL, rotate-in-place, soft-delete
revoke, `last_used_at` - but bound to exactly one consumer name at
creation, unlike a write token which authorizes any object. Full HTTP
create/list/rotate/revoke parity with write tokens, under a separate
`/consumer-tokens` resource rather than nested under `/tokens` (a nested
`/tokens/consumer/{id}` was tried first; Go's `ServeMux` resolves it
against `/tokens/{id}/rotate` without ambiguity, but `redocly lint`'s
`no-ambiguous-paths` rule flags it since not every router shares Go's
tie-breaking rule, so a separate resource sidesteps the question rather
than depending on one router's resolution order).

A consumer token presented for an object outside its scope gets the same
404 an unknown slug would, not 403 - so it can't be used to enumerate
which other slugs exist, consistent with [ADR 7](0007-listing-gated-by-write-token.md)'s
enumeration concern.

Scoping the token to the consumer itself (rather than, say, per-object)
was chosen because `used_by` already models "which consumers can use
this object," and a consumer already has its own identity in the system
(a `consumers` row, an optional registered public key) - reusing that
existing identity needed no new modelling.

## Consequences

- **BREAKING**: a caller relying on the previously open read path stops
  working the moment this ships. There's no deprecation window - Ryan
  asked for the closed behaviour directly, not a transition period. The
  CLI, the GitHub Action, and every client SDK need a follow-up to start
  sending a credential (alrayyes/hush-hush-cli#133, hush-hush-action#31,
  and one issue per SDK).
- A write bearer token or an admin session still reads any object,
  unrestricted - only the previously anonymous path is closed, not the
  existing writer/admin access.
- Every successful read now has real actor attribution (`token`,
  `session`, or `consumer_token`) in the audit log, where it previously
  had none - a byproduct of requiring a credential at all, not a separate
  design choice.
- A consumer token's scope check runs against data `GetObject` already
  fetches (`used_by`), not a second query - no new read cost per request.
- Hard-delete (purge) of a dead token, for either credential type, is
  separate scope (alrayyes/hush-hush#439) - this decision only adds the
  new credential and the new gate, it doesn't change how a token is
  removed.
