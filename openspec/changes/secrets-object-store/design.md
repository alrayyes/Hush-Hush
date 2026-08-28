## Context

See proposal.md - Why. This is a new, standalone service with no prior implementation in this repo, so there is no existing architecture to reconcile with - the constraints below come from the exploration that preceded this change, not from legacy code.

## Goals / Non-Goals

**Goals:**

- The writer's process never handles a private key, at any point.
- The service never computes or persists plaintext, at any point.
- CI is a first-class consumer, not a special-cased one requiring its own trust mechanism.
- `used_by` lineage is a queryable record, not an implied convention.
- v1 substitutes audit logging for a live read-side ACL, deliberately, not by omission.

**Non-Goals:**

- Per-consumer read authorization - deferred to a later change; v1's confidentiality boundary is "who holds a matching private key," not a service-enforced ACL.
- Dynamic/short-lived credential issuance (Vault-style secret generation) - out of scope; this is a store for existing values, not a credential issuer.
- Certificate-based or CA-issued delivery for CI - considered for the one known case where CI must wield a value as raw plaintext directly, and rejected as unnecessary complexity for a value that is set once and never rotated.

## Decisions

**Encryption: age, sealed per-object to one or more recipient public keys chosen at write time.**
Alternative considered: a single universal service keypair, with the server decrypting and re-encrypting per reader. Rejected - it turns the server into a live decryption oracle gated only by request-time logic, a materially larger blast radius than a store that structurally cannot produce plaintext at all. Multi-recipient sealing on a single object covers the case of one secret genuinely shared across several consumers, without needing the server to mediate.

**Read-path: flat, unauthenticated GET, paired with mandatory audit logging.**
Alternative considered: gating fetches behind a per-consumer ACL. Deferred, not rejected outright - for v1 the real confidentiality boundary is already "who holds the matching key," so a fetch-time ACL adds an enforcement layer without adding confidentiality. Audit logging answers the actual operational question an incident needs ("was this fetched, by whom, when") more directly than an ACL would on its own.

**Write-path: a single bearer token, checked on create/update/delete.**
Alternative considered: signed requests or mTLS. Rejected as unnecessary complexity - there is one writer, no multi-consumer write story, and the token is audit-logged on every use the same as reads.

**Storage: SQLite via `modernc.org/sqlite` (pure Go, no cgo).**
Alternatives considered:

- bbolt - rejected because `used_by` lineage, audit-log queries, and shared-secret tracking are all relational access patterns; bbolt would mean hand-rolling secondary indexes for all three instead of using joins.
- Postgres - rejected as an external dependency this scale doesn't need, and one that would conflict with staying free of environment-specific deployment assumptions.
- `mattn/go-sqlite3` specifically was avoided over `modernc.org/sqlite` because it needs cgo, which complicates single-binary cross-compilation.

**CI is a normal consumer; there is no server-mediated decrypt path.**
Alternative considered: the server decrypting on behalf of a strongly-verified ephemeral CI identity (e.g. via OIDC), for objects flagged as CI-bound. Rejected once it became clear CI can simply hold its own decrypting private key, delivered through whatever secret storage its own CI platform already provides - the same pattern as any other consumer. This avoids a second trust model, a special object "kind," and an OIDC integration that isn't needed once CI just holds a decrypting key like anyone else.

**API-first: the OpenAPI spec is the reviewed design surface; CLI and server are built concurrently against it.**
This follows the project's spec-first convention for web services and avoids either side silently drifting from an agreed contract during implementation.

**`get` returns exactly one decrypted value to stdout per call - never an assembled file.**
Alternative considered: an assembled `.env`-style output covering multiple objects at once. Rejected after input from a real consumer's deploy tooling, whose own mapping is the single source of truth for which secret goes where, in what shape, at what file permissions - an assembled file from this service would create a second, competing source of truth for exactly that mapping.

## Risks / Trade-offs

- [Risk] A leaked write bearer token grants full write access (create/update/delete on any object) → [Mitigation] Deliberate v1 simplification; rotate it like any other credential. Every write is audit-logged, giving visibility if it's misused. Scoped write tokens are a natural v2 extension if this proves insufficient.
- [Risk] Flat read access means any network-reachable caller can fetch any object's ciphertext → [Mitigation] Confidentiality holds regardless, since ciphertext without a matching private key is inert. Audit logging closes the operational gap (knowing whether access happened) that a pre-fetch ACL would otherwise cover.
- [Risk] SQLite serializes writes to a single writer at a time → [Mitigation] Acceptable at this scale - one writer, infrequent rotations. Revisit only if usage patterns change materially.
- [Risk] A secret that must be wielded as raw plaintext directly by CI (rather than decrypted by a long-lived process) doesn't fit this model cleanly → [Mitigation] Out of scope for v1; the one known case is a static, never-rotated value and does not require a general solution yet.

## Migration Plan

- Deploy the service, then migrate one real secret end-to-end first to validate the whole path before any broader migration begins.
- Each consuming repo migrates independently and incrementally; those migrations are tracked separately per repo and are not part of this change's rollout.
- No data-layer rollback complexity: this is a new service with no prior state. A consumer whose migration doesn't succeed simply continues using its prior secret-delivery mechanism until it does.
