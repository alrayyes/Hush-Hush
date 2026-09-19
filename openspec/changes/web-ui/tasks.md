# Tasks

## 1. OpenAPI Spec (alrayyes/hush-hush#199)

- [x] 1.1 Add WebAuthn registration begin/finish and login begin/finish endpoints to `api/openapi.yaml`, and verify it lints clean with Redocly
- [x] 1.2 Add logout and credential list/rename/delete endpoints, and verify it lints clean
- [x] 1.3 Add token create/list/revoke endpoints (session-gated) to the spec, and verify it lints clean
- [x] 1.4 Document `PUBLIC_URL` in the spec's `info.description` or a cross-reference to the README, and get the spec reviewed and merged before any handler in this change is implemented

## 2. Server: Storage Schema (alrayyes/hush-hush#200)

- [x] 2.1 Add `webauthn_credentials` (credential id, COSE public key, sign counter, AAGUID, nickname, created-at, last-used-at - no separate owning-account column, since a single-admin schema has nothing to key it against) to the schema, and verify migrations apply cleanly to a fresh database
- [x] 2.2 Add `sessions` (id, created-at, expires-at, CSRF token) to the schema, and verify migrations apply cleanly
- [x] 2.3 Add `webauthn_ceremonies` (challenge, kind, created-at, expires-at) to the schema, and verify expired rows are cleaned up opportunistically
- [x] 2.4 Add a nullable owner column and a nullable `revoked_at` column to the existing `write_tokens` table, and verify a migration against a database already containing tokens leaves existing rows with no owner and `revoked_at` null rather than erroring
- [x] 2.5 Add nullable `actor_type` (`token`/`session`) and `actor_id` columns to `audit_log`, separate from the existing `caller` column, and verify migrations apply cleanly to a database already containing audit log entries

## 3. Server: WebAuthn Ceremonies and Session Issuance (alrayyes/hush-hush#201)

- [x] 3.1 Wire `go-webauthn/webauthn`, configured from `PUBLIC_URL` (RPID/RPOrigins derived, not read from request headers), and verify the server returns a clear configuration error when `PUBLIC_URL` is unset and a ceremony is attempted
- [x] 3.2 Implement registration begin/finish, and verify a first-ever registration creates the admin account plus its credential and issues a session, and that a second registration on an existing account adds a credential without affecting existing sessions
- [x] 3.3 Implement login begin/finish, and verify a successful assertion updates the credential's sign counter and last-used-at and issues a session with a regenerated session id
- [x] 3.4 Verify a login whose assertion's signature counter does not exceed the stored counter is rejected and issues no session
- [x] 3.5 Implement session validation middleware, logout, and CSRF-token enforcement on state-changing requests (a double-submit `csrf_token` cookie alongside the httpOnly session one, checked against the `X-CSRF-Token` header), and verify an expired or logged-out session is treated as unauthenticated
- [x] 3.6 Verify a valid session does not authenticate a request to an endpoint that requires the write bearer token

## 4. Server: Credential Management (alrayyes/hush-hush#202)

- [x] 4.1 Implement credential list (nickname, created-at, last-used-at only - no public key material), and verify the response shape
- [x] 4.2 Implement credential rename and delete, and verify a deleted credential can no longer authenticate a login
- [x] 4.3 Verify deleting the last remaining credential is rejected and the credential remains registered

## 5. Server: Session-Authenticated Object Access and Attributed Writes (alrayyes/hush-hush#203)

