# 14. A web-UI session authenticates `/objects` as its own credential

## Status

Accepted. Revises the original `auth/spec.md`, which said a session never
authenticates a bearer-token-gated endpoint - written before the web UI's
"Secrets overview" requirement existed.

## Context

`/objects` is the only bearer-token-gated resource in the service ([ADR
3](0003-single-bearer-token-write-path.md)). The web UI never holds a
bearer token of its own, but the proposal's "Secrets overview: list, view,
create, edit, delete" requirement is impossible without the UI's session
authenticating those calls somehow. #201 had shipped and tested the
original "session never substitutes for a bearer token" behaviour; #203
caught the contradiction while implementing the overview page.

## Decision

A valid session is accepted as a credential on `/objects`, equally valid
to the write bearer token - not routed through it.

An auto-provisioned internal bearer token, minted per session and attached
to the UI's own requests server-side, was considered and rejected: it's
the same access grant with an extra layer of indirection and a second
credential to keep in sync with the session's own lifetime, for no real
gain over accepting the session directly.

The two credentials stay independent otherwise: a session can't read,
derive, or manage a bearer token's value, and expiring or revoking one
never touches the other.

## Consequences

- `/objects` now accepts two credential types instead of one, each
  checked and audit logged independently - see [ADR
  16](0016-audit-log-actor-columns.md) for how the audit log distinguishes
  which one authenticated a given write.
- No hidden token exists anywhere that a session implicitly wields; the
  session itself is the credential the handler checks.
- Any future bearer-gated endpoint has to make the same choice explicitly -
  this decision doesn't generalize "sessions authenticate everything
  bearer tokens do" as a blanket rule, only `/objects` specifically.
