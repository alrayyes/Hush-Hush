## Purpose

Records every read and write against a secret object so a rotation or incident can determine who accessed or changed a given secret and when - the mechanism v1's flat, unauthenticated read path relies on in place of a pre-fetch access check.

## ADDED Requirements

### Requirement: Every operation is logged

The system SHALL record an audit log entry for every create, read, update, and delete call, capturing the object id, a timestamp, and the caller's presented identity if any.

#### Scenario: A fetch is logged

- **WHEN** a get request is served for an object
- **THEN** an audit log entry is recorded with that object's id, the timestamp, and the caller's identity if one was presented

#### Scenario: A write is logged

- **WHEN** a create, update, or delete request is served
- **THEN** an audit log entry is recorded with that object's id, the timestamp, and the caller's identity if one was presented

### Requirement: Audit log is queryable

The system SHALL allow audit log entries to be filtered by object id, by caller identity, and by time range.

#### Scenario: Filtering by object

- **WHEN** the audit log is queried for a specific object id
- **THEN** only entries for that object are returned

#### Scenario: Filtering by caller identity

- **WHEN** the audit log is queried for a specific caller identity
- **THEN** only entries recorded with that caller identity are returned

#### Scenario: Filtering by time range

- **WHEN** the audit log is queried with a start and end time
- **THEN** only entries recorded within that range are returned

### Requirement: Audit log entries are immutable

The system SHALL NOT allow an audit log entry to be modified or deleted once recorded.

#### Scenario: Attempting to alter a recorded entry

- **WHEN** a request attempts to modify or delete an existing audit log entry
- **THEN** the system rejects the request