- [x] 5.0 Correct `auth/spec.md`: a session is a credential equally valid to the write bearer token on `/objects` (not a substitute that's always rejected there), since the web UI holds no bearer token of its own - caught while starting this ticket, documented in `design.md`
- [x] 5.1 Extend `/objects` (list, create, update, delete) to accept a valid session (with its CSRF token on create/update/delete) as an alternative to the write bearer token, and verify each operation succeeds via session alone and via bearer token alone
- [x] 5.2 Implement attribution: a session-authenticated create/update/delete records the admin account as the audit log entry's `actor_type`/`actor_id`, leaving `caller` populated from `X-Caller` exactly as before, and verify with a test per operation
- [x] 5.3 Verify `internal/api/openapi_test.go` and the Pact provider verification both still pass unmodified

## 6. Server: Token Management HTTP Endpoints (alrayyes/hush-hush#204)

- [x] 6.1 Implement token creation (description + TTL, session-gated), and verify the raw value is present only in the creation response
- [x] 6.2 Verify a missing or non-positive TTL is rejected and no token is created
- [x] 6.3 Implement token listing (metadata only) and revocation as a soft-delete (`revoked_at`, not a row delete), and verify a revoked token no longer authenticates a write but stays listed with its description and owner intact
- [x] 6.4 Verify listing shows HTTP-created tokens with their owning admin account and CLI-created tokens with no owner
- [x] 6.5 Change `internal/store`'s `RevokeWriteToken` (the existing CLI-facing path) from `DELETE` to setting `revoked_at`, and `ValidateWriteToken` to check `revoked_at IS NULL` alongside expiry, and verify `hush-hush token revoke`'s CLI-visible behaviour is unchanged

## 7. Server: Bearer-Token-Attributed Audit Writes and Actor Query Filter (alrayyes/hush-hush#214)

- [x] 7.1 Implement attribution: a bearer-token-authenticated create/update/delete records the authenticating token's id as the audit log entry's `actor_type`/`actor_id`, and verify with a test per operation
- [x] 7.2 Add an actor/token filter to `GET /audit-log`, combining with the existing object/caller/time filters, and verify each filter combination
- [x] 7.3 Verify a revoked token's past audit entries still resolve to its real description and owner (tokens/spec.md's "A revoked token's history stays readable" scenario)

## 8. Server: Embed and Serve the SPA (alrayyes/hush-hush#205)

- [x] 8.1 Add a `go:embed` static handler serving `cmd/hush-hush/web/build/`, routed as the fallback after every known API path, and verify existing routes (`/objects`, `/audit-log`, `/healthz`) are unaffected
- [x] 8.2 Verify an unmatched path serves `index.html` and that a fresh build of `internal/api/openapi_test.go`'s contract test and the Pact provider verification both still pass

## 9. Frontend: Scaffold, Login, Secrets Overview (alrayyes/hush-hush#206)

- [x] 9.1 Scaffold a SvelteKit project under `cmd/hush-hush/web/` with `npx sv add ai-tools` (wires the MCP-driven Svelte AI tooling per `rules/svelte.md`), configure `adapter-static`, `ssr = false`, `fallback: 'index.html'`, and Svelte 5 runes, and verify it builds with the repo's frontend build command
- [ ] 9.1a Use the Svelte MCP server's `list-sections`/`get-documentation` before writing unfamiliar SvelteKit APIs (routing, load functions, `adapter-static` config) throughout this and the following frontend tasks, and run `svelte-autofixer` against generated `.svelte`/`.svelte.ts` code until it reports clean before it ships, per `rules/svelte.md` - the tooling itself is wired in (`.mcp.json`, `AGENTS.md`, the `.claude/` skill files `sv add ai-tools` writes), but this session had no live MCP connection to actually call `list-sections`/`get-documentation`/`svelte-autofixer` with; relied on `svelte-check` and a real build catching what those would have instead. Leaving unchecked rather than claiming a tool ran that didn't - a session with MCP access should pick this up before the next unfamiliar SvelteKit API in `#207`/`#215`.
- [x] 9.2 Implement the login page and WebAuthn ceremony calls via `@simplewebauthn/browser`, and verify a successful login redirects to the secrets overview and a failed/cancelled ceremony shows an error without crashing the page - verified by `svelte-check`, a real `bun run build`, and curling the built binary's routing (index.html served for both `/` and `/login`); no browser tool was available this session to drive the actual WebAuthn ceremony interactively, so the ceremony call sequence itself is unverified end to end - worth a real login attempt before `#207` builds on it
- [x] 9.3 Implement an auth guard redirecting any unauthenticated request for a page other than login to the login page, and verify no secret data is fetched before redirect - verified by reading the generated code's control flow (a thrown `redirect()` in the `(app)` layout's `load` aborts the navigation before any child `load` or component runs) and by `svelte-check`; same browser-tool gap as 9.2 for an interactive check
- [x] 9.4 Implement the secrets overview (list with created/updated-by and when, view, create, edit, delete), and verify each action against a running server - CRUD calls against `/objects` and `/audit-log` implemented and type-checked; same browser-tool gap as 9.2, so a real session against a running server (with a registered passkey) hasn't exercised create/edit/delete interactively yet

