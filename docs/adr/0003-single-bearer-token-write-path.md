# 3. Gate writes with a single bearer token, checked on every mutation

## Status

Accepted. Partially superseded in scope by [ADR
14](0014-session-authenticates-secret-object-access.md), which lets a
web-UI session authenticate the same endpoint as an additional, independent
credential.

## Context

Create/update/delete needs some check the read path deliberately doesn't
have. v1 has one writer (the CLI/CI path) and no multi-consumer write
story yet, so the question was how much machinery that one relationship
actually needs.

## Decision

A single bearer token, checked on every create/update/delete call, audit
logged the same as reads.

Signed requests or mTLS were considered and rejected as unnecessary
complexity for a single writer with no multi-consumer write story.

## Consequences

- A leaked token grants full write access to every object; there's no
  per-token scoping in v1. Rotation and audit-log review are the mitigation,
  not prevention.
- Adding a second, independently scoped writer later means extending this
  model (multiple tokens, see the token-management work that followed),
  not redesigning the write path.
- Every write is unconditionally audit logged, which is what makes token
  misuse detectable after the fact.
