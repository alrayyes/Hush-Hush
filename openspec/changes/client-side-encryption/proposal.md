# Proposal

## Why

The web UI's "New secret" dialog only offers a base64-encoded plaintext
mode (labeled "not encrypted") or a paste-in-ciphertext mode requiring age
sealing done by hand outside the browser - there is no real client-side
encryption path today, and the consumer directory (`used_by`) is a free-text
bookkeeping label with no cryptographic meaning, so the UI can't resolve a
selected consumer into an actual recipient. Confidentiality for anything
written through the browser currently depends entirely on an admin already
having sealed the value themselves, outside the app, which the app itself
does nothing to help with or enforce.

## What Changes

- The web UI performs real client-side age encryption at write time,
  sealing to the selected consumers' registered public keys. **BREAKING**:
  the existing "plain text (base64)" and "paste ciphertext" create modes
  are removed.
- Consumers gain a registered public key, stored server-side (safe - it's
  public), resolved automatically when selected via the existing
  combobox. Nothing about a consumer's own private key ever reaches the
  server.
- The admin account becomes a real `users` resource (one row today, ready
  for more than one later): each user holds an escrowed writer identity,
  wrapped separately under each of their registered WebAuthn passkeys via
  the PRF extension, so losing any one passkey (while others remain) never
  strands the identity. A secret's owner is recorded as metadata; including
  the owner's own key as a decrypt recipient is opt-in per secret ("keep a
  copy I can read back"), not a default, so a secret meant only for
  consumers never depends on the admin account's own key lifecycle.
- `objects.id` becomes an internal identifier, decoupled from the
  user-facing slug used in URLs, the CLI, and the API path today.
- Deliberately out of scope for this change: inviting a second user,
  transferring a secret's ownership, and a user-deletion guard - the schema
  and security model account for more than one user existing, but no UI or
  flow for it ships until there's a real second user to build it for.

## Capabilities

### New Capabilities

- `users`: user accounts, each holding an escrowed writer identity wrapped
  per registered WebAuthn credential.

### Modified Capabilities

- `secret-objects`: `id` is decoupled from the user-facing slug; an
  `owner_id` is recorded per secret; the owner's key is an opt-in decrypt
  recipient rather than implicit; the create/update request shape moves
  from an already-sealed value to plaintext sealed client-side against
  resolved recipients.
- `consumers`: each consumer gains a registered public key field, resolved
  into a real age recipient instead of remaining a bookkeeping label only.
- `auth`: WebAuthn credentials are attributed to a user and, on
  registration, additionally wrap a copy of that user's escrowed writer
  identity via the PRF extension.

## Impact

- `internal/store`: new `users` table; `credentials`/`objects` gain
  `user_id`/`owner_id` foreign keys; `objects.id` migrates to an internal
  identifier alongside a new `slug` column.
- `internal/api`: `/consumers` gains a public-key field; `/objects`
  create/update accepts opt-in owner-recipient inclusion; `/credentials`
  registration performs PRF-based wrapping.
- `api/openapi.yaml`: updated request/response shapes for the above -
  reviewed as the design before any handler changes, per this project's own
  spec-first convention.
- `cmd/hush-hush/web`: real client-side age sealing (`age-encryption`/
  `agewasm` or equivalent), replacing the two existing create modes;
  consumer combobox resolves real recipients; passkey registration
  performs PRF derivation and wrapping.
- One or more new ADRs recording the encryption/identity model change,
  alongside the existing `docs/adr/0001-per-object-age-encryption.md`
  and `docs/adr/0002-unauthenticated-read-path-with-audit-log.md`,
  which this extends rather than reverses.
- Cross-repo: `hush-hush-go` (and the other generated SDKs) regenerate from
  the updated spec via the existing dispatch (`alrayyes/Hush-Hush#348`).
  `hush-hush-cli` needs a follow-up to resolve `--used-by` against the new
  consumer public-key field (tracked separately,
  `alrayyes/hush-hush-cli#125`, blocked on this change). `hush-hush-action`
  is unaffected on the read side but should be checked once this lands.
- Tracked in `alrayyes/Hush-Hush#383`.