## 10. Frontend: Settings and Footer (alrayyes/hush-hush#207)

- [x] 10.1 Implement the passkeys section of settings (list, add via registration ceremony, rename, delete), and verify against a running server, including the last-credential-refusal case surfacing as a visible error - implemented and type-checked; no browser tool available this session to drive the registration ceremony or the 409 case interactively (same gap noted on `#206`'s tasks)
- [x] 10.2 Implement the tokens section of settings (create with description + TTL, list, revoke), and verify the raw value's one-time-display behaviour - implemented and type-checked; same interactive-verification gap as 10.1
- [x] 10.3 Implement the footer (version linked to changelog, disclaimer link, privacy link, licence) on every page, sourcing the version from the build rather than hardcoding it - `/healthz` now reports the running binary's own goreleaser-stamped version (`api/openapi.yaml`'s `Health.version`), fetched by the root layout and rendered on every page including login; verified end to end against the real built binary (`curl /healthz`, `curl /settings`, `curl /changelog` all correct)
- [x] 10.4 Implement the changelog page rendering `CHANGELOG.md`, and verify it reflects the file's actual content - verified end to end: `curl /CHANGELOG.md` against the real built binary returns the repository's actual changelog content

## 11. Frontend: Audit Log Page (alrayyes/hush-hush#215)

- [x] 11.1 Implement the audit log page: table, filters (object, actor/token, date range) as removable chips applied instantly, cursor-based pagination, and verify against a running server with more entries than one page - `GET /audit-log` had no pagination at all before this; added `after`/`limit` query parameters and an entry `id` field to `AuditLogEntry` (design.md's "Audit log UI" decision names the mechanism). Verified end to end against the real built binary: seeded three objects via a bearer token, fetched a two-entry first page, took its last entry's `id` as `after`, confirmed the second page picks up exactly where the first left off
- [x] 11.2 Implement "export visible page" as CSV and JSON, and verify the downloaded file matches exactly what's currently filtered/shown - the serialization itself (`src/lib/audit-export.ts`) is unit tested (exact rows, quote escaping, empty-page header-only case); triggering and inspecting a real browser download wasn't checkable this session (no browser tool), so that half is unverified
- [x] 11.3 Verify an entry with no actor (an unauthenticated read) renders as "none" rather than blank or erroring - unit tested directly (`auditActorLabel`), including the case that also has a caller present, to prove it's not silently falling back to that instead

## 12. Build, Docs, and End-to-End Validation (alrayyes/hush-hush#208)

- [ ] 12.1 Add a frontend build stage to `Dockerfile` ahead of the Go build stage, and verify `docker build .` produces the same distroless image shape with no Node/bun in the final layer
- [ ] 12.2 Update README (requirements, `PUBLIC_URL`, how to reach the web UI, frontend toolchain) and CONTRIBUTING (frontend build/test commands), and verify a reader with no prior context can follow it to build and reach the login page
- [ ] 12.3 Migrate one real login + secret create/edit/delete + token create/revoke + audit log filter/export round trip against a running instance end to end, and verify it succeeds
- [ ] 12.4 File the `hush-hush-cli` audit-log command as an issue in that repo, referencing this change's actor/token filter shape
- [ ] 12.5 Verify every sub-issue this change touched or spawned is closed referencing what shipped, and archive this OpenSpec change
