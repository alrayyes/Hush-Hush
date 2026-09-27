# Tasks

Each numbered group below is sized as its own pull request, per this
account's one-feature-per-PR convention. None of this implementation
work is part of the pull request that adds these planning artifacts;
it starts once this proposal's PR merges.

## 1. `users`/`credentials.user_id`/`objects.owner_id` schema migration

- [ ] 1.1 Add a `users` table (UUID primary key) and backfill one row for
      the existing admin account; verify with a migration test asserting
      exactly one user row exists post-migration
- [ ] 1.2 Add `credentials.user_id` (foreign key to `users`), backfilled
      to the single existing user; verify existing credential-listing and
      login tests still pass unchanged
- [ ] 1.3 Add `objects.owner_id` (foreign key to `users`, nullable during
      backfill, backfilled to the single existing user); verify a test
      asserting every pre-existing object has a non-null `owner_id` after
      migration
- [ ] 1.4 Document the new tables/columns in `internal/store`'s existing
      schema documentation; verify by re-reading it against the actual
      migration file

## 2. Consumer public-key field + API

- [ ] 2.1 Add a nullable public-key column to the consumer record;
      verify with a store-layer test round-tripping a consumer with and
      without a key
- [ ] 2.2 Extend the consumer listing endpoint and `api/openapi.yaml` to
      return the public key alongside name/count; verify with an API
      test asserting the field's presence/absence matches requirement
      `consumers`'s "Listing includes a registered public key" /
      "Consumer with no registered public key" scenarios
- [ ] 2.3 Add a way to register/update a consumer's public key through
      the API; verify with an API test covering the "Registering a
      consumer's public key" scenario
- [ ] 2.4 Regenerate `hush-hush-go` (and other generated SDKs) from the
      updated spec via the existing dispatch (`alrayyes/Hush-Hush#348`);
      verify the SDK's generated client compiles and its own test suite
      passes

## 3. PRF-based credential wrapping on registration

- [ ] 3.1 Detect `getClientExtensionResults().prf` support during the
      WebAuthn registration ceremony; verify with a browser/unit test
      covering both a PRF-capable and a non-PRF-capable simulated
      authenticator
- [ ] 3.2 On first registration, generate the user's escrowed writer
      identity client-side and display its break-glass recovery phrase
      exactly once; verify with a UI test asserting the phrase is shown
      once and never re-displayed or requested from the server afterward
- [ ] 3.3 Wrap a copy of the escrowed identity using the derived PRF
      secret and store it against the new credential; verify with a
      store-layer test asserting one wrapped copy per PRF-capable
      credential, and none for a non-PRF credential
- [ ] 3.4 Verify deleting one of several credentials leaves the identity
      recoverable through any surviving credential's wrapped copy (a
      test exercising requirement `users`'s "Deleting one of several
      passkeys does not strand the identity" scenario)

## 4. Client-side age sealing in the web UI, replacing the two old create modes

- [ ] 4.1 Add a client-side age sealing library (`age-encryption`/typage
      or `agewasm`) to `cmd/hush-hush/web`'s dependencies, pinned exact
      per this account's dependency-pinning convention; verify
      `bun install` succeeds and the lock file is committed
- [ ] 4.2 Remove the "plain text (base64)" and "paste ciphertext" create
      modes from the "New secret" dialog; verify with an updated e2e
      journey test (`cmd/hush-hush/web/e2e/journey.spec.ts`) asserting
      neither mode is reachable
- [ ] 4.3 Implement client-side sealing against the resolved consumer
      recipients (built on group 2's public-key field) at create and
      update time; verify with an e2e test creating a secret and
      confirming the stored value is genuine ciphertext the test can
      decrypt with the matching consumer private key
- [ ] 4.4 Update `ARCHITECTURE.md`'s write-path description to reflect
      real client-side sealing; verify by re-reading it against the
      shipped behavior

## 5. Opt-in owner-recipient checkbox

- [ ] 5.1 Add the owner-recipient opt-in field to the create/update
      request shape and `api/openapi.yaml`; verify with an API test
      covering requirement `secret-objects`'s "Owner opts in to keep a
      readable copy" / "Owner recipient is not the default" scenarios
- [ ] 5.2 Add the checkbox to the "New secret" dialog, wired to include
      the owner's own public key as an additional sealing recipient when
      checked; verify with an e2e test asserting the owner's escrowed
      identity can decrypt an opted-in secret and cannot decrypt one that
      wasn't opted into

## 6. UUID/slug split on `objects`

- [ ] 6.1 Add a `slug` column to `objects`, backfilled from each existing
      row's current id value; verify with a migration test asserting
      every pre-existing object's slug equals its former id
- [ ] 6.2 Point the API path, CLI argument, and audit log's object
      reference at slug instead of the internal id; verify
      with an API test asserting fetch/update/delete all resolve by slug
      per requirement `secret-objects`'s "Internal id decoupled from
      user-facing slug"
- [ ] 6.3 Update `api/openapi.yaml` and regenerate SDKs (same dispatch as
      group 2); verify the CLI's existing tests pass against the new
      path shape
- [ ] 6.4 File the follow-up for `hush-hush-cli` to resolve `--used-by`
      against the new consumer public-key field if not already tracked
      (already opened as `alrayyes/hush-hush-cli#125`); verify the issue
      references this change

## 7. New ADR or ADRs recording the model

- [ ] 7.1 Write an ADR recording the escrowed-identity/PRF-wrapping model
      as an extension of `docs/adr/0001-per-object-age-encryption.md`;
      verify it's added to `docs/adr/` following the existing numbering
      and template
- [ ] 7.2 Write an ADR (or a section of the same one) recording the
      `objects.id`/`slug` split and its rationale; verify it cross-
      references the migration in group 6

## 8. `ARCHITECTURE.md`/`README.md` updates

- [ ] 8.1 Update `ARCHITECTURE.md`'s encryption/identity model section to
      describe `users`, escrowed identities, and consumer public keys as
      shipped; verify by re-reading it against the final implementation
- [ ] 8.2 Update `README.md` wherever it describes the old "plain text"/
      "paste ciphertext" create modes or the implicit single-admin model;
      verify by re-reading it before opening that pull request, per this
      account's own README convention
- [ ] 8.3 Close `alrayyes/Hush-Hush#383`, referencing whichever pull
      request or requests closed each of its acceptance criteria; verify
      all of the issue's acceptance criteria are individually satisfied
      first
