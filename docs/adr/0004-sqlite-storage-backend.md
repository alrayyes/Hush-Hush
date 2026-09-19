# 4. Store state in SQLite via `modernc.org/sqlite`

## Status

Accepted

## Context

The service needs relational access patterns - `used_by` lineage, audit-log
queries, shared-secret tracking - not just key-value lookup, and it ships
as a single distroless binary with no assumed deployment environment.

## Decision

SQLite, through `modernc.org/sqlite` (a pure-Go driver, no cgo).

Alternatives considered:

- **bbolt** - rejected because `used_by` lineage, audit-log queries, and
  shared-secret tracking are all relational access patterns; bbolt would
  mean hand-rolling secondary indexes for all three instead of using joins.
- **Postgres** - rejected as an external dependency this scale doesn't
  need, and one that would conflict with staying free of
  environment-specific deployment assumptions.
- **`mattn/go-sqlite3`** specifically was avoided in favour of
  `modernc.org/sqlite` because it needs cgo, which complicates
  single-binary cross-compilation.

## Consequences

- Writes serialize to a single writer at a time. Accepted as fine at this
  scale (one writer, infrequent rotations); revisit only if usage patterns
  change materially.
- Every new table this service has grown since (sessions, WebAuthn
  ceremonies, credentials) lives in the same database file rather than a
  second store, per this same decision.
- No external database process to deploy, back up, or version alongside
  the binary.
