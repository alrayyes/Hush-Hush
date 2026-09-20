# Design

## Context

See proposal.md - Why. Concretely: `src/routes/+layout.svelte` (root) only
renders the favicon and `Footer`; `src/routes/(app)/+layout.svelte` adds
the authenticated nav and session guard, but only wraps the three pages
under `(app)/`. `login`, `changelog`, `disclaimer`, and `privacy` sit
outside that group and get none of it. There is no global stylesheet
anywhere in `src/` - every page's colors, spacing, and breakpoints are
hard-coded per component. `checkSession()`
(`cmd/hush-hush/web/src/lib/api.ts`) already exists and is what `(app)`'s
guard calls today.

## Goals / Non-Goals

**Goals:**

- One shared chrome, one set of design tokens, applied everywhere.
- Mobile-first base styles for every existing page.
- Dark mode, changelog rendering, and PWA installability on top of that
  shared foundation.

**Non-Goals:**

- A general component library or a third-party UI kit - the page count
  (six) doesn't justify one yet.
- Changing what any page _does_ - this only touches layout, styling, and
  the changelog's rendering, not the settings/secrets/audit-log
  functionality itself.
- Offline-first data access. The service worker (PWA requirement) caches
  static shell assets for installability; it does not cache or serve
  `/objects`, `/audit-log`, or `/auth/*` responses.

## Decisions

- **Root layout becomes the one chrome.** `src/routes/+layout.svelte`
  gains the nav markup currently in `(app)/+layout.svelte`, calls
  `checkSession()` itself, and conditionally renders the authenticated
  links. `(app)/+layout.svelte` keeps only the redirect-when-unauthenticated
  guard (`web-ui/spec.md`'s "Unauthenticated access is blocked"
  requirement, unaffected by this change). This is simpler than adding a
  second, parallel "public chrome" component, and it's the only way
  changelog/disclaimer/privacy/login end up sharing the exact same markup
  as the authenticated pages rather than a close copy that can drift.
- **Design tokens as CSS custom properties on `:root`**, redefined under
  `@media (prefers-color-scheme: dark)` and again under
  `:root[data-theme="dark"]` for an explicit override - the same pattern
  `artifact-design` already documents for artifacts, applied here to a
  real app. Preference cascade on load: `localStorage` choice > OS
  `prefers-color-scheme` > light default, matching the flash-avoidance
  approach in [whitep4nth3r's dark/light toggle
  writeup](https://whitep4nth3r.com/blog/best-light-dark-mode-theme-toggle-javascript/):
  the theme attribute is set from an inline, render-blocking script in
  `app.html` before the page paints, not from a Svelte `onMount` (which
  would flash the default theme first).
- **Changelog rendering: `marked` at runtime, not `mdsvex`.**
  `mdsvex` compiles `.md` _files_ into routes at build time; this page
  fetches `CHANGELOG.md` as a plain-text string at runtime
  (`+page.ts`'s existing `fetch('/CHANGELOG.md')`), so there's no build-time
  file for `mdsvex` to compile - a runtime Markdown-to-HTML parser is the
  right tool for a runtime string, not a build-time page compiler. Output
  is rendered via Svelte's `{@html}` after passing through `DOMPurify`,
  since the string still crosses a network fetch even though the source
  file is trusted repo content.
- **PWA via `@vite-pwa/sveltekit`** rather than hand-writing a manifest
  and service worker: it's the maintained plugin for exactly this
  combination (Vite + SvelteKit), it handles the static-adapter rebuild
  step, and it fits `CLAUDE.md`'s "check for an existing SDK/tool before
  hand-rolling" guidance. `generateSW` (not `injectManifest`) is enough
  here since there's no custom runtime caching logic - installability, not
  offline data access, is the goal (see Non-Goals).

## Risks / Trade-offs

- [Moving nav into the root layout means it runs `checkSession()` on
  _every_ page load, including changelog/disclaimer/privacy, which didn't
  need a session check before] → acceptable: it's the same one lightweight
  call `(app)` already makes, and it's what makes an authenticated
  visitor's nav follow them onto those pages at all.
- [A service worker precaching stale HTML after a deploy] → mitigated by
  `@vite-pwa/sveltekit`'s default `autoUpdate` registration, which checks
  for a new service worker on each load and activates it once the old
  page's clients close.
- [`{@html}` after Markdown parsing is an XSS vector if sanitization is
  skipped or misconfigured] → mitigated by always piping `marked`'s output
  through `DOMPurify` before rendering, never `{@html}` on unsanitized
  output.
