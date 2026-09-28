# Spec Delta

## Purpose

Defines how a caller reads a stored secret object's ciphertext, and what
credential that read now requires.

## ADDED Requirements

### Requirement: Reading an object requires a credential

`GET /objects/{slug}` SHALL require one of: a valid write bearer token, a
valid admin session, or a valid consumer read token whose bound consumer
appears in that object's `used_by` list. A request presenting none of
these SHALL be rejected.

#### Scenario: No credential presented

- **WHEN** a request to `GET /objects/{slug}` carries no bearer token and
  no session
- **THEN** the server rejects the request and does not return the
  object's ciphertext

#### Scenario: Invalid, expired, or revoked credential

- **WHEN** a request to `GET /objects/{slug}` carries a bearer token or
  session that is unknown, expired, or revoked
- **THEN** the server rejects the request and does not return the
  object's ciphertext

#### Scenario: Write token or admin session reads any object

- **WHEN** a request to `GET /objects/{slug}` carries a valid write bearer
  token or a valid admin session
- **THEN** the object's ciphertext is returned, regardless of that
  object's `used_by` list

#### Scenario: Consumer token reads an object in its scope

- **WHEN** a request to `GET /objects/{slug}` carries a valid consumer
  read token bound to consumer `X`, and the requested object's `used_by`
  list includes `X`
- **THEN** the object's ciphertext is returned

### Requirement: A consumer token outside its scope reads as not found

A request to `GET /objects/{slug}` authenticated with a valid consumer
read token, for an object whose `used_by` list does not include that
token's bound consumer, SHALL be rejected the same way an unknown slug is

- not distinguishable from "this object doesn't exist."

#### Scenario: Consumer token requests an object outside its scope

- **WHEN** a request to `GET /objects/{slug}` carries a valid consumer
  read token bound to consumer `X`, and the requested object's `used_by`
  list does not include `X`
- **THEN** the server responds the same way it would for a slug that does
  not exist, not with a distinct "forbidden" response
