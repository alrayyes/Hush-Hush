# hush-hush web UI

The SvelteKit single-page app served by the `hush-hush` binary - login,
secrets overview, and (once `alrayyes/hush-hush#207`/`#215` land) settings
and the audit log page. See the repo root [`README.md`](../../../README.md)
and `openspec/changes/web-ui/` for the full design.

## Requirements

- [bun](https://bun.sh) 1.3.x (pinned via `packageManager` in
  `package.json` - see `rules/javascript.md`'s note on why not 1.4 yet)
- A running `hush-hush` server to develop against - `go run ./cmd/hush-hush`
  from the repo root, with `PUBLIC_URL` set

## Developing

```sh
bun install
bun run dev
```

`bun run dev`'s own server proxies nothing - point a browser at the Go
server's own address (`PUBLIC_URL`) once both are running; every request
goes straight to the Go API via `fetch`, same-origin in production.

## Building

```sh
bun run build
```

Writes the static site to `build/` (adapter-static, SPA fallback mode),
which `cmd/hush-hush/embed.go` embeds into the Go binary. Needed before
running the real binary (`e2e/server.sh`, Docker, goreleaser) so it
serves the actual app rather than the committed `build/index.html`
placeholder - not needed for `go build`/`go test ./...` on their own,
which never read the real content either way. See this directory's own
`.gitignore` for why only that one placeholder file is tracked.

## Styling

Tailwind CSS v4 - utility classes on the markup, not hand-rolled CSS.
`src/app.css` holds the design tokens (`@theme`: `bg`, `surface`,
`text`, `text-muted`, `border`, `border-subtle`, `accent`, `error`,
`warning`, `accent-contrast`, `danger`, `danger-contrast` - each usable as
a `bg-*`/`text-*`/`border-*` utility) plus the one pattern that stays
handwritten because no Tailwind utility covers it: the
responsive-table reflow pattern (`.responsive-table`), under Tailwind's
`@layer` system. A new component reaches for these tokens and
Tailwind's own spacing/radius scale rather than a raw value or a new
scoped `<style>` block; `bun run lint:tailwind` (`@shadcn/lint` flags a
raw color, an arbitrary value, or an inline `style=`) is the check for
that.

## Checking

```sh
bun run check         # svelte-check: types, unused exports, Svelte-aware lint
bun run lint          # biome check: JS/TS/CSS/JSON lint and format check
bun run lint:tailwind # oxlint + @shadcn/lint: Tailwind class usage
bun run test          # vitest: unit tests for src/lib's pure logic
```

### End-to-end and accessibility (Playwright + axe-core)

```sh
bun run build                        # e2e/server.sh runs the built output, not vite dev
bunx playwright install --with-deps chromium  # once, or after a Playwright version bump
bun run test:e2e
```

`e2e/journey.spec.ts` and `e2e/viewport.spec.ts` drive real Playwright
journeys through the login page and every authenticated page area
(secrets overview, consumers, audit log, settings) against
`e2e/server.sh`'s real Go binary - `vite preview` alone returns a 500 for
every route, since the app has no SSR and every page's own `+layout.ts` fetches
`GET /healthz` at load. Login uses a Chromium CDP virtual authenticator to
register a real passkey rather than a stubbed session, the same WebAuthn
ceremony a real browser performs (`rules/a11y.md`). Each page a journey
test reaches also gets an accessibility scan (`@axe-core/playwright`)
against WCAG 2.1 AA (`wcag2a`/`wcag2aa`/`wcag21a`/`wcag21aa`) - any
violation fails the run, same as CI's `e2e` job.

### Performance and best-practices (Lighthouse)

```sh
bun run build
bunx playwright install --with-deps chromium
bun run lighthouse
```

`e2e/lighthouse.ts` runs a real Lighthouse audit (via
`playwright-lighthouse`, reusing the same CDP virtual-authenticator login
as the Playwright journey above) against `/login` and the secrets
overview, checking only the `performance` and `best-practices` categories

- SEO and PWA are skipped outright, not meaningful for a self-hosted
  secrets UI with no public search presence or installable-app ambitions.
  Unlike the axe-core scan above, a score under threshold only warns in the
  terminal and in CI (`rules/browser-compat.md`'s "warn, don't fail the
  build" default for a judgment-call check) - it never fails the command or
  the pipeline.

Both commands need a native Go toolchain on `PATH` (`e2e/server.sh` runs
`go build` directly, not through Docker) - that's also why neither is
part of the `pre-push` git hook, which otherwise never assumes one; see
`lefthook.yml`'s `web-check` comment for the timed decision.

## WebMCP tools

Once logged in, the page declares two [WebMCP](https://webmachinelearning.github.io/webmcp/)
tools for an in-browser agent - `list_objects` and `get_object_metadata`
(`src/lib/webmcp.ts`), both wrapping the same `listObjects()` call the
secrets overview page itself uses. Read-only and metadata-only: neither
tool ever returns a secret's sealed value, nor performs a create,
update, or delete.

This does nothing in a browser without `document.modelContext` - every
browser today, short of Chrome or Edge with `chrome://flags/#enable-webmcp-testing`
manually enabled, since WebMCP is still an origin-trial API with no
token this self-hosted project can practically ship
(`docs/adr/0020-webmcp-tools.md`). Verify the real registration path with
that flag enabled, or the
[Model Context Tool Inspector](https://github.com/beaufortfrancois/model-context-tool-inspector)
extension, against a running `hush-hush` instance you're logged into.
