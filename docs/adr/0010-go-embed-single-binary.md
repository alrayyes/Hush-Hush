# 10. Embed the built frontend into the Go binary with go:embed

## Status

Accepted

## Context

The service ships as a single distroless binary with no assumed deployment
environment. Adding a browser UI raised the question of whether that stays
true, or whether the frontend becomes a second deployable served from
somewhere else.

## Decision

The frontend is built to `cmd/hush-hush/web/build/` and embedded into the
Go binary with `go:embed`, served for every non-API-prefixed path.

Serving the SPA from a separate static host/CDN was considered and
rejected: it reintroduces the CORS and separate-deploy complexity
explicitly decided against, and this service already ships as a single
distroless binary - a second deployable would defeat that. The
`Dockerfile` gains a frontend build stage (Node/bun) ahead of the Go build
stage, discarded from the final image the same way the Go build stage
already is (see `rules/go-releases.md`'s multi-arch build guidance).

**Correction caught while implementing #205:** `go:embed` can only reach a
subdirectory of the file that declares the directive - no `../` escapes,
per `go doc embed`. A repo-root `web/build/` is unreachable from any file
under `cmd/` or `internal/`, and this repo has no root-level Go package for
a directive to live in otherwise. The SvelteKit project (and its `build/`
output) lives under `cmd/hush-hush/web/` instead, directly alongside
`cmd/hush-hush/main.go`, the one place in the tree that can declare
`//go:embed all:web/build`. The `all:` prefix matters specifically because
SvelteKit's build output includes a `_app/` directory, and `go:embed`
excludes `_`-prefixed files and directories by default.

## Consequences

- A frontend-only change still requires a full binary rebuild to ship,
  rather than an independently deployable static asset. Accepted per the
  explicit decision to avoid a separate deploy and CORS; frontend
  development iteration still uses SvelteKit's own development server
  against the running Go API, only production builds embed.
- The build directory's location (`cmd/hush-hush/web/`, not repo-root
  `web/`) is load-bearing, not cosmetic - moving it back would silently
  break the embed directive.
- No CDN, no separate origin, no CORS configuration needed anywhere in the
  service.
