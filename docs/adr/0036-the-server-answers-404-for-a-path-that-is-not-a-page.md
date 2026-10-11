# 36. The server answers 404 for a path that isn't a page

## Status

Accepted. Refines the fallback of [ADR 9](0009-sveltekit-static-adapter.md) and
[ADR 15](0015-no-api-prefix-for-web-ui-routing.md), which answered every
unmatched path with the app and a 200.

## Context

`/nothing-here` returned `index.html` with a 200, a soft 404. Search engines
flag a 2xx page that reads like an error, and a monitor can't tell a dead link
from a live one. Only the server can set the status, and it can't tell a
client route from a typo without a list of routes.

## Decision

- **The server keeps a list of the web UI's client routes** (`pageRoutes` in
  `internal/api/static.go`) and answers 200 with the app for a path on it, and
  404 for any other path that isn't a file in the build. A `[slug]` segment
  matches one segment, and a trailing slash is ignored.
- **The 404 carries the same `index.html`**, with `Cache-Control: no-cache` and
  the same Content-Security-Policy. The app boots, finds no route and renders
  its own not-found page, so the visitor sees the app's design and the crawler
  sees the status.
- **A missing file under `/_app/` is a plain 404**, not the app. A script that
  isn't there should fail, not parse as HTML.
- **The list is kept by hand, and a test derives the real one.**
  `TestPageRoutesMatchTheWebUIRoutes` walks `src/routes`, turns each
  `+page.svelte` into a pattern the way SvelteKit does, and fails if the two
  differ. Adding a page without a line in the list fails the build.

Rejected: generating the list at build time. It needs a Vite plugin and a file
the Go build must find, so a missing or stale file would 404 every page. A
checked test costs one line per page and fails before it can ship.

Rejected: a separate `404.html`. The app already has to render the error, so a
second document would only repeat `index.html`.

## Consequences

- A new page is two edits: the route and a line in `pageRoutes`.
- An API path that is also a page (`/audit-log`, `/consumers`) is unchanged;
  [ADR 18](0018-audit-log-route-collision-content-negotiation.md)'s dispatch
  runs before this fallback.
- A path a SvelteKit page would never serve, such as `/secrets` on its own, is
  now a 404 in the browser as well.
