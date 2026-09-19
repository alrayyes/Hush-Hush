# Spec Delta

## Purpose

Ties every recorded create/read/update/delete to a real, verified actor -
the bearer token or session that authenticated it - instead of only the
existing self-reported `X-Caller` label, and makes the resulting trail
queryable by that actor and viewable from the web UI.

## ADDED Requirements

### Requirement: Verified actor attribution

Every audit log entry resulting from a bearer-token-authenticated or
session-authenticated request SHALL record which specific token or
session authenticated it, as a verified actor distinct from the existing,
self-reported `X-Caller` field. An unauthenticated read SHALL have no
actor.

#### Scenario: Token-authenticated write is attributed to that token

- **WHEN** a create, update, or delete is authenticated by a bearer token
- **THEN** the resulting audit log entry's actor identifies that specific
  token by id, independent of and in addition to whatever `X-Caller`
  header the request carried

#### Scenario: Session-authenticated write is attributed to the session

- **WHEN** a create, update, or delete is authenticated by a session
- **THEN** the resulting audit log entry's actor identifies the admin
  account, per `auth/spec.md`'s "Session-attributed writes" requirement

#### Scenario: An unauthenticated read has no actor

- **WHEN** an object is fetched via the unauthenticated read path
- **THEN** the resulting audit log entry records no actor, the same as
  today

### Requirement: Audit log query by actor

`GET /audit-log` SHALL accept a filter restricting results to entries
attributed to a specific actor (token id or the admin account), combining
with the existing object/caller/time filters.

#### Scenario: Filtering by token

- **WHEN** the audit log is queried with an actor filter naming a specific
  token id
- **THEN** only entries authenticated by that token are returned

### Requirement: The web UI's audit log page

The web UI SHALL provide a page listing audit log entries, filterable by
object, actor (token or session), and date range, with the visible page
exportable as CSV or JSON.

#### Scenario: Viewing and filtering the audit log

- **WHEN** the authenticated admin opens the audit log page and applies an
  object, actor, or date-range filter
- **THEN** the listed entries update to match, and each entry shows its
  object, action, verified actor (or "none" for an unauthenticated read),
  the self-reported caller label if any, and timestamp

#### Scenario: Exporting the visible page

- **WHEN** the authenticated admin exports the currently visible audit log
  page
- **THEN** a CSV or JSON file downloads containing exactly the entries
  currently shown, not the full unfiltered log

#### Scenario: Paging through a log with more entries than one page

- **WHEN** the log has more entries than fit on one page and the
  authenticated admin pages forward
- **THEN** the next page picks up exactly where the previous one left off,
  by the previous page's last entry's own id - never by a row offset that
  could shift or double-count a row under a concurrent write
