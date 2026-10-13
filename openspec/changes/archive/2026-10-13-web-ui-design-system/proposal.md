# Proposal

## Why

The web UI has never had a shared design system: there is no global
stylesheet, no design tokens, and no shared page chrome - every page owns
its own ad hoc inline `<style>` block. The practical symptoms are all one
root cause: the changelog page dumps `CHANGELOG.md` as raw text instead of
rendering it, the top navigation only exists inside the authenticated
`(app)` route group so it's absent on changelog/disclaimer/privacy, nothing
adapts to a phone-width viewport, and there's no dark mode or install
support. Fixing these one at a time would mean making the same "what's the
shared layout" decision six times.

## What Changes

- Introduce a shared site chrome (header/nav + footer) used by every page,
  including changelog/disclaimer/privacy and login - not just the pages
  inside the authenticated `(app)` group today. The chrome shows the
  authenticated nav links (Secrets/Audit log/Settings/Log out) only when a
  valid session exists; otherwise it shows the brand and the existing
  footer links with no broken authenticated-only links.
- Add a small set of design tokens (color, spacing, type scale) as global
  CSS custom properties, replacing the scattered per-page hard-coded
  values, and rework every existing page's layout to be mobile-first
  (single-column base styles, wider layouts via `min-width` media
  queries).
- Add a light/dark theme toggle, persisted per visitor, defaulting to the
  browser/OS preference when no preference has been chosen yet.
- Render the changelog page's Markdown content as formatted HTML instead
  of preformatted plain text.
- Make the web UI installable as a PWA (manifest + service worker for the
  prerendered/static output), so it can be added to a phone's home
  screen.

## Capabilities

### New Capabilities

- `web-ui`: adds the shared site chrome, design tokens, mobile-first
  layout, theme toggle, Markdown rendering for the changelog, and PWA
  installability. (No main spec exists for `web-ui` yet -
  `openspec/changes/web-ui/specs/web-ui/spec.md` is still an in-flight
  delta covering the pages themselves - so this is declared new here,
  additive to that pending delta, rather than as a modification of an
  archived spec.)

### Modified Capabilities

_None._

## Impact

- `cmd/hush-hush/web/src/routes/+layout.svelte`: becomes the one shared
  chrome for every route, replacing the split between it and
  `(app)/+layout.svelte`.
- `cmd/hush-hush/web/src/routes/(app)/+layout.svelte`: nav markup moves
  into the shared chrome; this file keeps only the session guard.
- `cmd/hush-hush/web/src/routes/changelog/+page.svelte` and
  `+page.ts`: Markdown parsing added to the load or render path.
- `cmd/hush-hush/web/src/lib/`: new global stylesheet / design tokens, and
  a theme-toggle component with its persistence.
- `cmd/hush-hush/web/src/app.html`, `static/`: PWA manifest, icons, and a
  service worker; every existing page's own `<style>` block is trimmed
  down to what the shared tokens don't already cover.
- New build dependency for Markdown rendering and for PWA asset
  generation (exact packages are a design.md decision).
