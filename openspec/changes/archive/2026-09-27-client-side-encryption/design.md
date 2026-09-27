# Design

## Context

See proposal.md's "Why" for motivation. The current state this design
starts from:

- The web UI's "New secret" dialog offers two create modes: base64
  "plain text" (explicitly labelled "not encrypted") and paste-in
  ciphertext sealed by hand outside the browser. Neither is real
  client-side encryption; both are removed by this change.
- The consumer directory's `used_by` is a free-text bookkeeping label
  (`consumer-directory`, `consumer-combobox`) with no cryptographic
  meaning - the server holds zero public keys today.
- There is one admin account, modelled implicitly rather than as a table
  row, authenticating via WebAuthn passkeys (`docs/adr/0011-webauthn-
library-choice.md`, `0012-public-url-required-for-webauthn.md`,
  `0013-cookie-session-not-jwt.md`). `/credentials` already supports
  multiple passkeys per account and already refuses to delete the last
  one (`gate-passkey-registration-ui`'s "Credential management").
  Registration and login run entirely server-verified WebAuthn ceremonies
  today - no client-held secret is derived from a passkey.
- `docs/adr/0001-per-object-age-encryption.md` established that the
  server never decrypts and never holds a service-wide keypair; this
  change is an extension of that model to real client-side sealing and
  real per-consumer/per-user public keys, not a reversal of it.
- `objects.id` is today the single, caller-facing identifier used in
  storage, the API path, the CLI, and the audit log.

## Goals / Non-Goals

**Goals:**

- Real client-side age sealing in the browser, replacing both existing
  create modes, sealed to resolved consumer public keys.
- A registered public key per consumer, safe to store server-side,
  resolved automatically by the existing combobox.
- A durable, multi-passkey-resilient escrow of each user's writer
  identity, private key material never touching the server unwrapped.
- A `users` table and `owner_id`/`user_id` foreign keys sufficient to
  support a second user later without further schema change.
- Decoupling `objects.id` (internal) from a new `slug` (external) so a
  slug can be renamed later without a foreign-key cascade.

**Non-Goals (this change):**

- Inviting or creating a second user account - no invite flow, no
  multi-user first-run UI.
- Transferring a secret's ownership between users.
- A "cannot delete a user who still owns secrets" guard - nothing to
  guard against yet, with exactly one user.
- A "Users" settings page beyond showing the single existing account.
- Any change to the unauthenticated read path or the audit log's
  existing shape, beyond `actor_id` being able to disambiguate a user
  going forward.

These are deferred, not rejected - the same spirit as ADR 2 deferring a
per-consumer ACL. The schema and security model already account for more
than one user (`users` table, `owner_id`, `audit_log.actor_id`), so
adding a second user later doesn't require redesigning any of this.

## Decisions

### Client-side sealing library

Use a WASM or pure-JS age implementation (`age-encryption`/`typage`, or
`agewasm`) run in the browser at create/update time, sealing
to the resolved recipient list before the request ever leaves the
client. Alternative considered: keep server-side sealing with the server
holding recipient public keys and doing the age operation itself -
rejected, since that reintroduces a point where the server handles
plaintext, exactly what ADR 1 ruled out.

### Consumer public key: a plain stored field, not a directory service

A consumer's public key is stored as a plain column on the consumer
record (surfaced through the existing listing endpoint), not fetched from
an external key directory or verified against any PKI. Alternative
considered: an out-of-band key-verification step (fingerprint
confirmation, TOFU warnings) - deferred as unnecessary complexity for a
single-admin tool where the admin registering a consumer's key is
generally the same person or team controlling that consumer.

### Escrowed identity: client-generated, server stores only wrapped copies

The user's age keypair is generated client-side once, at first-passkey
registration. The server never sees the private key unwrapped. Each
PRF-capable credential wraps its own copy (via a key derived from
`getClientExtensionResults().prf`), and one copy is also wrapped by the
break-glass recovery phrase. Alternative considered: a server-generated
keypair, encrypted server-side at rest - rejected, since the server
having ever held the private key in the clear (even transiently, even if
only in memory before wrapping) is a materially different trust story
than it never existing there at all.

### Multi-copy wrapping over a single shared wrap

Each credential gets its own independently wrapped copy of the escrowed
identity, rather than one wrapped copy re-wrapped or shared across
credentials. This is what makes "delete one passkey, keep the identity"
true without a re-encryption step: deleting a credential just deletes its
own wrapped copy, and every other credential's copy is already
independently valid.

### Recovery phrase as the PRF fallback, not a required step for everyone

The recovery phrase is generated and shown once regardless of whether the
first passkey supports PRF, since PRF support isn't known to be
universal across authenticators and the user may add a non-PRF passkey
later. Showing it unconditionally, once, at setup is simpler and safer
than trying to detect after the fact whether a phrase still needs
generating.

### Owner-as-recipient is opt-in, not a default

Defaulting the owner's key into every secret's recipient list would make
every secret's confidentiality partially dependent on the admin/owner's
own key lifecycle, even for secrets meant only for a consumer - directly
contrary to the proposal's "Why". Making it an explicit, per-secret
opt-in checkbox keeps that dependency out unless the person creating the
secret deliberately wants a readable copy back.

### `owner_id` is accountability metadata, not an access-control mechanism

Recording who created a secret is useful for audit and, later, for a
delete-guard - but it must not silently grant that owner decrypt access.
Keeping who owns a secret and who can decrypt it as two independent facts
(one row, one opt-in flag) avoids the ambiguity of "ownership implies
read access" leaking into the encryption model.

### `objects.id` (UUID) vs `slug`

`id` becomes an internal, never-addressable UUID; a new `slug` column is
the sole external identifier - URL path, CLI argument, API path. This is
what lets a slug be renamed later (a consumer relocates, a name was a
typo) without cascading through every foreign key that references the
object, since those reference the stable internal id. This split is
orthogonal to user count - it would be worth doing even with a single
user - but is bundled into this change because it touches the same
`objects` migration as `owner_id`.

## Risks / Trade-offs

- [Risk] A user loses every PRF-capable passkey and their recovery phrase
  both → Mitigation: this is an inherent property of client-held key
  escrow, no different in kind from losing a password-manager master
  password; the recovery phrase's one-time, explicit display is the
  designed mitigation, not a guarantee against user error.
- [Risk] Not every authenticator supports PRF, so early adopters may
  register a passkey that turns out not to wrap anything → Mitigation:
  registration surfaces PRF support (or its absence) at registration
  time, and the recovery phrase is always generated regardless, so no
  passkey is a single point of failure for the identity.
- [Risk] Client-side WASM/JS age sealing is new browser-side attack
  surface (a compromised or tampered page could seal to a wrong or
  attacker-controlled recipient) → Mitigation: this is inherent to any
  client-side encryption model and no worse than today's paste-in mode,
  which already required trusting the browser not to leak a pasted
  plaintext value out; out of scope to solve via this change (for
  example, a page-integrity check such as the browser's own SRI
  attribute is a separate, orthogonal hardening step).
- [Risk] Migrating `objects.id` to an internal id while introducing
  `slug` touches every existing object row → Mitigation: `slug` is
  backfilled from each object's current id value as part of the
  migration, so every existing external reference (CLI scripts, stored
  URLs) keeps working unchanged; covered as its own task (tasks.md, step 6) before any consumer of the new field is built.
- [Trade-off] Wrapping the escrowed identity per-credential means N
  wrapped copies stored per user (N = passkey count) rather than one -
  accepted, since it's what makes independent passkey loss recoverable
  without re-encrypting every secret the user can read back.

## Migration Plan

Sequenced in tasks.md, each step its own pull request:

1. Schema migration: `users`, `credentials.user_id`, `objects.owner_id`,
   additive and backward-compatible (existing single account backfilled
   as the first user row; existing credentials backfilled to reference
   it).
2. Consumer public-key field, additive (nullable) - no existing consumer
   loses its entry for lacking one.
3. PRF-based wrapping on registration - additive; existing credentials
   registered before this ships simply have no wrapped copy of the
   escrowed identity until re-registered, since there is no escrowed
   identity to wrap yet either.
4. Client-side sealing replacing both create modes - the breaking change
   the proposal calls out; no dual-write period, since the old modes are
   being removed outright rather than deprecated.
5. Owner-recipient opt-in checkbox - additive.
6. `objects.id`/`slug` split - additive column plus backfill, old id
   values preserved as the initial slug so nothing already depending on
   today's id breaks.
7. One or more ADRs and docs updates recording the shipped model.

Rollback: each step above is reversible independently (drop the additive
column/table, or revert the UI change) since none of them, individually,
removes data the previous state depended on until step 4 (which is
explicitly a breaking removal the proposal already calls out, not an
incidental one).

This change's `secret-objects`, `consumers`, and `auth` spec deltas use
MODIFIED Requirements against capabilities whose own defining changes
(`secrets-object-store`, `consumer-directory`/`consumer-combobox`,
`gate-passkey-registration-ui`) haven't archived yet - `openspec validate
--strict` flags this as an archive-order dependency (INFO, not an
error): those changes need to archive into `openspec/specs/` before this
one can archive cleanly. That's already the natural order, since this
change's implementation (group 4 onward) builds on the create dialog and
consumer combobox those changes ship.

## Open Questions

None - the encryption/identity model, its deferred edges, and the
migration order were settled in the design conversation this document
records.
