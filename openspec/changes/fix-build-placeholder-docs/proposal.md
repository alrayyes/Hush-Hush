# Proposal

## Why

`cmd/hush-hush/web/.gitignore`'s own comment contradicts the repo root
`.gitignore`, `cmd/hush-hush/embed.go`, and `CONTRIBUTING.md` - all three
of which correctly document that only `build/index.html` is a tracked
placeholder for `go:embed`, never real build output. The stale comment
claims the opposite and instructs contributors to run `bun run build`
before committing a frontend change, which is exactly backwards. Filed
as `alrayyes/hush-hush#470` after tracing why this session's own web-ui
PRs (#443, #453, #454, #458, #463, #468) each committed real, hashed
build output as if it were required.

## What Changes

- Rewrite `cmd/hush-hush/web/.gitignore`'s comment to match the already-
  correct explanation elsewhere, dropping the false claims, the stale
  reference, and the wrong instruction.
- Fix `cmd/hush-hush/web/README.md`'s pointer to it.
- Replace the currently committed `build/index.html` (real build output
  from #468) with a genuinely static, minimal placeholder.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

_None. Docs and a static placeholder asset only - no behavior change
(`skip_specs: true`)._

## Impact

- `cmd/hush-hush/web/.gitignore`
- `cmd/hush-hush/web/README.md`
- `cmd/hush-hush/web/build/index.html`
