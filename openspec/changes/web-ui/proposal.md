# Proposal

## Why

The service currently has no operator-facing interface at all: managing a
secret means the CLI or direct SQLite access, and managing a bearer token
means running the server binary again against its database file, since
there is deliberately no HTTP endpoint for token management. That's fine
for CI and scripted use, but there's no way to browse what's stored, see
who touched an object and when, or manage passkeys/tokens without a
terminal and filesystem access to the host. A web UI closes that gap for a
human operator, tracked in
[alrayyes/hush-hush#198](https://github.com/alrayyes/Hush-Hush/issues/198).

## What Changes

- **BREAKING** (additive to the trust model, not a removal): introduces
  real authenticated identity for the first time. WebAuthn (passkey) login
  for a single admin account, any number of registered credentials.
  Successful login issues a browser session (cookie) that authenticates
  `/objects` in its own right - a second, independent credential
  alongside the existing write bearer token, not a replacement for it.
  The web UI holds no bearer token of its own; this is what lets its
  secrets overview list, create, edit, and delete secrets at all. The
  existing CLI/CI bearer-token write path itself is untouched.
- New HTTP endpoints for bearer-token lifecycle management (create, list,
  revoke) - reversing the v1 decision that minting write credentials must
  never itself be reachable over the network. That decision held because
  the only prior caller was the operator's own terminal with direct DB
  access; a browser has neither, so the settings page needs the same
  capability over HTTP, gated by the new session auth.
- Tokens gain an owner (the admin account that created them) so the
  settings page can show who issued each one - previously implicit (there
  was only ever one operator with DB access).
- Revoking a token becomes a soft-delete (a `revoked_at` timestamp) instead
  of deleting its row - **BREAKING** to the existing `RevokeWriteToken`
  storage behaviour, though not to its CLI-visible interface. A revoked
  token has to stay identifiable for its past audit-log entries to remain
  attributable to a real, named token rather than a bare id nobody can
  look up any more.
- Every audit log entry now records a verified `actor` (`token` or
  `session`, plus the specific token/admin id) alongside the existing
  self-reported `X-Caller` header, not instead of it - `X-Caller` stays
  unauthenticated and unchanged. `GET /audit-log` gains an actor/token
  filter, and the audit log itself becomes viewable and filterable from
  the web UI - a new page, not just an endpoint a caller queries directly.
- New Svelte (SvelteKit + `adapter-static`) single-page app, built and
  embedded into the server binary, served from the same origin as the API
  (no separate deploy, no CORS):
  - Login page (WebAuthn ceremony).
  - Secrets overview: list, view, create, edit, delete, each showing
    created/updated-by and when.
  - Settings page: manage registered passkeys (add, nickname, delete) and
    bearer tokens (create with description + TTL, list, revoke).
  - Audit log page: filterable (object, actor/token, date range) and
    exportable (CSV/JSON of the visible page) view of every recorded
    action.
  - Footer: version linked to a changelog page (rendering `CHANGELOG.md`),
    a disclaimer link, a privacy link, and the licence.

## Capabilities

### New Capabilities

- `auth`: WebAuthn passkey registration and login for a single admin
  account, browser session issuance/validation/logout, and attributing an
  authenticated session's identity to secret-object writes made through it.
- `tokens`: HTTP API for bearer-token lifecycle management - create
  (description + TTL), list (metadata only, never the raw value after
  creation), revoke (soft-delete) - each token owned by the admin account
  that created it, and identifiable for as long as its audit history
  exists even after revocation.
- `audit-log`: verified actor attribution (token or session) on every
  recorded action, an actor/token query filter alongside the existing
  object/caller/time ones, and a web UI page to view, filter, and export
  it.
- `web-ui`: the embedded Svelte single-page app itself - routes, what each
  page must show and let the operator do, and the required footer
  links/content. Consumes the `auth`, `tokens`, `audit-log`, and existing
  secret-object HTTP APIs; adds no new server behaviour of its own beyond
  serving static assets.

### Modified Capabilities

None. `openspec list --specs` shows no existing capability specs in this
repo to delta against - the original secrets-object-store work predates
this project's use of the spec-driven OpenSpec workflow. The secret-object
CRUD endpoint behaviour this change builds on is unchanged; the existing
audit-log _endpoint's_ behaviour changes (new query filter, new response
fields) but there's no existing spec file for it to delta against either,
so that's described fully in the new `audit-log` capability instead.

## Impact

- New Go packages/endpoints in `internal/api` (and likely a new
  `internal/webauthn` or `internal/auth` package): WebAuthn ceremony
  handlers, session middleware and storage, token management endpoints.
- New SQLite tables: WebAuthn credentials, sessions, and whatever
  ceremony-in-progress state registration/login need - schema changes to
  `internal/store`.
- New dependency: `go-webauthn/webauthn` (Go, server-side WebAuthn) and its
  transitive dependencies.
- New `web/` directory: a SvelteKit project (Svelte 5, `adapter-static`),
  its own `package.json`/toolchain, built as a static bundle and embedded
  into the server binary via `go:embed`. New frontend dependency:
  `@simplewebauthn/browser`.
- `api/openapi.yaml` grows new paths (auth ceremonies, session, token
  management) - reviewed as the design surface per the project's spec-first
  convention, same as every other endpoint.
- `Dockerfile`/build process gains a frontend build stage ahead of the Go
  build, so the embedded assets exist before `go build` runs.
- README/CONTRIBUTING updated: how to build/run the web UI, the new
  frontend toolchain requirement (bun, already a dependency for tooling in
  this repo), and the new endpoints.
- `internal/store`'s `RevokeWriteToken` changes from `DELETE` to setting a
  `revoked_at` timestamp - existing behaviour for CLI callers
  (`hush-hush token revoke`) is unchanged at the interface level, but the
  row now persists.
- `hush-hush-cli` gains a new audit-log command mirroring the query
  filters below (object/actor/caller/time, table or JSON output) -
  tracked as a separate issue in that repo, not implemented as part of
  this change. The generated SDKs pick up the new `/audit-log` filter and
  response fields automatically on their next regeneration; no manual
  change needed there.
