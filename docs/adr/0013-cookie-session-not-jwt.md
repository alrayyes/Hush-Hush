# 13. Session auth: httpOnly cookie backed by a SQLite table, not a JWT in localStorage

## Status

Accepted

## Context

The web UI needed a session mechanism for its passkey-authenticated admin
account. The frontend is a same-origin embedded SPA ([ADR
10](0010-go-embed-single-binary.md)) - there's no separate API host to
avoid a cookie for.

## Decision

A `sessions` table in the same SQLite database, a `httpOnly`, `secure`,
`SameSite=Lax` cookie with no `Domain` attribute, and the session id
regenerated on every successful login.

Same-origin means no CORS, so `SameSite=Lax` covers most CSRF exposure by
itself; state-changing requests (create/update/delete secret objects,
token management, credential management) additionally require the
session's CSRF token to be echoed back in a header, checked server-side -
covering the gap `SameSite=Lax` leaves open (a cross-site top-level GET
navigation can still carry the cookie).

A JWT in `localStorage` was considered and rejected: vulnerable to
exfiltration via any XSS, and it buys nothing here since there's no
separate API host to avoid a cookie for in the first place.

## Consequences

- Session state is server-side and revocable (deleting the row ends the
  session immediately), unlike a self-contained JWT that stays valid until
  it expires.
- Every state-changing request needs the CSRF header wired through the
  frontend's `fetch` calls, not just the cookie.
- This only holds as designed because the frontend and API are same-origin
  by construction ([ADR 10](0010-go-embed-single-binary.md)) - splitting
  them later would require revisiting this whole decision, not just the
  cookie flags.
