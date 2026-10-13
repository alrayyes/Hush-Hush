# Spec Delta

## Purpose

Defines the embedded single-page app's required pages, what each must let
the admin see and do, and the footer content every page carries, so the
service has a usable browser interface on top of its existing API.

## ADDED Requirements

### Requirement: Unauthenticated access is blocked

Every page except the login page SHALL require a valid session, and an
unauthenticated visitor SHALL be redirected to the login page rather than
shown any secret or token data.

#### Scenario: Visiting without a session

- **WHEN** a visitor with no valid session requests any page other than
  login
- **THEN** they are redirected to the login page and no secret or token
  data is fetched or rendered

#### Scenario: Visiting with an expired session

- **WHEN** a visitor's session has expired
- **THEN** they are redirected to the login page the same as a visitor
  with no session at all

### Requirement: Login page

The login page SHALL let a visitor complete a WebAuthn login ceremony and,
on success, land on the secrets overview.

#### Scenario: Successful login redirects to the overview

- **WHEN** a visitor completes the WebAuthn ceremony successfully on the
  login page
- **THEN** they are redirected to the secrets overview page

#### Scenario: Failed ceremony shows an error, not a crash

- **WHEN** the WebAuthn ceremony fails or is cancelled by the browser
- **THEN** the login page shows an error and remains usable for another
  attempt

### Requirement: Secrets overview

The secrets overview SHALL list every stored secret object with its id,
description, created-by/created-at, and updated-by/updated-at, and SHALL
let an authenticated admin view, create, edit, and delete a secret object
from it.

#### Scenario: Listing shows attribution

- **WHEN** the authenticated admin opens the secrets overview
- **THEN** every listed object shows who created it and when, and who last
  updated it and when

#### Scenario: Creating a secret

- **WHEN** the authenticated admin submits the create form with a new id
  and value
- **THEN** the new object appears in the overview afterward with the admin
  as its creator

#### Scenario: Editing a secret

- **WHEN** the authenticated admin submits an updated value for an
  existing object
- **THEN** the object's value changes and its updated-by/updated-at
  reflect the admin and the time of the edit

#### Scenario: Deleting a secret

- **WHEN** the authenticated admin confirms deletion of an object
- **THEN** the object no longer appears in the overview afterward

### Requirement: Settings page - passkeys

The settings page SHALL let the authenticated admin view their registered
passkeys, add a new one, rename one, and delete one, subject to the `auth`
capability's rule against deleting the last remaining credential.

#### Scenario: Adding a passkey from settings

- **WHEN** the authenticated admin starts "add passkey" and completes the
  browser's WebAuthn registration prompt
- **THEN** the new credential appears in the settings page's passkey list

#### Scenario: Deleting a passkey from settings

- **WHEN** the authenticated admin deletes a passkey while at least one
  other remains
- **THEN** it disappears from the list and can no longer be used to log in

### Requirement: Settings page - tokens

The settings page SHALL let the authenticated admin view existing bearer
tokens (id, description, owner, created-at, expires-at, revoked status),
create a new one with a description and TTL, and revoke one, and SHALL
show a newly created token's raw value exactly once, with an explicit
warning that it will not be shown again.

#### Scenario: Creating a token shows the raw value once

- **WHEN** the authenticated admin creates a token from the settings page
- **THEN** the raw token value is displayed immediately with a warning
  that it will not be shown again, and is absent from the page on any
  subsequent view

#### Scenario: Revoking a token from settings

- **WHEN** the authenticated admin revokes a token from the list
- **THEN** its status updates to revoked in place

### Requirement: Footer

Every page SHALL show a footer containing the running version linked to a
changelog page, a disclaimer link, a privacy link, and the licence.

#### Scenario: Footer content is present on every page

- **WHEN** any page (including login) is rendered
- **THEN** the footer shows the current version as a link to the changelog
  page, a disclaimer link, a privacy link, and the licence

#### Scenario: The changelog page reflects the real changelog

- **WHEN** the changelog page is opened
- **THEN** it renders the content of the repository's `CHANGELOG.md`
