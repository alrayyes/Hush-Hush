# Spec Delta

## Purpose

Gives an authenticated admin session an HTTP-reachable way to create,
list, and revoke the same write bearer tokens that were previously only
manageable by running the server binary against its database file
directly, so the web UI's settings page has something to call.

## ADDED Requirements

### Requirement: Token creation over HTTP

An authenticated admin session SHALL be able to create a new write bearer
token by supplying a description and a TTL, and SHALL receive the raw
token value exactly once, in the creation response only.

#### Scenario: Creating a token

- **WHEN** an authenticated session submits a description and a TTL to the
  token creation endpoint
- **THEN** a new token is stored, owned by the admin account, with the
  given description and an expiry computed from the TTL, and the response
  includes the raw token value

#### Scenario: Raw value is never returned again

- **WHEN** a token is listed or fetched any time after its creation
  response
- **THEN** the raw token value is not present in the response

#### Scenario: Missing or zero TTL is rejected

- **WHEN** a token creation request omits the TTL or supplies a
  non-positive value
- **THEN** the server rejects the request and no token is created

### Requirement: Token listing over HTTP

An authenticated admin session SHALL be able to list every token's
metadata - id, description, owner, created-at, expires-at, and whether it
has been revoked - without exposing any raw token value.

#### Scenario: Listing tokens

- **WHEN** an authenticated session requests the token list
- **THEN** the response includes every token's id, description, owner,
  created-at, expires-at, and revoked status, sorted consistently, with no
  raw token value present

### Requirement: Token revocation over HTTP

An authenticated admin session SHALL be able to revoke a token by id,
after which that token SHALL no longer authenticate any write request.
Revocation SHALL be a soft-delete - the token's record persists, marked
revoked, rather than being removed.

#### Scenario: Revoking a token

- **WHEN** an authenticated session revokes a token by id
- **THEN** subsequent create/update/delete requests authenticated with
  that token's raw value are rejected, and the token's listed status shows
  it as revoked

#### Scenario: A revoked token stays listed

- **WHEN** a token is revoked
- **THEN** it continues to appear in the token list (marked revoked)
  rather than disappearing, and its id, description, and owner remain
  readable

#### Scenario: Revoking an already-expired or unknown token

- **WHEN** an authenticated session attempts to revoke a token id that is
  already expired or does not exist
- **THEN** the server responds without error and no other token's state
  changes

### Requirement: Revoked tokens stay attributable

A token, once created, SHALL remain identifiable by its id, description,
and owner for as long as any audit log entry references it - revocation
or expiry SHALL NOT remove that record.

#### Scenario: A revoked token's history stays readable

- **WHEN** an audit log entry recorded before a token was revoked is
  viewed after the revocation
- **THEN** the entry's attributed token still resolves to its real
  description and owner, not a bare id with nothing behind it

### Requirement: Token ownership

A token created over HTTP SHALL record the admin account that created it.
A token created via the existing CLI path (direct database access, no
session involved) SHALL be listed with no owner rather than a fabricated
one. Both forms SHALL appear in the same token list.

#### Scenario: HTTP-created token records its owner

- **WHEN** an authenticated session creates a token over HTTP
- **THEN** the token's stored metadata includes that admin account as its
  owner, and it is present in the token list response

#### Scenario: CLI-created token has no owner

- **WHEN** a token is issued via the existing `hush-hush token issue` CLI
  path
- **THEN** the token appears in the HTTP token list alongside
  HTTP-created ones, with its owner absent rather than guessed
