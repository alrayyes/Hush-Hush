# Spec Delta

## Purpose

Adds two pieces of user-facing behaviour that ride along with the
shadcn-svelte component migration: filtering the audit log by picking a
known object/actor/caller instead of typing one, and creating a consumer
directly from the consumer directory.

## ADDED Requirements

### Requirement: Audit log filters are populated from known values

The audit log page's object-id and actor/caller filters SHALL be
selectable from values that actually appear in the audit log, not
free-text input.

#### Scenario: Selecting a known object id filters the log

- **GIVEN** an audit log containing entries for objects "a" and "b"
- **WHEN** an admin selects "a" from the object-id filter
- **THEN** the log shows only entries for object "a"

#### Scenario: An unauthenticated read's actor is selectable as "none"

- **GIVEN** an audit log containing an entry with no authenticated actor
- **WHEN** an admin opens the actor filter
- **THEN** "none" is one of the selectable options, and selecting it
  filters to entries with no actor

### Requirement: A consumer can be created without an existing secret

An admin SHALL be able to add a consumer to the directory directly from
the consumers page, without first referencing it in any secret's
`used_by` list.

#### Scenario: Adding a new consumer

- **GIVEN** the consumer directory has no consumer named "homelab"
- **WHEN** an admin adds a consumer named "homelab"
- **THEN** "homelab" appears in the directory with a secret count of 0

#### Scenario: Adding a consumer that already exists is rejected

- **GIVEN** the consumer directory already has a consumer named "homelab"
  (whether added directly or via a secret's `used_by` list)
- **WHEN** an admin attempts to add "homelab" again
- **THEN** the request is rejected with a clear error and no duplicate is
  created

#### Scenario: A consumer added directly can be renamed and deleted

- **GIVEN** a consumer added directly with zero secrets using it
- **WHEN** an admin renames or deletes it
- **THEN** the rename or delete succeeds the same as for any other
  consumer
