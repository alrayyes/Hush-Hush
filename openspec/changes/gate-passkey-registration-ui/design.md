# Design

## Context

`internal/api/register.go`'s `registrationIsAuthorized` already refuses
anonymous registration once `len(user.credentials) > 0` - the server side
of this is correct today (see proposal.md - Why). The login page
(`cmd/hush-hush/web/src/routes/login/+page.svelte`) has no way to know
that state, so it always renders both a login button and a "Register the
first passkey" link.

## Goals / Non-Goals

**Goals:**

- Give the client a cheap, side-effect-free way to know whether an admin
  account exists.
- Make the login page's primary action match that state.

**Non-Goals:**

- Changing server-side registration authorization - it's already correct.
- A general "who am I" / session-introspection endpoint. `checkSession()`
  (`cmd/hush-hush/web/src/lib/api.ts`) already covers that for a logged-in
  visitor; this is specifically the pre-login, no-session case.

## Decisions

- **New endpoint: `GET /auth/status`**, unauthenticated, returning
  `{"bootstrapped": bool}`. Alternatives considered:
  - _Reuse `POST /auth/register/begin`'s failure as the signal_ - rejected:
    it has a side effect (starts a WebAuthn ceremony, sets a cookie) every
    time the login page loads, for a check that should have none.
    `svelte.md`'s own steer toward the Svelte MCP tools was checked for a
    framework-native alternative first; there isn't one, since this is a
    server API question, not a Svelte one.
  - _Reuse `POST /auth/login/begin`'s "no admin account registered yet"
    error_ - rejected for the same reason (side effect: starts a login
    ceremony) plus it conflates "checking status" with "attempting to log
    in", which would need its own error-message logic to disambiguate.
  - A dedicated read-only endpoint keeps the check boundary clean, and
    leaks nothing new: `POST /auth/login/begin` already reveals this same
    boolean today via its 400 error body.
- **The check runs server-side, not by asking the store from the
  client.** The client has no direct store access; `handleAuthStatus`
  reuses the same `loadAdminUser` + credential-count check
  `registrationIsAuthorized` already does, so the two can't drift.
- **Client-side gating is UX only, per OWASP A01's "enforce access control
  server-side"** (see proposal.md - What Changes): removing the
  always-on register link doesn't change what the server accepts: a
  crafted `POST /auth/register/begin` after bootstrap still gets rejected
  by `registrationIsAuthorized`, same as before this change.
- **Loading state**: the login page shows neither action until the status
  call resolves, to avoid flashing the wrong one. On a failed status call,
  show an inline error with a retry rather than guessing which action to
  offer.

## Risks / Trade-offs

- [An extra round trip before the login page can render its primary
  action] → acceptable: `GET /auth/status` is a single indexed read, no
  ceremony, no cookie.
- [A visitor's browser has JS disabled or the status call fails] →
  mitigated by the explicit retry state above rather than silently
  defaulting to one action, which could show "log in" to a fresh install
  with no way in, or "register" to an already-bootstrapped one, both
  needing more UI to recover from than repeating the failed check.
