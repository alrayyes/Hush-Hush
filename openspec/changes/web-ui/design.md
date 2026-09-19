# Design

## Context

See proposal.md - Why and What Changes. Two existing constraints shape
this design directly:

- `api/openapi.yaml`'s `servers` block has no canonical host - `scheme`
  and `host` are variables, because this is meant to run anywhere. WebAuthn
  needs a fixed relying-party ID and origin per deployment, so that has to
  be configurable the same deployment-agnostic way the rest of the service
  already is, not hardcoded.
- Storage is SQLite via `modernc.org/sqlite`, one writer at a time
  (`openspec/changes/secrets-object-store/design.md`). New tables
  (credentials, sessions, ceremony state) live in the same database file,
  not a second store.

## Goals / Non-Goals

**Goals:**

- A human operator can do everything the CLI/direct-DB-access path
  currently requires a terminal for, from a browser, authenticated by a
  passkey.
- The existing CLI/CI bearer-token write path keeps working completely
  unchanged - this is additive, not a migration.
- The frontend build and the Go build compose into one binary with no
  runtime dependency on Node, a frontend development server, or a CDN.

**Non-Goals:**

- Multi-user accounts, roles, or invitations - single admin account,
  multiple passkeys, per the decision already made with the user.
- Replacing or deprecating bearer tokens - they remain the only
  credential CI/scripts use.
- A general-purpose object-level ACL - unrelated to this change; still
  deferred per the original design.md's read-path decision.

## Decisions

**Frontend: SvelteKit + `@sveltejs/adapter-static`, not plain Svelte+Vite.**
Researched against <https://svelte.dev/docs/ai/overview> and SvelteKit's
own SPA docs. `adapter-static` with `export const ssr = false` in the root
`+layout.ts` and `fallback: 'index.html'` in `svelte.config.js` is the
documented pattern for "own backend, static frontend, no SSR." It buys
SvelteKit's router, `load` functions, and typed routing for the same
effort as hand-rolling a Vite SPA, with no `+page.server.js`/`+server.js`
files anywhere - every data access goes through the Go API via `fetch`.
Svelte 5 runes (`$state`, `$derived`) throughout. Per `rules/svelte.md`,
the project is scaffolded with `npx sv add ai-tools` so implementation
leans on Svelte's own MCP-driven docs workflow and `svelte-autofixer`
rather than generating Svelte 5 syntax unaided - this is the first Svelte
work in the repo, so that setup happens here rather than being inherited
from an existing scaffold. Alternative considered: bare Svelte + Vite
with a hand-rolled router. Rejected - SvelteKit's
file-based routing and load functions are worth the (small) extra
dependency weight for an app with several pages and auth guards on most of them,
and it's still a static build in the end.

**Component approach: Bits UI (headless) for interactive primitives
(dialogs for create/edit/delete confirmation, dropdowns), hand-styled
otherwise.** Chosen over shadcn-svelte, whose fuller pre-styled kit costs
more dependency surface than a few pages need; over Melt UI,
whose Svelte 5 support was still migrating as of this research. Revisit if
the page count grows enough that hand-styling becomes the bottleneck.

**Build embedding: frontend built to `web/build/`, embedded into the Go
binary with `go:embed`, served for every non-`/api`-prefixed path (or
however the existing router is namespaced - see tasks.md for the concrete
routing change).** Alternative considered: serving the SPA from a separate
static host/CDN. Rejected - it reintroduces the CORS and separate-deploy
complexity the user explicitly decided against, and this service already
ships as a single distroless binary; a second deployable would defeat that.
`Dockerfile` gains a frontend build stage (Node/bun) ahead of the Go build
stage, discarded from the final image the same way the Go build stage
already is.

**WebAuthn library: `go-webauthn/webauthn` server-side,
`@simplewebauthn/browser` client-side.** Confirmed still the maintained Go
option (successor to the archived `duo-labs/webauthn`, FIDO2-conformant,
active). `@simplewebauthn/browser` is a thin wrapper over
`navigator.credentials`, framework-agnostic, callable directly from Svelte
component code - no Svelte-specific WebAuthn binding needed or found worth
adding as a dependency.

**Relying party ID/origin: a new `PUBLIC_URL` environment variable (for
example `https://secrets.example.com`), required once any WebAuthn
registration is attempted; `RPID` and `RPOrigins` are derived from it, not
configured separately.** If unset, WebAuthn endpoints respond with a clear
configuration error rather than guessing a host from request headers -
guessing from `Host`/`X-Forwarded-Proto` is a known WebAuthn phishing
risk (an attacker-controlled Host header could otherwise shift the
relying party). This is one new required setting for whoever enables the
web UI, consistent with the rest of the service's environment-variable
configuration; it does not change `ADDR`'s existing meaning (the bind
address, unrelated to the public origin a browser sees).

