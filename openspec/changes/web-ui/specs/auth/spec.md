# Spec Delta

## Purpose

Gives the single admin account a way to authenticate to the web UI with a
WebAuthn passkey instead of a password, and ties the resulting session to
the writes that account makes through the UI so the audit log records a
real, verified identity rather than a self-reported label.

## ADDED Requirements

### Requirement: Passkey registration

An authenticated admin session SHALL be able to register a new WebAuthn
credential (passkey) to the single admin account, and assign it a
human-readable nickname.

#### Scenario: Registering a first passkey

- **WHEN** there is no admin account yet and a caller completes a WebAuthn
  registration ceremony against the registration endpoint
- **THEN** the admin account is created, the credential is stored against
  it, and the caller is issued a session

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

### Requirement: Passkey login

A caller holding a private key matching a registered credential SHALL be
able to authenticate via a WebAuthn login ceremony and receive a session.

#### Scenario: Successful login

- **WHEN** a caller completes a WebAuthn login ceremony using a registered
  credential
- **THEN** the server verifies the assertion, updates that credential's
  stored signature counter and last-used timestamp, and issues a new
  session

#### Scenario: Cloned authenticator detected

- **WHEN** a login assertion's signature counter is not greater than the
  credential's last stored counter
- **THEN** the server rejects the login and does not issue a session

#### Scenario: Login with an unregistered credential

- **WHEN** a login ceremony is attempted with a credential that is not
  registered to the admin account
- **THEN** the server rejects the login and issues no session

### Requirement: Credential management

An authenticated admin session SHALL be able to list its registered
credentials with nickname, created-at, and last-used-at, rename a
credential's nickname, and delete a credential, except that the last
remaining credential SHALL NOT be deletable.

#### Scenario: Listing credentials

- **WHEN** an authenticated session requests its credential list
- **THEN** the response includes every registered credential's nickname,
  created-at, and last-used-at, and excludes the credential's public key
  material

#### Scenario: Deleting a non-last credential

- **WHEN** an authenticated session deletes a credential and at least one
  other credential remains registered
- **THEN** the deleted credential can no longer be used to log in

#### Scenario: Refusing to delete the last credential

- **WHEN** an authenticated session attempts to delete the only remaining
  registered credential
- **THEN** the server rejects the deletion and the credential remains
  registered

### Requirement: Session lifecycle

A successful login SHALL issue a session usable by the web UI, which
SHALL be invalidated by an explicit logout or by expiry. A session is an
independent credential from any write bearer token: it authenticates
`/objects` requests on its own terms (`Session-attributed writes` below),
never by deriving, exposing, or reusing a bearer token's own value, and
revoking or expiring one SHALL NOT affect the other.

#### Scenario: Session grants UI access

- **WHEN** a request to a web-UI-facing endpoint carries a valid,
  unexpired session
- **THEN** the request is treated as authenticated as the admin account

#### Scenario: Logout invalidates the session

- **WHEN** an authenticated session calls logout
- **THEN** that session is immediately invalid for any subsequent request

#### Scenario: Expired session is rejected

- **WHEN** a request carries a session past its expiry
- **THEN** the request is treated as unauthenticated

#### Scenario: A session never exposes or substitutes for a token's own value

- **WHEN** an authenticated session is used against any endpoint
- **THEN** no response ever includes a bearer token's raw value on the
  strength of the session alone, and no bearer token is invalidated or
  altered as a side effect of a session being created, used, or ended

### Requirement: A session authenticates secret-object access

`/objects` (list, fetch, create, update, delete) SHALL accept a valid,
unexpired session - with its CSRF token on create/update/delete - as a
credential equally valid to the write bearer token. This is what lets
the web UI's secrets overview work at all: it holds a session, never a
bearer token. The two credentials are independent (`Session lifecycle`
above); a request may be authenticated by either without needing both.

#### Scenario: A session lists objects without a bearer token

- **WHEN** `GET /objects` carries a valid session and no bearer token
- **THEN** the request succeeds exactly as it would with a valid bearer
  token

#### Scenario: A session creates, updates, or deletes an object

- **WHEN** a create, update, or delete request carries a valid session
  and its matching CSRF token, and no bearer token
- **THEN** the request succeeds exactly as it would with a valid bearer
  token

#### Scenario: A session's write without its CSRF token is rejected

- **WHEN** a create, update, or delete request carries a valid session
  but a missing or wrong CSRF token, and no bearer token
- **THEN** the request is rejected

### Requirement: Session-attributed writes

A secret-object create, update, or delete made through an authenticated
session SHALL record that session's admin identity as the verified actor
on the resulting audit log entry (`audit-log/spec.md`'s "Verified actor
attribution" requirement), alongside - not in place of - any `X-Caller`
header sent on the same request.

#### Scenario: UI-driven write is attributed to the session

- **WHEN** an authenticated session creates, updates, or deletes a secret
  object
- **THEN** the resulting audit log entry's actor is the admin account,
  regardless of any `X-Caller` header present on the request, and that
  header - if present - is still recorded in the entry's separate caller
  field unchanged

#### Scenario: Token-authenticated write keeps existing attribution

- **WHEN** a create, update, or delete is authenticated by a bearer token
  rather than a session
- **THEN** the audit log entry's caller field is populated from the
  `X-Caller` header exactly as before this capability, and its actor is
  the authenticating token per `audit-log/spec.md`
