# Spec Delta

## Purpose

Lets the login page tell whether an admin account exists yet, so it can
offer exactly the right first action - register the first passkey, or log
in with one - instead of always offering both.

## ADDED Requirements

### Requirement: Bootstrap status is discoverable without authentication

Any caller, authenticated or not, SHALL be able to ask whether an admin
account exists yet, without that call requiring a session, starting a
ceremony, or otherwise having any side effect.

#### Scenario: No admin account exists yet

- **WHEN** the bootstrap-status endpoint is called and no admin account
  has been created
- **THEN** the response reports that no admin account exists

#### Scenario: An admin account already exists

- **WHEN** the bootstrap-status endpoint is called after an admin account
  has been created
- **THEN** the response reports that an admin account exists

### Requirement: The login page offers exactly one primary action

The login page SHALL determine, before the visitor acts, whether an admin
account exists, and SHALL offer registering a passkey as the primary
action only when none exists - never alongside a login action, and never
once an admin account exists.

#### Scenario: Fresh install offers registration, not login

- **WHEN** the login page loads and no admin account exists yet
- **THEN** the visitor is offered a way to register a passkey, and no way
  to log in is shown

#### Scenario: Bootstrapped install offers login, not registration

- **WHEN** the login page loads and an admin account already exists
- **THEN** the visitor is offered a way to log in with a passkey, and no
  way to register a new one is shown
