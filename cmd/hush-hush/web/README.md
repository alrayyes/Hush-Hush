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
bun run generate:api  # regenerate src/lib/api-schema.d.ts and api-limits.ts from api/openapi.yaml
bun run generate:api:check # fail if those generated files are stale (CI and pre-push run this)
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

`e2e/error-pages.spec.ts` checks that a path no route matches answers 404 and
shows the app's own error page. `src/routes/+error.svelte` draws that page and
the 500 one; `src/error.html` is the bare fallback for an error outside the
layout.

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
as the Playwright journey above) against every page of the app: the
login, changelog, disclaimer, privacy and licence pages, then, signed in
and with a few secrets, consumers and tokens seeded, the secrets overview,
a secret's detail page, consumers, the audit log and settings. A test
(`src/lighthouse/pages.spec.ts`) fails when a route has no audit. It
checks only the `performance` and `best-practices` categories

- SEO and PWA are skipped outright, not meaningful for a self-hosted
  secrets UI with no public search presence or installable-app ambitions.
  Unlike the axe-core scan above, a score under threshold only warns in the
  terminal and in CI (`rules/browser-compat.md`'s "warn, don't fail the
  build" default for a judgment-call check) - it never fails the command or
  the pipeline.

The build puts the style sheet inside the HTML (`inlineStyleThreshold` in
`vite.config.ts`), so the first paint waits on no CSS request and Lighthouse's
render-blocking and request-chain insights pass. The cost is that every
document load carries the style sheet, since the browser checks the HTML on
each visit while it caches the hashed CSS file. The threshold is finite on
purpose: if the style sheet outgrows it, the build links a file again and
those insights say so.

Both commands need a native Go toolchain on `PATH` (`e2e/server.sh` runs
`go build` directly, not through Docker) - that's also why neither is
part of the `pre-push` git hook, which otherwise never assumes one; see
`lefthook.yml`'s `web-check` comment for the timed decision.

## Paging a list

`src/lib/Pager.svelte` pages a list that knows its total, and
`src/lib/CursorPager.svelte` pages one read by cursor, the audit log. Both
render nothing when everything fits on one page, so a list passes its own page
size and a short list shows no pager at all. `Pager` takes `page`, `total`,
`pageSize` and `href`, a function from a page number to its URL: each page is a
link, so it survives a reload and can be shared. The numbers to show, the
"Showing 21–40 of 137" range and the clamp for a page past the end are plain
functions in `src/lib/pageMath.ts`.

A page past the end shows the last page, so a stale link never reads
"Showing 381–137".

Secrets and the audit log also let the reader pick how many rows a page holds.
They pass `sizeOptions` and `onSizeChange` (and, for `CursorPager`, `size`), and
the bar then shows a labelled "Rows per page" select. The choice is kept in
the address as `?size=`, and only an offered size counts (`parsePageSize`). The
bar appears once the list outgrows the smallest option, and stays when one page
holds everything, so a size picked too big can be undone. The other lists offer
no choice.

The three Settings lists (passkeys, bearer tokens, consumer tokens) page 10 at
a time and each keeps its own page in the address (`passkey_page`, `token_page`,
`consumer_token_page`), so paging one leaves the others where they are. They
offer no size choice, and a list of 10 or fewer shows no pager. They pass
`keepScroll`, so paging a list low on the page doesn't jump back to the top.

From 640px up the page numbers show. Below that they collapse to Previous,
"Page 2 of 7" and Next, with 44px targets. While `loading`, every control stays
in place, dimmed.

## Installable app

The UI can be added to a phone's home screen or installed from a desktop
browser, and then opens in its own window. That takes
`static/manifest.webmanifest`, the icons in `static/icons/` and the service
worker in `src/service-worker/`.

The worker caches the app shell (the build's files and `static/`) and nothing
else. An API response never reaches a cache, because it can carry a secret's
sealed value or a token's metadata. So an installed app still needs the server,
and there is no offline mode.

The browser's window chrome takes its color from the `theme-color` tag, which
the theme script and the toggle keep in step with the theme the page shows.
The manifest's colors are fixed, so an installed app's splash screen is light.

The PNG icons are rendered from the SVG files beside them. After you edit one,
run `scripts/generate-icons.sh` (it needs `rsvg-convert`) and commit the PNG
files. The build doesn't render them. `e2e/installable.spec.ts` checks the
manifest, the icons, Chromium's installability report and that no API response
is cached.

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
