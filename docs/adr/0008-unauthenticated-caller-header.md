# 8. Caller identity for the audit log is a self-reported, unauthenticated header

## Status

Accepted. Extended, not superseded, by [ADR
16](0016-audit-log-actor-columns.md), which adds a separately tracked
_verified_ actor alongside this same self-reported one.

## Context

The audit log's `caller` field was always described as "the caller's
presented identity, if any," but the spec never defined how a caller
presents one - a gap surfaced while implementing audit logging. There is
one shared write token in v1, not per-consumer tokens, so deriving caller
identity from the credential itself carries no distinguishing information,
and the read path has no credential at all.

## Decision

An optional `X-Caller` request header, unauthenticated, is the caller's own
self-reported label.

Deriving caller identity from the bearer token was considered and rejected
for v1: one shared write token carries no caller-distinguishing
information, and the read path has no token to derive anything from
anyway.

## Consequences

- `X-Caller` is a courtesy label, not a verified identity - anyone can send
  any value, or omit it.
- The audit log's operational value comes from correlating id, time, and
  this label during an incident, not from cryptographically proving who
  made the call.
- Once a real verified identity exists (a session, a scoped token), it's
  tracked as a separate, distinctly named field rather than overwriting or
  conflating with this one - see [ADR 16](0016-audit-log-actor-columns.md).
