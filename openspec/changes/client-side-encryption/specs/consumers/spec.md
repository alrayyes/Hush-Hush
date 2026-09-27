# Spec Delta

## MODIFIED Requirements

### Requirement: Distinct consumer listing

A caller with write access (bearer token or session, same as `/objects`)
SHALL be able to list every distinct consumer name currently present in
any object's `used_by` list, sorted, with no duplicates, each entry
including that consumer's registered age public key when one has been
set.

#### Scenario: Listing reflects recorded consumers

- **WHEN** a caller with write access requests the consumer list
- **THEN** the response includes exactly the distinct consumer names
  currently present across every object's `used_by` list, and no others

#### Scenario: A consumer used by multiple objects is listed once

- **WHEN** two or more objects record the same consumer in their
  `used_by` list
- **THEN** that consumer name appears exactly once in the listing

#### Scenario: Listing includes a registered public key

- **WHEN** a consumer with a registered public key is included in the
  listing
- **THEN** that consumer's entry includes its public key alongside its
  name

### Requirement: Secret form offers existing consumers and accepts a new one

The secret create/edit form's `used_by` input SHALL let the admin pick
from existing consumer names and SHALL also accept a value that matches
none of them, treating it as a new consumer to record. Selecting a
consumer with a registered public key SHALL resolve it to that public key
as a real age recipient for client-side sealing; selecting or adding a
consumer with no registered public key SHALL NOT resolve to any recipient.

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

#### Scenario: Picking a consumer with a registered public key resolves a recipient

- **WHEN** the admin selects a consumer whose entry includes a registered
  public key
- **THEN** that public key is added to the set of recipients the secret is
  sealed to

#### Scenario: Picking a consumer with no registered public key resolves no recipient

- **WHEN** the admin selects or adds a consumer with no registered public
  key
- **THEN** no recipient is resolved for that consumer, and the form
  indicates the secret cannot yet be sealed to them

## ADDED Requirements

### Requirement: Consumer public key registration

The consumers API SHALL accept and store a registered age public key for
a consumer, safe to store server-side since it is public, and SHALL
return it alongside a consumer's existing name and count fields. A
consumer's private key SHALL never be sent to or stored by the server.

#### Scenario: Registering a consumer's public key

- **WHEN** a caller with write access registers a public key for a
  consumer
- **THEN** the consumers API subsequently returns that public key
  alongside that consumer's name and count fields

#### Scenario: Consumer with no registered public key

- **WHEN** a consumer has no registered public key
- **THEN** the consumers API returns that consumer's entry with no public
  key field populated, rather than an error
