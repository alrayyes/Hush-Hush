# 5. CI holds its own decrypting key; there's no server-mediated decrypt path

## Status

Accepted

## Context

CI is a first-class consumer, not a special-cased one requiring its own
trust mechanism (see the service's own stated goals). The question was
whether CI, specifically, needed a different access pattern from any other
reader - for example because it commonly needs to consume a value directly
in a pipeline rather than through a long-lived process holding a key.

## Decision

CI is treated exactly like any other consumer: it holds its own age private
key, delivered through whatever secret storage its own CI platform already
provides, and decrypts client-side like everyone else.

The alternative considered was the server decrypting on behalf of a
strongly verified ephemeral CI identity (for example via OIDC), for objects
flagged as CI-bound. Rejected once it became clear CI can simply hold its
own decrypting key - this avoids a second trust model, a special object
"kind," and an OIDC integration that isn't needed once CI just holds a key
like anyone else.

Certificate-based or CA-issued delivery for CI was also considered, for the
one known case where CI must wield a value as raw plaintext directly, and
rejected as unnecessary complexity for a value that's set once and never
rotated.

## Consequences

- No CI-specific code path anywhere in the service - one fewer trust model
  to maintain and reason about.
- A CI platform that can't securely hold a private key at all is out of
  scope; that's a gap in the platform's own secret storage, not something
  this service works around.
- Dynamic/short-lived credential issuance (Vault-style secret generation)
  stays explicitly out of scope - this is a store for existing values, not
  a credential issuer.
