# Spec Delta

## ADDED Requirements

### Requirement: Token last-used tracking

A write bearer token's most recent successful authentication SHALL be
recorded as a last-used-at timestamp, included in that token's metadata
wherever it is listed or returned.

#### Scenario: A never-used token has no last-used time

- **WHEN** a token's metadata is read before it has ever authenticated a
  request
- **THEN** its last-used-at field is absent or empty

#### Scenario: A used token shows its most recent use

- **WHEN** a token successfully authenticates a create, update, delete, or
  list request, and its metadata is read afterward
- **THEN** its last-used-at field reflects that request's time, replacing
  any earlier value

#### Scenario: A revoked token keeps its last recorded use

- **WHEN** a token is revoked after having authenticated at least one
  request
- **THEN** its last-used-at field still reflects the most recent use it
  had before revocation
