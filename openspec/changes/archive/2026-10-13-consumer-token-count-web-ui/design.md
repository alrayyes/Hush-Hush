# Design

## Context

See proposal.md - Why. The Consumers page (`+page.ts`/`+page.svelte`)
and Settings page (`+page.ts`/`+page.svelte`) are separate routes; both
already call `listConsumerTokens()` on the Settings page only. The directory
currently has no consumer-token data at all.

## Goals / Non-Goals

**Goals:**

- Decide where the consumer-token grouping/counting logic lives.
- Decide the query-parameter and filter-chip mechanism for the Settings page.
- Decide how "scroll to the Consumer tokens section" is implemented.

**Non-Goals:**

- No API or schema change - `GET /consumer-tokens` already returns each
  token's `consumer` field, which is all this needs.
- No change to the existing "Secrets" column or `used_by` filtering.

## Decisions

- **Counting lives in `consumers/+page.ts`.** The load function already
  calls `listConsumersPage(...)`; add a `listConsumerTokens()` call
  alongside it (`Promise.all`, matching the existing pattern on the Secrets
  page and Settings page) and reduce the result into a `Map<string, number>`
  passed to the page as `tokenCounts`. Keeping the reduce in the load
  function (not the component) matches how `attributionByObject` is
  precomputed for the Secrets page rather than done inline in the template.
- **New href helper in `src/lib/consumers.ts`**:
  ``tokensHref(consumer: string): string { return `/settings?consumer=${encodeURIComponent(consumer)}`; }``,
  alongside the existing `secretsOverviewHref`. Same shape, same file, so
  both directory-table link helpers stay together.
- **Filtering lives in `settings/+page.svelte`, not `+page.ts`.** Unlike the
  Secrets page's `used_by`, which the API accepts as a server-side filter,
  there's no server-side consumer filter for tokens - `+page.ts` still
  fetches the full `listConsumerTokens()` result unconditionally, and
  `+page.svelte` derives a filtered view from it with a `$derived` over
  `page.url.searchParams.get('consumer')`. This avoids a second, differently
  shaped request-flow for the one page whose API happens not to support
  server-side filtering, and keeps `+page.ts` unchanged.
- **Filter chip mirrors the audit-log page's existing chip markup** (a
  removable `<li>`/badge with an "×" control), reading and clearing the
  `consumer` query parameter the same way audit-log already reads and clears
  its own filters (`goto` with the parameter stripped, `replaceState: true` so
  clearing doesn't add a back-button entry).
- **Scroll-on-load** via a `$effect` in `settings/+page.svelte` that runs
  once when `page.url.searchParams.has('consumer')` on mount, calling
  `document.getElementById('consumer-tokens').scrollIntoView()` - the
  Consumer tokens section already has a heading that can take that id.
  No new library; SvelteKit's own hash-based scroll handling isn't used
  since the parameter carries the consumer name, not an anchor.

## Risks / Trade-offs

- Computing the per-consumer count client-side means the Consumers page
  now also fetches every consumer token on every load, not just secret
  counts. The existing Settings page already pays this same cost, and
  consumer-token volumes here are small (personal-project data store, not a
  fleet), so this is an acceptable trade against a server-side count
  endpoint that isn't needed anywhere else.
- A stale `?consumer=` for a name that no longer exists (renamed or deleted
  consumer) → filters to zero rows with the chip still showing that name.
  Acceptable: the chip's remove control clears it, and this can only happen
  by following a stale bookmarked link, not through normal in-app
  navigation.