**Session storage: a `sessions` table in the same SQLite database,
`httpOnly`, `secure`, `SameSite=Lax` cookie, no `Domain` attribute,
session id regenerated on every successful login.** Same-origin embedded
SPA means no CORS, so `SameSite=Lax` covers most CSRF exposure by itself;
state-changing requests (create/update/delete secret objects, token
management, credential management) additionally require the session's
CSRF token to be echoed back in a header, checked server-side - covers the
gap `SameSite=Lax` leaves open (it still allows a cross-site top-level GET
navigation to carry the cookie). Alternative considered: JWT in
`localStorage`. Rejected - vulnerable to exfiltration via any XSS, and
buys nothing here since there's no separate API host to avoid a cookie
for.

**WebAuthn ceremony state: short-lived rows in the same database (a
`webauthn_ceremonies` table), not in-memory.** The server is a single
process today, so in-memory would work, but a DB row costs nothing extra
here (one more table, same writer) and survives a process restart
mid-ceremony instead of forcing the browser to retry from scratch.
Ceremony rows expire (a few minutes) and are cleaned up opportunistically
on the next ceremony start, the same pattern audit-log-adjacent tables
already use.

**Credential storage fields: credential id, COSE public key, sign
counter, AAGUID, nickname, created-at, last-used-at.** Sign counter is
checked and updated on every login to detect a cloned authenticator,
per `go-webauthn`'s own verification step - see auth/spec.md's "Cloned
authenticator detected" scenario.

**Token ownership is nullable, not backfilled or guessed.** A token
issued via the existing CLI path has no session to attribute to, and
predates this change entirely for tokens already in a running
deployment's database. Guessing an owner (for example "the admin
account") would be a fabricated attribution the audit trail shouldn't
carry. See
tokens/spec.md's "CLI-created token has no owner" scenario.

**`api/openapi.yaml` gains the new endpoints (WebAuthn ceremonies,
session, credential management, token management) under the project's
existing spec-first convention - reviewed there before the handlers are
written, same as every prior endpoint.**

## Risks / Trade-offs

- [Risk] A single admin account with a lost/inaccessible last passkey and
  no recovery mechanism locks the operator out of the web UI entirely →
  [Mitigation] The CLI's existing direct-DB-access path is untouched and
  still works for token management even if the UI is unreachable; account
  recovery (for example, a break-glass CLI command to register a new
  credential directly against the DB) is worth a follow-up issue but is
  out of scope for this change - filed as deferred scope once this lands.
- [Risk] `PUBLIC_URL` misconfigured (for example, changed after
  credentials are already registered) breaks existing passkeys, since RPID
  changes
  invalidate them → [Mitigation] Documented prominently in the README next
  to the variable; this is inherent to how WebAuthn binds credentials to
  an origin, not something this design can route around.
- [Risk] New tables and endpoints expand the attack surface of a service
  whose whole design point was minimizing trust surface → [Mitigation]
  Scoped tightly: sessions authenticate the UI only, never substitute for
  the bearer-token write path (see auth/spec.md's "A session does not
  substitute for a bearer token" scenario); every new endpoint gated by
  session auth, audit-logged the same as existing writes.
- [Trade-off] Embedding the frontend build into the Go binary means a
  frontend-only change still requires a full binary rebuild to ship,
  rather than an independently deployable static asset → [Mitigation]
  Accepted per the user's explicit decision to avoid a separate deploy and
  CORS; frontend development iteration still uses SvelteKit's own
  development server against the running Go API, only production builds
  embed.

**Routing boundary: existing and new JSON endpoints keep their current
paths, with no new prefix added (`/objects`, `/audit-log`, `/healthz`, and
the new `/auth/*`, `/tokens/*` endpoints); the embedded SPA is served as
the fallback handler for any request that doesn't match a known API
route.** Alternative considered: moving everything under a `/api/*`
prefix, which `adapter-static`'s `fallback: 'index.html'` pattern would
make simpler to reason about. Rejected - the existing paths are the
published contract every generated SDK
(`hush-hush-go`/`-python`/`-node`/`-php`) and `hush-hush-cli` already
call; moving them under a new prefix is a breaking change to every one of
those repos for no benefit the user asked for. The Go mux
matches known API routes first and falls through to the embedded static
file handler (serving `index.html` for any unmatched path, the same
fallback `adapter-static` expects) otherwise.

## Migration Plan

- Additive only: existing deployments keep working with no web UI
  configured until `PUBLIC_URL` is set and a first passkey is registered.
  No existing data migrates; new tables start empty.
- First-run bootstrap: the first successful WebAuthn registration
  (no admin account yet) creates the admin account - see auth/spec.md's
  "Registering a first passkey" scenario. No separate account-creation
  step or default credential to rotate away from.
- No rollback complexity beyond redeploying the prior binary - the new
  tables are additive and unused by anything else if the web UI is never
  enabled.
