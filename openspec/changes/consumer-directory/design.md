# Design

## Context

`consumer-combobox` adds `GET /consumers` returning every distinct
`used_by` value, unpaginated, with no count and no filter (see that
change's proposal.md and design.md). This change extends the same
endpoint and adds the browsing page on top of it, rather than
introducing a second listing endpoint.

## Goals / Non-Goals

**Goals:**

- A browsable, filterable, paginated view of consumers, each with how
  many secrets reference it.
- Keep `consumer-combobox`'s existing unfiltered/unpaginated call site
  working unchanged.

**Non-Goals:**

- Renaming or merging consumer values - this is a read-only directory,
  not a management page. A near-duplicate consumer name created before
  `consumer-combobox` existed stays a separate entry.
- Any change to how `used_by` is stored or validated.

## Decisions

- **Offset-based pagination with page numbers**, not a cursor. A
  consumer list is bounded by the number of distinct `used_by` values in
  a single secrets store - a small, close-to-static dataset - exactly the
  case offset pagination fits: it needs the total count for page-number
  navigation, and doesn't have cursor pagination's reason for existing
  (avoiding drift under high write volume on a large, frequently changing
  collection). It also matches the "like Google" page-number navigation
  the proposal asks for. That navigation style itself assumes offset
  semantics (`?page=`, not an opaque cursor token).
- **The count is a per-request join/count in the store query**, not a
  denormalized counter column. There's no write path this count needs to
  stay fast on - it's read at directory-page-load time, over a dataset
  small enough that a count join is not a performance concern here.
- **Selecting a consumer reuses `GET /objects?used_by=`** rather than a
  new filtered endpoint - the secrets overview already supports exactly
  this filter (`api/openapi.yaml`'s existing `used_by` query parameter),
  so the directory only needs to navigate to it with that parameter set.

## Risks / Trade-offs

- [Extending `GET /consumers`'s response shape (adding a count, and
  wrapping results for pagination) could break `consumer-combobox`'s
  existing caller] → mitigated by keeping the no-parameters response
  unchanged, both its shape and its content: the combobox's own
  unfiltered call should be checked against this change's actual response
  once both are implemented, and adjusted if the wrapping shape changed.
