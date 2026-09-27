# Spec Delta

## MODIFIED Requirements

### Requirement: Passkey registration

An authenticated admin session SHALL be able to register a new WebAuthn
credential (passkey) to the admin account, and assign it a human-readable
nickname. Registration SHALL detect whether the authenticator supports
the PRF extension, and when it does, SHALL derive a PRF secret and use it
to wrap a copy of that user's escrowed writer identity, stored against
the new credential.

#### Scenario: Registering a first passkey

- **WHEN** there is no admin account yet and a caller completes a WebAuthn
  registration ceremony against the registration endpoint
- **THEN** the admin account is created, the credential is stored against
  it, the user's escrowed writer identity is established, and the caller
  is issued a session

#### Scenario: Registering an additional passkey

- **WHEN** an authenticated admin session completes a WebAuthn registration
  ceremony
- **THEN** a new credential is added to the same admin account without
  affecting any existing credential or active session

#### Scenario: Registration ceremony expires

- **WHEN** the registration ceremony's challenge is not completed within
  its validity window
- **THEN** the server rejects the completion attempt and no credential is
  stored

#### Scenario: PRF-capable authenticator wraps a copy of the escrowed identity

- **WHEN** a credential is registered whose extension results indicate PRF
  support
- **THEN** a wrapped copy of the user's escrowed writer identity is stored
  against that credential

#### Scenario: Non-PRF authenticator registers without wrapping

- **WHEN** a credential is registered whose extension results do not
  indicate PRF support
- **THEN** the credential is stored as usual, and no wrapped copy of the
  escrowed identity is attempted against it

## ADDED Requirements

### Requirement: Credentials are attributed to a user

Each registered WebAuthn credential SHALL be associated with exactly one
user account (`users` capability), rather than an implicit single-account
model.

#### Scenario: Credential lookup resolves to its owning user

- **WHEN** a registered credential is used to authenticate
- **THEN** the resulting session is attributed to the specific user
  account that credential is registered to
