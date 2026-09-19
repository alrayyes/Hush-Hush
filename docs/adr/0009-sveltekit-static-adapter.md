# 9. Build the web UI with SvelteKit + adapter-static, not bare Svelte + Vite

## Status

Accepted

## Context

The web UI needed a browser-based frontend for an app with several pages
and auth guards on most of them, served by this service's own Go backend
with no separate frontend server or CDN in production.

## Decision

SvelteKit with `@sveltejs/adapter-static`: `export const ssr = false` in
the root `+layout.ts`, `fallback: 'index.html'` in `svelte.config.js` - the
documented pattern for "own backend, static frontend, no SSR." No
`+page.server.js`/`+server.js` files anywhere; every data access goes
through the Go API via `fetch`. Svelte 5 runes (`$state`, `$derived`)
throughout.

Bare Svelte + Vite with a hand-rolled router was considered and rejected:
SvelteKit's file-based routing and `load` functions are worth the small
extra dependency weight for an app with several pages and auth guards on
most of them, and it's still a static build in the end - no SSR server to
run or deploy.

Researched against <https://svelte.dev/docs/ai/overview> and SvelteKit's
own SPA docs. Per `rules/svelte.md`, this being the first Svelte work in
the repo, the project was scaffolded with `npx sv add ai-tools` so
implementation leans on Svelte's own MCP-driven docs workflow and
`svelte-autofixer`.

## Consequences

- Routing, typed `load` functions, and auth-guard structure come from
  SvelteKit's own conventions rather than a hand-rolled router.
- The build output is fully static (see [ADR
  10](0010-go-embed-single-binary.md) for how it's shipped), so there's no
  Node runtime dependency in production despite using a full-featured
  meta-framework at build time.
- Any future server-rendered page would need a different adapter and a
  real Node runtime in production - not something this decision leaves room
  for without revisiting it.
