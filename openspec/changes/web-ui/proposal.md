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
  Successful login issues a browser session (cookie), scoped to the web UI
  only - the existing CLI/CI bearer-token write path is untouched.
- New HTTP endpoints for bearer-token lifecycle management (create, list,
  revoke) - reversing the v1 decision that minting write credentials must
  never itself be reachable over the network. That decision held because
  the only prior caller was the operator's own terminal with direct DB
  access; a browser has neither, so the settings page needs the same
  capability over HTTP, gated by the new session auth.
- Tokens gain an owner (the admin account that created them) so the
  settings page can show who issued each one - previously implicit (there
  was only ever one operator with DB access).
- Writes made through the web UI (create/update/delete on a secret object)
  record the authenticated session's identity as the audit log's caller,
  instead of the unauthenticated, self-reported `X-Caller` header. The
  header stays as-is for CLI/CI callers, which have no session to attribute
  to.
- New Svelte (SvelteKit + `adapter-static`) single-page app, built and
  embedded into the server binary, served from the same origin as the API
  (no separate deploy, no CORS):
  - Login page (WebAuthn ceremony).
  - Secrets overview: list, view, create, edit, delete, each showing
    created/updated-by and when.
  - Settings page: manage registered passkeys (add, nickname, delete) and
    bearer tokens (create with description + TTL, list, revoke).
  - Footer: version linked to a changelog page (rendering `CHANGELOG.md`),
    a disclaimer link, a privacy link, and the licence.

## Capabilities

### New Capabilities

- `auth`: WebAuthn passkey registration and login for a single admin
  account, browser session issuance/validation/logout, and attributing an
  authenticated session's identity to secret-object writes made through it.
- `tokens`: HTTP API for bearer-token lifecycle management - create
  (description + TTL), list (metadata only, never the raw value after
  creation), revoke - each token owned by the admin account that created
  it.
- `web-ui`: the embedded Svelte single-page app itself - routes, what each
  page must show and let the operator do, and the required footer
  links/content. Consumes the `auth`, `tokens`, and existing secret-object
  HTTP APIs; adds no new server behaviour of its own beyond serving static
  assets.

### Modified Capabilities

None. `openspec list --specs` shows no existing capability specs in this
repo to delta against - the original secrets-object-store work predates
this project's use of the spec-driven OpenSpec workflow. The secret-object
CRUD and audit-log endpoints this change builds on are unchanged; only the
new `auth` capability's session-attribution requirement adds a new _source_
of the audit log's existing `caller` field, described there rather than as
a delta to a nonexistent spec.

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
- No impact on `hush-hush-cli` or the generated SDKs - the write bearer
  token and existing object/audit endpoints are unchanged; new endpoints
  are additive to `api/openapi.yaml`.
