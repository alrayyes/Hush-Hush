# Proposal

## Why

The Consumers page (`/consumers`) shows a "Secrets" count per consumer
but nothing about consumer read tokens, so checking whether a consumer
already has one means leaving that page and scanning a flat, unfiltered
list on the Settings page (`alrayyes/hush-hush#476`).

## What Changes

- Add a "Tokens" column to the Consumers page's table, computed
  client-side from the existing `listConsumerTokens()` call grouped by
  consumer name (0 when a consumer has none).
- The count links to `/settings?consumer=<name>` when greater than 0; a
  consumer with 0 tokens shows plain text with nothing to link to.
- The Settings page reads `?consumer=` and filters the Consumer tokens table
  to that consumer client-side, showing a removable "consumer: `<name>`"
  filter chip (mirroring the audit-log page's existing chip pattern) and
  scrolling to the Consumer tokens section on load.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `tokens`: adds a per-consumer token count and a consumer-scoped filter view
  to the existing consumer-token listing behaviour. `openspec list --specs`
  reports no existing specs in this repo, so this is declared the same way
  every prior consumer-token change here has (`#463`, `#472`) — as the
  `tokens` capability, added fresh alongside this delta.

## Impact

- `cmd/hush-hush/web/src/routes/(app)/consumers/+page.ts` and `+page.svelte`:
  fetch consumer tokens, compute per-consumer counts, add the column.
- `cmd/hush-hush/web/src/lib/consumers.ts`: new href helper alongside
  `secretsOverviewHref`.
- `cmd/hush-hush/web/src/routes/(app)/settings/+page.ts` and `+page.svelte`:
  read `?consumer=`, filter the Consumer tokens table, render the chip.
- No API or Go changes — `GET /consumer-tokens` already returns every token
  with its `consumer` field; both the count and the filter are computed from
  data already fetched.
