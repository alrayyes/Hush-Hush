# Proposal

## Why

`used_by` is a free-text field: creating or editing a secret means typing
consumer names from memory, with no way to see what's already recorded
elsewhere or avoid creating a near-duplicate (`homelab/vps-docker` vs
`homelab-vps-docker`) by typo. A picker that shows existing consumers and
still lets a new one be added removes both problems at once.

## What Changes

- Add a read endpoint returning every distinct consumer name currently
  recorded across all objects' `used_by` lists.
- Replace the free-text `used_by` input on the secret create/edit form
  with a combobox: typing filters the existing consumer list, and
  entering a value with no match offers "Add `<value>`" as the last
  option, which behaves exactly like selecting an existing one.
- No change to how `used_by` is stored (still a list of strings on the
  object) or to `GET /objects`'s existing `used_by` filter.

## Capabilities

### New Capabilities

- `consumers`: adds a read endpoint listing every distinct consumer name
  in use, and the combobox UI built on it.

### Modified Capabilities

_None._

## Impact

- `internal/api/`: new `GET /consumers` handler and route, backed by a new
  store query.
- `internal/store/`: a query returning distinct `used_by` values across
  all objects.
- `api/openapi.yaml`: new `/consumers` path and response schema.
- `cmd/hush-hush/web/src/routes/(app)/+page.svelte`: `used_by` input on
  the create/edit form becomes a combobox instead of a free-text field.
