# Spec Delta

## Purpose

Represents each account as a real user record holding an escrowed writer
identity, so the encryption model has somewhere to hang key custody and
audit attribution even while only one account exists.

## ADDED Requirements

### Requirement: User accounts

The system SHALL represent each account as a row in a `users` table,
keyed by a stable UUID, rather than an implicit single-admin model. Every
existing WebAuthn credential and every secret's owner metadata SHALL
reference a user by this id.

#### Scenario: Single existing account is a real user row

- **WHEN** the system is queried for its users
- **THEN** the one existing admin account is returned as a user row with a
  stable UUID, not an implicit or synthesized identity

### Requirement: Escrowed writer identity

Each user SHALL hold exactly one escrowed writer identity: an age keypair
generated client-side once, whose private key never reaches the server
unwrapped. The server SHALL only ever store wrapped copies of this
private key, never the key itself in the clear.

#### Scenario: Escrowed identity generated once

- **WHEN** a user's escrowed writer identity is first established
- **THEN** the keypair is generated client-side and the server receives
  only wrapped copies of the private key, never the plaintext private key

### Requirement: Per-credential wrapping of the escrowed identity

Each of a user's registered WebAuthn credentials that supports the PRF
extension SHALL additionally derive a PRF secret at registration time and
use it to wrap a copy of that user's escrowed writer identity, stored
alongside that credential. Losing any one credential, while at least one
other PRF-capable or recovery-phrase-derived copy remains, SHALL NOT
strand the identity.

#### Scenario: Registering a second PRF-capable passkey wraps a second copy

- **WHEN** a user registers a second WebAuthn credential that supports the
  PRF extension
- **THEN** a second wrapped copy of that user's escrowed writer identity is
  stored against the new credential, independent of the first

#### Scenario: Deleting one of several passkeys does not strand the identity

- **WHEN** a user deletes one registered credential while at least one
  other credential with a wrapped copy of the escrowed identity remains
- **THEN** the escrowed identity remains recoverable through any surviving
  credential, with no re-encryption of past secrets required

### Requirement: Break-glass recovery phrase

At escrowed-identity setup, the system SHALL generate an exportable
recovery phrase capable of independently unwrapping the escrowed writer
identity, and SHALL display it to the user exactly once. The system SHALL
NOT retain any copy of the recovery phrase itself after that display.

#### Scenario: Recovery phrase shown once at setup

- **WHEN** a user's escrowed writer identity is first established
- **THEN** the recovery phrase is displayed once and the server retains no
  copy of the phrase itself

#### Scenario: No PRF-capable passkey remains

- **WHEN** a user's registered passkeys have never supported the PRF
  extension, or every PRF-capable passkey is lost outside the app's own
  flow for deleting a credential
- **THEN** the escrowed writer identity is still recoverable using the
  break-glass recovery phrase

### Requirement: PRF support is detected at registration

The system SHALL detect, during each WebAuthn registration ceremony,
whether `getClientExtensionResults().prf`, as returned by the
authenticator, indicates PRF support, and SHALL only attempt PRF-based
wrapping against a credential that reports support.

#### Scenario: Authenticator without PRF support registers without wrapping

- **WHEN** a credential is registered whose extension results do not
  indicate PRF support
- **THEN** the system does not attempt to wrap the escrowed identity
  against that credential, and registration otherwise succeeds
