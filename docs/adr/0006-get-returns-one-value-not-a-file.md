# 6. `get` returns exactly one decrypted value to stdout, never an assembled file

## Status

Accepted

## Context

A real consumer's deploy tooling already has its own mapping of which
secret goes where, in what shape, at what file permissions. The CLI's `get`
command needed a scope decision: does it stay a single-value primitive, or
grow into something that assembles multiple secrets into a delivery
artifact (for example, a `.env` file)?

## Decision

`get` returns exactly one decrypted value to stdout per call. No multi-object
assembly of any kind lives in this service or its CLI.

An assembled `.env`-style output covering multiple objects at once was
considered and rejected after input from a real consumer's deploy tooling:
that tooling's own mapping is the single source of truth for which secret
goes where, in what shape, at what file permissions. An assembled file from
this service would create a second, competing source of truth for exactly
that mapping.

## Consequences

- Multi-secret delivery is entirely the calling tooling's job - scripting
  `get` in a loop, or building a `.env` from several calls, is on the
  consumer side.
- This keeps the CLI's contract simple and composable rather than growing
  a competing assembly feature over time.
- A future request for "give me a file with all my secrets" is a rejected
  alternative here, not an oversight - the reasoning above is what a later
  proposal for it needs to address, not just override.
