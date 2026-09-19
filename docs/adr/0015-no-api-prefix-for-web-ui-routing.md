# 15. Keep existing API paths unprefixed; the embedded SPA is the fallback route

## Status

Accepted

## Context

Adding the web UI meant deciding how JSON API routes and the embedded SPA
share the same origin and port. `adapter-static`'s `fallback:
'index.html'` pattern is simplest to reason about when every API route
lives under one common prefix.

## Decision

Existing and new JSON endpoints keep their current paths, with no new
prefix added (`/objects`, `/audit-log`, `/healthz`, and the new `/auth/*`,
`/tokens/*` endpoints). The embedded SPA is served as the fallback handler
for any request that doesn't match a known API route - the Go mux matches
known API routes first and falls through to the static file handler
(serving `index.html` for any unmatched path, the fallback
`adapter-static` expects) otherwise.

Moving everything under a `/api/*` prefix was considered - it's what
`adapter-static`'s fallback pattern would make simplest to reason about -
and rejected: the existing paths are the published contract every
generated SDK (`hush-hush-go`/`-python`/`-node`/`-php`) and `hush-hush-cli`
already call. Moving them under a new prefix would be a breaking change to
every one of those repos for no benefit anyone asked for.

## Consequences

- The fallback-routing logic has to check every known API route before
  falling through to the SPA handler, rather than a single prefix check -
  slightly more routing complexity, contained to one place.
- Every SDK and the CLI keep working against the same paths, unaffected by
  the web UI's existence.
- Any brand-new API surface added later has to pick a path that doesn't
  collide with a plausible SPA route (or vice versa), since there's no
  prefix boundary separating the two namespaces.
