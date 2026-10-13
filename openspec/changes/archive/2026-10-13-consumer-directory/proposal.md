# Proposal

## Why

`consumer-combobox` lets an admin pick or add a consumer while editing a
secret, but there's still no way to see the consumer list on its own:
which consumers exist, how many secrets each is tied to, or to find one
among many by name. As the number of distinct consumers grows, that's
increasingly the more useful view - a browsable, filterable directory
rather than only ever meeting a consumer name while editing one specific
secret.

## What Changes

- Add a "Consumers" page listing every distinct consumer name, each with
  a count of the secrets whose `used_by` includes it, paginated with
  numbered pages (page-number navigation, like a typical search results
  list) and filterable by a name substring.
- Extend `GET /consumers` (added by `consumer-combobox`) with pagination
  and filter query parameters, and add the per-consumer secret count.
  `consumer-combobox`'s own unfiltered, unpaginated use of the endpoint
  (populating the create/edit combobox) keeps working: pagination and the
  filter are both optional parameters, so the endpoint returns the
  existing full listing when neither is given.
- Selecting a consumer from the directory navigates to the secrets
  overview pre-filtered to that consumer, reusing `GET /objects`'s
  existing `used_by` query parameter.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `consumers`: `GET /consumers` gains pagination, a name filter, and a
  secret count per consumer; a new directory page is added to the
  authenticated app. (No main spec exists for `consumers` yet - it's
  `consumer-combobox`'s own still-in-flight delta - so this is declared
  as an addition here, building on that pending change, rather than a
  modification of an archived requirement. This change depends on
  `consumer-combobox` landing first.)

## Impact

- `internal/api/`: `GET /consumers` gains `page`/`page_size`/`q` query
  parameters and a per-consumer count in its response.
- `internal/store/`: the distinct-consumer query gains pagination,
  filtering, and a join/count against objects.
- `api/openapi.yaml`: `/consumers` response and parameters updated.
- `cmd/hush-hush/web/src/routes/(app)/consumers/`: new page.
- `cmd/hush-hush/web/src/routes/(app)/+layout.svelte` (or the shared
  chrome from `web-ui-design-system`, whichever lands first): new nav
  link to the consumers page.
