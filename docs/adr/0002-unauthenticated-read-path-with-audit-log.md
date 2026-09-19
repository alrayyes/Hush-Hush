# 2. Leave the read path unauthenticated; audit log instead of an ACL

## Status

Accepted

## Context

Every stored object is already confidential by construction ([ADR
1](0001-per-object-age-encryption.md)) - ciphertext without a matching
private key is inert. The open question was whether to add a second layer
of protection on top: a per-consumer access-control list checked before a
fetch is served.

## Decision

`GET` reads are flat and unauthenticated, paired with mandatory audit
logging on every fetch.

A per-consumer ACL was deferred, not rejected outright. For v1 the real
confidentiality boundary is already "who holds the matching key," so a
fetch-time ACL adds an enforcement layer without adding confidentiality.
Audit logging answers the actual operational question an incident needs -
"was this fetched, by whom, when" - more directly than an ACL would on its
own, and without needing a consumer identity model to exist yet.

## Consequences

- Any network-reachable caller can request any object's ciphertext; nothing
  server-side stops the request, only the encryption stops it being useful.
- Detecting misuse is retrospective, off the audit log, not preventive.
- Listing objects doesn't get this same treatment - see [ADR
  7](0007-listing-gated-by-write-token.md) for why enumeration is treated
  differently from a `get` the caller already has an id for.
- Per-consumer read authorization remains explicit deferred scope for a
  later change, not something this design closes off.
