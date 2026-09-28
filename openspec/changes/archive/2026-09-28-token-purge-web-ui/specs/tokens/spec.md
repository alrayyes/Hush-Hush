# Spec Delta

## Purpose

Lets an authenticated admin permanently remove a dead token - write bearer
or consumer read - from the settings page, once soft-deleting (revoke) or
natural expiry has already made it unusable.

## ADDED Requirements

### Requirement: Dead tokens show a "Delete permanently" action

The settings page SHALL show a "Delete permanently" action for a write or
consumer token that is revoked or past its expiry, and SHALL NOT show it
for a token that is still active.

#### Scenario: Revoked token offers permanent deletion

- **WHEN** a token's status is revoked
- **THEN** the settings page shows a "Delete permanently" action for that
  token, and no rotate/revoke actions

#### Scenario: Expired but unrevoked token offers permanent deletion

- **WHEN** a token's `expires_at` is in the past and it was never revoked
- **THEN** the settings page shows a "Delete permanently" action for that
  token, and its status reads as expired rather than active

#### Scenario: Active token offers no permanent deletion

- **WHEN** a token is neither revoked nor past its expiry
- **THEN** the settings page shows only rotate/revoke actions for it, and
  no "Delete permanently" action

### Requirement: Purging warns about audit-log attribution

Confirming "Delete permanently" SHALL show a warning naming the real
consequence - that audit-log entries referencing the token become
unresolvable - before the token is actually removed.

#### Scenario: Confirming a purge

- **WHEN** an admin clicks "Delete permanently" for a dead token
- **THEN** a confirmation dialog states that audit-log entries referencing
  it will show as unresolvable afterward
- **WHEN** the admin confirms
- **THEN** the token is removed from its list immediately, without a page
  reload
