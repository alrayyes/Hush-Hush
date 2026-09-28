# Spec Delta

## Purpose

Lets an authenticated admin manage, from the web UI, the credentials that
authenticate requests to this store - write bearer tokens (existing) and
per-consumer read tokens (this change), without calling the HTTP API
directly.

## ADDED Requirements

### Requirement: Consumer tokens are listed in the web UI

The settings page SHALL show every issued consumer token's consumer name,
description, created-at, expires-at, revoked status, and last-used-at, in
a section separate from bearer tokens.

#### Scenario: Viewing the consumer tokens section

- **WHEN** an authenticated admin loads the settings page
- **THEN** a "Consumer tokens" section lists every issued consumer token
  with its consumer name, description, created-at, expires-at, revoked
  status, and last-used-at (or an indicator that it has never been used)

### Requirement: Creating a consumer token from the web UI

The settings page SHALL let an admin create a consumer token by choosing
a consumer, a description, and a validity period, and SHALL show the raw
token value exactly once, in a copyable field, immediately after
creation.

#### Scenario: Creating a consumer token

- **WHEN** an admin submits a consumer, description, and validity period
  through the create-consumer-token form
- **THEN** the new token appears in the consumer tokens list and its raw
  value is shown once, in a copyable field, and is not shown again after
  the dialog is dismissed

#### Scenario: Choosing the consumer

- **WHEN** an admin opens the create-consumer-token form
- **THEN** the consumer field offers the same autocomplete-and-suggest
  picker used elsewhere in the UI to choose a consumer, constrained to
  selecting exactly one consumer

### Requirement: Rotating a consumer token from the web UI

The settings page SHALL let an admin rotate an active consumer token,
replacing its value while keeping its consumer and description unchanged,
and SHALL show the new raw value exactly once.

#### Scenario: Rotating an active consumer token

- **WHEN** an admin rotates an active consumer token with a new validity
  period
- **THEN** the token's consumer and description are unchanged in the
  list, and the new raw value is shown once, in a copyable field

### Requirement: Revoking a consumer token from the web UI

The settings page SHALL let an admin revoke a consumer token, after which
it SHALL show as revoked in the list immediately, without a page reload.

#### Scenario: Revoking a consumer token

- **WHEN** an admin confirms revoking a consumer token
- **THEN** that token's status shows as revoked in the list, and its
  rotate/revoke actions are no longer offered
