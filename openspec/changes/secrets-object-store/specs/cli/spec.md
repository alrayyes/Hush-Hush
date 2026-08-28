## Purpose

Provides the single command-line client that both the writer and every consumer use to create, fetch, update, and delete secret objects, including directly inside CI jobs with no bespoke wrapper.

## ADDED Requirements

### Requirement: inject creates a new object

The system SHALL provide an inject command that seals a given value to one or more configured recipient public keys and creates it as a new object via the service's create endpoint, without the writer's process ever handling a private key.

#### Scenario: Writer injects a new secret

- **WHEN** the writer runs inject with an object id, a value, and a bearer token
- **THEN** the CLI seals the value locally and creates the object, without the writer's process ever handling a private key

### Requirement: get fetches and decrypts one value

The system SHALL provide a get command that fetches an object's ciphertext and decrypts it locally, printing exactly one decrypted value to stdout per call, with no assembled file or consumer-side file-shape logic.

#### Scenario: Consumer fetches a decrypted value

- **WHEN** a consumer runs get with an object id and holds a private key matching one of the object's sealed recipients
- **THEN** the CLI prints the decrypted value to stdout and does not write any file itself

#### Scenario: Consumer lacks a matching private key

- **WHEN** a consumer runs get for an object it cannot decrypt
- **THEN** the CLI reports a decryption failure rather than producing incorrect output

### Requirement: update rotates an existing object's value

The system SHALL provide an update command that seals a new value and replaces an existing object's stored value via the service's update endpoint.

#### Scenario: Writer rotates a value via the CLI

- **WHEN** the writer runs update with an existing object id, a new value, and a bearer token
- **THEN** the CLI seals the new value and replaces the object's stored value

### Requirement: delete removes an object

The system SHALL provide a delete command that removes an object via the service's delete endpoint.

#### Scenario: Writer deletes an object via the CLI

- **WHEN** the writer runs delete with an existing object id and a bearer token
- **THEN** the CLI removes the object from the service

### Requirement: Runs unmodified inside CI

The system SHALL require nothing beyond normal environment or configuration (a decrypting private key for reads, a bearer token for writes) to run inside a CI job - no bespoke wrapper or platform-specific integration.

#### Scenario: CI job fetches a secret

- **WHEN** a CI job invokes get with a decrypting private key supplied via its normal environment or configuration
- **THEN** the CLI functions identically to running on any other host, with no CI-specific code path
