# Proposal

## Why

The login page always shows a "Register the first passkey" link, even after
an admin account already exists. The server already refuses anonymous
registration once an admin account exists
(`registrationIsAuthorized` in `internal/api/register.go`,
`auth/spec.md`'s "Registering a first passkey" scenario) - so the link is
misleading rather than a real security gap, but every visitor to an already-
bootstrapped install sees a "register" affordance that will only ever fail
for them. It's the wrong action to keep showing.

## What Changes

- Add an unauthenticated `GET /auth/status` endpoint that reports whether an
  admin account exists yet, using the same admin-existence check the
  registration and login endpoints already apply. It's a boolean, and no
  more of a leak than what `POST /auth/login/begin` already exposes today
  (its "no admin account registered yet" error).
- The login page fetches that status once on load and renders exactly one
  primary action: "Register passkey" when no admin account exists yet, or
  "Log in with a passkey" once one does. The always-visible secondary
  "Register the first passkey" link is removed - registering another
  passkey after bootstrap stays settings' job, behind a real session, same
  as today.
- No change to server-side registration authorization. This is a
  discoverability/UX fix on top of an access control decision the server
  already enforces correctly ([OWASP Top 10 A01: Broken Access
  Control](https://owasp.org/Top10/2025/A01_2025-Broken_Access_Control/) -
  the client-side toggle this change adds is a convenience, not the
  control).

## Capabilities

### New Capabilities

- `auth`: adds a bootstrap-status check the login page uses to decide
  whether to offer "Register passkey" or "Log in with a passkey", so the
  UI never offers registration once an admin account exists. (No main spec
  exists for `auth` yet - `openspec/changes/web-ui/specs/auth/spec.md` is
  still an in-flight delta - so this is declared new here rather than as a
  modification of an archived spec.)

### Modified Capabilities

_None._

## Impact

- `internal/api/server.go`: new `GET /auth/status` route.
- `internal/api/register.go` (or a new `internal/api/status.go`): handler
  reusing the existing admin-existence check.
- `api/openapi.yaml`: new `/auth/status` path and response schema.
- `cmd/hush-hush/web/src/lib/api.ts`: new client call for the status
  endpoint.
- `cmd/hush-hush/web/src/routes/login/+page.svelte`: fetches status on
  load, renders a single primary action instead of a login button plus a
  permanent register link.
