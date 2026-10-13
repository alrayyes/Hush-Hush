# Spec Delta

## Purpose

Lets an authenticated admin session permanently remove a token that's
already dead (revoked or expired), on top of the existing ability to
create, list, rotate, and revoke one.

## ADDED Requirements

### Requirement: Purging a dead write token removes it

An authenticated admin session SHALL be able to permanently remove a
write bearer token by id, when that token is already revoked or past its
`expires_at`. The token's row SHALL no longer exist afterward.

#### Scenario: Purging a revoked write token

- **WHEN** an authenticated session purges a write token that has already
  been revoked
- **THEN** the token's row is removed, and it no longer appears in the
  token list

#### Scenario: Purging an expired write token

- **WHEN** an authenticated session purges a write token whose
  `expires_at` has already passed, without ever having been revoked
- **THEN** the token's row is removed, and it no longer appears in the
  token list

### Requirement: An active write token can't be purged

Purging a write token that is neither revoked nor expired SHALL be
rejected, and the token SHALL remain unaffected.

#### Scenario: Purging an active write token is rejected

- **WHEN** an authenticated session attempts to purge a write token that
  is currently valid (unrevoked, unexpired)
- **THEN** the server rejects the request, and the token still
  authenticates requests exactly as before

### Requirement: A purged token's audit history stays queryable

Purging a token SHALL NOT break querying the audit log. An entry
attributed to a since-purged token's id SHALL still be returned, with
that id no longer resolving to a description or owner.

#### Scenario: Querying the audit log after a purge

- **WHEN** the audit log is queried after a token it recorded an entry
  against has been purged
- **THEN** the query succeeds and returns that entry, with the purged
  token's id present but its description and owner absent

### Requirement: Purging a dead consumer token removes it

An authenticated admin session SHALL be able to permanently remove a
consumer read token by id, under the same revoked-or-expired restriction
as a write token.

#### Scenario: Purging a revoked consumer token

- **WHEN** an authenticated session purges a consumer token that has
  already been revoked
- **THEN** the token's row is removed, and it no longer appears in the
  consumer token list

#### Scenario: Purging an active consumer token is rejected

- **WHEN** an authenticated session attempts to purge a consumer token
  that is currently valid (unrevoked, unexpired)
- **THEN** the server rejects the request, and the token still
  authenticates reads exactly as before
