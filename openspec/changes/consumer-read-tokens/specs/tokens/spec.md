# Spec Delta

## Purpose

Lets an authenticated admin session issue and manage the credentials that
authenticate requests to this store - write bearer tokens (existing) and,
as of this change, per-consumer read tokens that scope a consumer to only
its own objects.

## ADDED Requirements

### Requirement: Consumer read token creation over HTTP

An authenticated admin session SHALL be able to create a new consumer read
token by supplying a consumer name, a description, and a TTL, and SHALL
receive the raw token value exactly once, in the creation response only.

#### Scenario: Creating a consumer token

- **WHEN** an authenticated session submits a consumer name, description,
  and TTL to the consumer token creation endpoint
- **THEN** a new consumer token is stored, bound to that consumer name,
  with the given description and an expiry computed from the TTL, and the
  response includes the raw token value

#### Scenario: Raw value is never returned again

- **WHEN** a consumer token is listed or fetched any time after its
  creation response
- **THEN** the raw token value is not present in the response

#### Scenario: Missing or zero TTL is rejected

- **WHEN** a consumer token creation request omits the TTL or supplies a
  non-positive value
- **THEN** the server rejects the request and no token is created

#### Scenario: Missing consumer name is rejected

- **WHEN** a consumer token creation request omits the consumer name
- **THEN** the server rejects the request and no token is created

### Requirement: Consumer read token listing over HTTP

An authenticated admin session SHALL be able to list every consumer
token's metadata - id, consumer name, description, created-at,
expires-at, revoked status, and last-used-at - without exposing any raw
token value.

#### Scenario: Listing consumer tokens

- **WHEN** an authenticated session requests the consumer token list
- **THEN** the response includes every consumer token's id, consumer
  name, description, created-at, expires-at, revoked status, and
  last-used-at, with no raw token value present

### Requirement: Consumer read token rotation over HTTP

An authenticated admin session SHALL be able to rotate a consumer token's
secret in place, replacing its value and expiry while keeping its id,
consumer name, and description unchanged. Rotating a revoked or expired
token SHALL be rejected rather than reviving it.

#### Scenario: Rotating an active consumer token

- **WHEN** an authenticated session rotates a currently valid consumer
  token by id with a new TTL
- **THEN** the token's id, consumer name, and description are unchanged,
  its expiry and raw value are replaced, and the response includes the new
  raw value exactly once

#### Scenario: Rotating a revoked or expired consumer token is rejected

- **WHEN** an authenticated session attempts to rotate a consumer token
  that is already revoked or past its expiry
- **THEN** the server rejects the request and no token is issued

### Requirement: Consumer read token revocation over HTTP

An authenticated admin session SHALL be able to revoke a consumer token by
id, after which that token SHALL no longer authenticate any read request.
Revocation SHALL be a soft-delete - the token's record persists, marked
revoked, rather than being removed.

#### Scenario: Revoking a consumer token

- **WHEN** an authenticated session revokes a consumer token by id
- **THEN** subsequent read requests authenticated with that token's raw
  value are rejected, and the token's listed status shows it as revoked

#### Scenario: A revoked consumer token stays listed

- **WHEN** a consumer token is revoked
- **THEN** it continues to appear in the consumer token list (marked
  revoked) rather than disappearing, and its id, consumer name, and
  description remain readable
