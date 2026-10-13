# 38. The web UI is installable and caches only its shell

## Status

Accepted.

## Context

Used on a phone, the web UI opened as a browser tab with browser chrome and
couldn't be added to the home screen as an app. Browsers offer that when a page
has a web app manifest with icons and a service worker.

A service worker sits in front of every request the page makes, and this app's
requests carry secrets: a sealed value, a token's metadata, an audit entry. A
worker that cached what it fetched would write those into a store the browser
keeps on disk, past logout and past a revoked token.

## Decision

- **The app ships a manifest, three icons and a service worker.** The manifest
  is `static/manifest.webmanifest`, served as `application/manifest+json`
  because Go's built-in MIME table has no entry for the extension. The icons
  are rendered from SVG sources by `scripts/generate-icons.sh` and committed, so
  the build needs no image tooling. `theme-color` follows the OS color scheme.
- **The service worker caches the app shell and nothing else.** It stores the
  build's own files and the static assets, matched by exact path, and answers
  those from the cache. Every other request is left alone: it goes to the
  network and its response is never stored. There is no allow-list of API paths
  to forget to extend, because the default is "not cached".
- **It's SvelteKit's own service worker support**, in `src/service-worker/`. The
  cache is named for the build, so a release starts clean and drops the old one.
- **Page loads aren't cached, so the app doesn't work offline.** An installed
  app needs the server for its first paint, as the page needs `/healthz` to
  load at all.

Rejected: `@vite-pwa/sveltekit`. It brings a runtime-caching
config whose defaults would have to be switched off one by one, to buy
offline support this app can't use. SvelteKit's worker is forty lines that read
as a whole.

Rejected: caching API reads for offline use. A secret list is what an attacker
with the device wants, and a stale one misleads. If offline reads are ever
wanted, they get their own decision, with a story for logout and revocation.

## Consequences

- A new static asset is shell automatically. A new route is not, so it can't
  leak into the cache by being added.
- `Content-Security-Policy` needed no change: the worker script is same-origin
  and falls under `script-src 'self'`.
- A user installing the app gets no offline mode, only the window and the icon.
- Changing the icon means editing an SVG and running the generator script, then
  committing the PNG files.
- A new release's worker takes over open tabs at once and deletes the old
  cache. A tab still running the previous release's JavaScript that then loads a
  chunk it hadn't fetched yet gets a 404 and reloads. A redeploy already does
  this, since the old hashed files are gone from the server, so the worker
  doesn't add a new failure.
- `theme-color` and the manifest colors follow the OS color scheme, not the
  in-app theme toggle. Someone who picks dark on a light OS sees light window
  chrome and a light splash screen.
