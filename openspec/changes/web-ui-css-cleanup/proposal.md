# Proposal

## Why

The shadcn-svelte migration (#317-325) replaced hand-rolled dialogs and
most raw form elements with generated components, but
`cmd/hush-hush/web/src/app.css` was never swept afterward: it still
carries CSS that's now dead, plus a few base-layer rules that are only
still load-bearing because a handful of elements never got migrated.
Filed as `alrayyes/hush-hush#442`.

## What Changes

- Migrate `ConsumerCombobox.svelte`'s remove-consumer `<button>` and its
  text `<input>`, and `audit-log/+page.svelte`'s two `datetime-local`
  `<input>` filters, to the shadcn `Button`/`Input` components.
- Remove now-dead rules from `src/app.css`: `.overlay`, `.dialog`
  (`@layer components`), `button.danger`/`button.danger:hover`, and the
  bare `button`/`input, textarea` tag-selector rules in `@layer base`
  (dead once the preceding elements are migrated).
- Update `cmd/hush-hush/web/README.md`'s "Styling" section and
  `cmd/hush-hush/web/AGENTS.md`'s "Styling convention" note to stop
  describing `.overlay`/`.dialog` as required global classes.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

_None. This is a pure refactor - markup and CSS only, with no change to
any externally observable behaviour (`skip_specs: true` set in
`.openspec.yaml`)._

## Impact

- `cmd/hush-hush/web/src/lib/ConsumerCombobox.svelte`
- `cmd/hush-hush/web/src/routes/(app)/audit-log/+page.svelte`
- `cmd/hush-hush/web/src/app.css`
- `cmd/hush-hush/web/README.md`, `cmd/hush-hush/web/AGENTS.md`
