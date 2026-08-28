## Purpose

Defines the CRUD lifecycle for sealed secret objects: how values are created, fetched, updated, and deleted, how they are sealed to one or more recipients, how write access is gated, and how used_by lineage is tracked.

## ADDED Requirements

### Requirement: Create a sealed secret object
The system SHALL allow a writer holding a valid bearer token to create a new object from an already-sealed value, associated with an object id and optional used_by metadata. The system SHALL NOT decrypt the value at any point.

#### Scenario: Writer creates a new object
- **WHEN** a writer with a valid bearer token submits a create request with a sealed value and an object id
- **THEN** the system stores the object and returns success without ever decrypting the value

#### Scenario: Create request without a valid bearer token is rejected
- **WHEN** a create request is submitted without a valid bearer token
- **THEN** the system rejects the request and does not store the object

### Requirement: Fetch a secret object's ciphertext
The system SHALL return an object's stored ciphertext to any requester who supplies its object id, without requiring pre-fetch authorization in v1.

#### Scenario: Any requester fetches an object by id
- **WHEN** a requester submits a get request with a valid object id
- **THEN** the system returns the stored ciphertext exactly as sealed, without decrypting it

#### Scenario: Fetch of an unknown object id
- **WHEN** a requester submits a get request with an object id that does not exist
- **THEN** the system returns a not-found response

### Requirement: Update a secret object's value
The system SHALL allow a writer holding a valid bearer token to replace an existing object's sealed value while preserving its object id and used_by metadata.

#### Scenario: Writer rotates an existing object's value
- **WHEN** a writer with a valid bearer token submits an update request with a new sealed value for an existing object id
- **THEN** the system replaces the stored value while the object id and used_by metadata remain unchanged

#### Scenario: Update request without a valid bearer token is rejected
- **WHEN** an update request is submitted without a valid bearer token
- **THEN** the system rejects the request and does not modify the object

### Requirement: Delete a secret object
The system SHALL allow a writer holding a valid bearer token to permanently remove an object, after which fetches for that id return not-found.

#### Scenario: Writer deletes an existing object
- **WHEN** a writer with a valid bearer token submits a delete request for an existing object id
- **THEN** the system removes the object and subsequent get requests for that id return not-found

#### Scenario: Delete request without a valid bearer token is rejected
- **WHEN** a delete request is submitted without a valid bearer token
- **THEN** the system rejects the request and does not remove the object

### Requirement: Multi-recipient sealing
The system SHALL support an object being sealed to more than one recipient public key at write time, so a single object can serve multiple independent consumers without duplication.

#### Scenario: Object sealed to multiple recipients
- **WHEN** a writer creates an object sealed to more than one recipient's public key
- **THEN** each recipient holding a matching private key can independently decrypt the fetched ciphertext

### Requirement: used_by lineage tracking
The system SHALL store, alongside each object, a queryable record of which consumers (repos or hosts) depend on it.

#### Scenario: Querying what depends on a secret
- **WHEN** a caller queries an object's used_by metadata
- **THEN** the system returns the list of recorded consumers for that object

#### Scenario: used_by metadata persists across an update
- **WHEN** an object's value is updated
- **THEN** its used_by metadata is unchanged by the update

### Requirement: The service never computes or persists plaintext
The system SHALL NOT decrypt a stored value at any point during create, read, update, or delete.

#### Scenario: No decryption occurs during any operation
- **WHEN** any create, read, update, or delete operation is performed
- **THEN** the system's stored and returned data remains sealed ciphertext, never plaintext
