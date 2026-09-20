# Tasks

## 1. Design tokens and global styles

- [x] 1.1 Add a global stylesheet with color/spacing/type-scale custom
      properties on `:root`, dark-mode overrides under
      `@media (prefers-color-scheme: dark)` and `:root[data-theme="dark"]`,
      and import it from `src/routes/+layout.svelte`; verify no page's
      hard-coded colors remain by grepping `src/routes` for hex literals.
- [x] 1.2 Rework each existing page's own `<style>` block to mobile-first
      base styles with `min-width` media queries for wider layouts;
      verify at 320px and 1280px viewport widths in a Playwright test with
      no horizontal scroll at 320px. No page actually needed a `min-width`
      breakpoint in the end - every page was already a single centred
      `max-width` column with side padding, which degrades to full-width
      on a narrow viewport with no media query required; the only real
      320px overflow risk was a wide `<table>`, fixed with a
      `.table-scroll` wrapper (`src/app.css`) instead.

## 2. Shared chrome

- [ ] 2.1 Move nav markup from `(app)/+layout.svelte` into
      `src/routes/+layout.svelte`, gated on `checkSession()`, leaving
      `(app)/+layout.svelte` with only its redirect guard; verify with a
      component test that the nav appears with a session and is absent
      without one, on both a route under `(app)` and `changelog`.
- [ ] 2.2 Add an axe-core scan (`@axe-core/playwright`, WCAG 2.1 AA tags)
      to the journey test that already logs in and navigates the app;
      verify it reports zero violations.

## 3. Theme toggle

- [ ] 3.1 Add an inline, render-blocking theme-init script in `app.html`
      that reads `localStorage` then `prefers-color-scheme` and sets
      `data-theme` before first paint; verify no flash of the wrong theme
      in a manual check with each OS preference.
- [ ] 3.2 Add a toggle control in the shared chrome that flips
      `data-theme` and persists the choice to `localStorage`; verify with
      a component test that toggling persists across a simulated reload.

## 4. Changelog rendering

- [ ] 4.1 Add `marked` and `dompurify` as exact-pinned dependencies;
      render the fetched `CHANGELOG.md` text through both before
      `{@html}` in `src/routes/changelog/+page.svelte`; verify with a unit
      test that a heading and a list item in fixture Markdown produce the
      matching HTML tags, and that a script tag in fixture input is
      stripped.

## 5. PWA installability

- [ ] 5.1 Add `@vite-pwa/sveltekit` (`generateSW` strategy), a web app
      manifest with icons, and wire its static-adapter build step; verify
      the built output serves a valid manifest and registers a service
      worker in a Playwright check against the built binary.

## 6. Verification

- [ ] 6.1 `bun run test`, `bun run check`, `bun run lint`, and
      `bun run format:check` pass in `cmd/hush-hush/web`.
- [ ] 6.2 `go build ./... && go vet ./... && go test ./...` pass (static
      asset embedding/build wiring only - no Go behaviour changes
      expected).
- [ ] 6.3 Manually verified against the running binary: nav present on
      changelog/disclaimer/privacy while logged in, absent while logged
      out; theme toggle persists across a reload; changelog renders
      formatted Markdown; the app installs from a mobile browser's "Add to
      Home Screen" prompt.
