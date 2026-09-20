# Spec Delta

## Purpose

Lets a caller with write access discover every consumer name already in
use, so the secret create/edit form can offer them instead of relying on
free-text recall, while still allowing a genuinely new consumer name.

## ADDED Requirements

### Requirement: Distinct consumer listing

A caller with write access (bearer token or session, same as `/objects`)
SHALL be able to list every distinct consumer name currently present in
any object's `used_by` list, sorted, with no duplicates.

#### Scenario: Listing reflects recorded consumers

- **WHEN** a caller with write access requests the consumer list
- **THEN** the response includes exactly the distinct consumer names
  currently present across every object's `used_by` list, and no others

#### Scenario: A consumer used by multiple objects is listed once

- **WHEN** two or more objects record the same consumer in their
  `used_by` list
- **THEN** that consumer name appears exactly once in the listing

### Requirement: Secret form offers existing consumers and accepts a new one

The secret create/edit form's `used_by` input SHALL let the admin pick
from existing consumer names and SHALL also accept a value that matches
none of them, treating it as a new consumer to record.

#### Scenario: Picking an existing consumer

- **WHEN** the admin selects a consumer already offered by the input
- **THEN** that exact consumer name is included in the submitted
  `used_by` list

#### Scenario: Adding a consumer not yet recorded anywhere

- **WHEN** the admin types a consumer name that matches none of the
  offered options and confirms adding it
- **THEN** that new consumer name is included in the submitted `used_by`
  list, and the object is created or updated with it same as any other
  entry
