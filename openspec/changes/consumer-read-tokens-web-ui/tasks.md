# Tasks

## 1. API client

- [x] 1.1 Add `ConsumerTokenMetadata`/`ConsumerTokenWithValue` types and
      `listConsumerTokens`/`createConsumerToken`/`revokeConsumerToken`/
      `rotateConsumerToken` functions to `src/lib/api.ts`, mirroring the
      existing `TokenMetadata`/`TokenWithValue`/`listTokens`/
      `createToken`/`revokeToken`/`rotateToken` shape (no `owner` field,
      adds `consumer`); verify with `bun run check` that the new exports
      type-check against `api/openapi.yaml`'s `ConsumerTokenMetadata`/
      `ConsumerTokenWithValue` schemas.

## 2. ConsumerCombobox single-select mode

- [x] 2.1 Add the `max?: number` prop from `design.md`: `addConsumer`
      replaces `value` with `[name]` once `value.length >= max`, and the
      input/listbox render only while `!max || value.length < max`.
      No Svelte component-test harness exists in this repo (confirmed
      absent for every prior component change - the design-system
      change's own tasks.md notes this too), so `max={1}` replacing rather than
      appending, and the input hiding once one is chosen, are verified
      through the real e2e create-consumer-token flow in task group 4
      instead, the same substitution every earlier component change in
      this repo made.
- [x] 2.2 Verify every existing call site (`+page.svelte`'s create/edit
      secret dialogs) is unaffected - `bun run test` and a manual check
      that `used_by` still accepts multiple consumers there.

## 3. Settings page: consumer tokens section

- [x] 3.1 `settings/+page.ts` loads `listConsumerTokens()` alongside the
      existing `listCredentials()`/`listTokens()`; verify the page's own
      unit/type coverage picks up the new `PageData` field.
- [x] 3.2 Add the "Consumer tokens" table to `settings/+page.svelte`
      (Consumer, Description, Created, Expires, Last used, Status,
      Actions - same column shape as Bearer tokens), styled with
      Tailwind utility classes only, no new `app.css` rules; verify by
      running the built app and confirming the table matches the
      existing tables' look (density, action-button spacing per
      alrayyes/hush-hush#457's fix).
- [x] 3.3 Add the create-consumer-token dialog: description field, TTL
      field, and the single-select `ConsumerCombobox` (`max={1}`) for
      choosing the consumer; on success, show the raw value once in a
      copyable, read-only field, matching the bearer-token create
      dialog's own "shown once" pattern; verify by creating a token
      through the real UI and confirming the value is never shown again
      after the dialog closes.
- [x] 3.4 Add the rotate dialog (TTL field, shows the new raw value once)
      and the revoke confirmation (`AlertDialog`, immediate), mirroring
      the bearer-token dialogs exactly; verify both against the real UI -
      rotate updates expiry and shows a new value, revoke marks the row
      revoked without a reload.

## 4. Tests

- [x] 4.1 Extend `e2e/journey.spec.ts` (or add a new spec file if the
      journey test is already long enough to warrant splitting) with a
      flow that creates, rotates, and revokes a consumer token through
      the real UI, plus an axe-core scan of the settings page with the
      new section populated (`rules/a11y.md`); verify the test passes
      against a fresh `bun run build`.

## 5. Verification

- [x] 5.1 `bun run lint:tailwind`, `bun run check`, `bun run lint`,
      `bun run test`, and (after a fresh `bun run build`)
      `bun run test:e2e` all pass in `cmd/hush-hush/web`.
- [ ] 5.2 Manually verified against the running binary: the full
      create/rotate/revoke flow for a consumer token, and that the
      existing secret create/edit dialogs' multi-consumer picking still
      works unchanged.
