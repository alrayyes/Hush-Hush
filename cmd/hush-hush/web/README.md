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
which `cmd/hush-hush/embed.go` embeds into the Go binary. Run this before
building/testing the Go module if you've changed anything under `src/` -
see this directory's own `.gitignore` for why `build/` stays committed
for now.

## Styling

Tailwind CSS v4 - utility classes on the markup, not hand-rolled CSS.
`src/app.css` holds the design tokens (`@theme`: `bg`, `surface`, `overlay`,
`text`, `text-muted`, `border`, `border-subtle`, `accent`, `error`,
`warning`, `accent-contrast`, `danger`, `danger-contrast` - each usable as
a `bg-*`/`text-*`/`border-*` utility) and the two global classes
`bits-ui`'s portal-rendered `Dialog`/`AlertDialog` content needs (`.overlay`,
`.dialog`) plus the responsive-table reflow pattern (`.responsive-table`),
all under Tailwind's `@layer` system. A new component reaches for these
tokens and Tailwind's own spacing/radius scale rather than a raw value or
a new scoped `<style>` block; `bun run lint:tailwind` (`@shadcn/lint`
flags a raw color, an arbitrary value, or an inline `style=`) is the check
for that.

## Checking

```sh
bun run check         # svelte-check: types, unused exports, Svelte-aware lint
bun run lint          # biome check: JS/TS/CSS/JSON lint and format check
bun run lint:tailwind # oxlint + @shadcn/lint: Tailwind class usage
bun run test          # vitest: unit tests for src/lib's pure logic
```
