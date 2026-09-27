# Spec Delta

## MODIFIED Requirements

### Requirement: Create a sealed secret object

The system SHALL allow a writer holding a valid bearer token or session to
create a new object from an already-sealed value, associated with an
internal object id, a user-facing slug, the creating user as owner, and
optional used_by metadata. The system SHALL NOT decrypt the value at any
point.

#### Scenario: Writer creates a new object

- **WHEN** a writer with a valid bearer token submits a create request with
  a sealed value and a slug
- **THEN** the system stores the object under a new internal id, records
  the requested slug, and returns success without ever decrypting the
  value

#### Scenario: Create request without a valid bearer token is rejected

- **WHEN** a create request is submitted without a valid bearer token
- **THEN** the system rejects the request and does not store the object

#### Scenario: Owner recorded from the creating session

- **WHEN** an authenticated session creates a new object
- **THEN** the object's owner is recorded as that session's user, without
  that owner automatically becoming a decrypt recipient

## ADDED Requirements

### Requirement: Internal id decoupled from user-facing slug

`objects.id` SHALL be an internal identifier, never exposed to or
addressable by a caller. Every caller-facing path - URLs, the CLI, and the
API - SHALL address an object by its `slug` instead. A slug SHALL be
unique and SHALL remain associated with the same internal id even if the
slug's own value is later changed.

#### Scenario: Fetch, update, and delete address an object by slug

- **WHEN** a fetch, update, or delete request supplies an object's slug
- **THEN** the system resolves it to the matching internal id and performs
  the operation on that object

#### Scenario: An object's internal id is never returned as an addressable value

- **WHEN** an object's metadata is returned to a caller
- **THEN** any internal id present is informational only, and no
  documented request path accepts it as an address

### Requirement: Secret ownership is recorded but not an implicit recipient

Each secret object SHALL record its owning user (`owner_id`), captured at
creation from the authenticated session or user that created it, as
accountability and audit metadata. Recording ownership SHALL NOT, by
itself, make the owner's public key a decrypt recipient of the object.

#### Scenario: Owner cannot decrypt a secret they did not opt into

- **WHEN** a secret is created without opting the owner in as a recipient
- **THEN** the owner's own escrowed identity cannot decrypt the stored
  ciphertext, the same as any other non-recipient

#### Scenario: Owner's account loss does not affect consumer access

- **WHEN** a secret's owner did not opt in as a recipient, and that
  owner's user account and all of their passkeys are later lost entirely
- **THEN** every consumer-recipient's ability to decrypt that secret is
  unaffected

### Requirement: Opt-in owner-recipient inclusion at create time

The create (and update) request SHALL accept an explicit, optional flag
requesting that the owner's own public key be included as an additional
decrypt recipient alongside the resolved consumer recipients. Omitting the
flag SHALL NOT include the owner as a recipient.

#### Scenario: Owner opts in to keep a readable copy

- **WHEN** a writer creates an object and sets the owner-recipient flag
- **THEN** the sealed value includes the owner's public key as one of its
  recipients, and the owner's escrowed identity can decrypt it

#### Scenario: Owner recipient is not the default

- **WHEN** a writer creates an object without setting the owner-recipient
  flag
- **THEN** the sealed value's recipients are exactly the resolved consumer
  public keys, with no owner key added
