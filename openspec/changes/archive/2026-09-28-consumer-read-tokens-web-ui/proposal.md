# Proposal

## Why

The backend half of consumer read tokens landed in #438/#446 -
`POST`/`GET /consumer-tokens`, `DELETE /consumer-tokens/{id}`, and
`POST /consumer-tokens/{id}/rotate`, fully documented in
`api/openapi.yaml`. There's no way to manage one from the web UI yet - an
admin has no way to create, view, rotate, or revoke a consumer read token
short of calling the API directly. Filed as `alrayyes/hush-hush#440`.

## What Changes

- Add a "Consumer tokens" section to `/settings`, alongside the existing
  "Bearer tokens" section, listing every issued consumer token (consumer
  name, description, created/expires/last-used, revoked state).
- Add create/rotate/revoke flows for consumer tokens, mirroring the
  existing bearer-token dialogs: a raw value shown exactly once at
  creation and at rotation, and an immediate revoke.
- Add a single-consumer picker to the create dialog, extending
  `ConsumerCombobox` with a `max` prop rather than building a second
  component (see `design.md`).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `tokens`: adds consumer-token management to the web UI, alongside the
  existing bearer-token management the `tokens` capability already
  covers (`openspec/changes/archive/2026-09-28-consumer-read-tokens/
specs/tokens/spec.md` added the backend side of this capability; no
  main spec has been synced from any archived change yet, so this is
  declared as an addition here rather than a modification of an archived
  requirement).

## Impact

- `cmd/hush-hush/web/src/lib/api.ts`: new `ConsumerTokenMetadata`/
  `ConsumerTokenWithValue` types and `listConsumerTokens`/
  `createConsumerToken`/`revokeConsumerToken`/`rotateConsumerToken`
  functions.
- `cmd/hush-hush/web/src/lib/ConsumerCombobox.svelte`: new `max` prop.
- `cmd/hush-hush/web/src/routes/(app)/settings/+page.ts`: loads consumer
  tokens alongside credentials and bearer tokens.
- `cmd/hush-hush/web/src/routes/(app)/settings/+page.svelte`: new
  section, dialogs, and their state.
- `cmd/hush-hush/web/e2e/journey.spec.ts`: new coverage plus an
  axe-core scan of the new section (`rules/a11y.md`).
