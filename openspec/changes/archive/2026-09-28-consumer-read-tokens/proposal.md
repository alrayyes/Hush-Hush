# Proposal

## Why

`GET /objects/{slug}` is currently unauthenticated by deliberate design
([ADR 2](../../../docs/adr/0002-unauthenticated-read-path-with-audit-log.md)),
which reasons that confidentiality already comes from who holds the matching
age private key. That ADR explicitly leaves per-consumer read authorization
as deferred scope for later, not something the design closes off. Ryan asked
for that gap to close: consumers should authenticate reads with their own
token, so a leaked token exposes only that consumer's own secrets instead of
every object in the store
([alrayyes/hush-hush#438](https://github.com/alrayyes/hush-hush/issues/438)).

## What Changes

- A new consumer read token type, bound to exactly one consumer name, issued
  and managed the same way write bearer tokens already are (hashed at rest,
  TTL, rotate-in-place, soft-delete revoke, `last_used_at`).
- New session+CSRF-gated HTTP endpoints for consumer tokens: create, list,
  rotate, revoke - full CRUD parity with the existing `/tokens` endpoints.
- **BREAKING**: `GET /objects/{slug}` no longer accepts anonymous requests.
  It now requires one of: a valid write bearer token, a valid admin session,
  or a valid consumer token whose bound consumer appears in that object's
  `used_by` list.
- A consumer token presented against an object outside its scope gets `404`
  (indistinguishable from an unknown slug), not `403` - consistent with
  [ADR 7](../../../docs/adr/0007-listing-gated-by-write-token.md)'s existing
  concern about not letting a caller enumerate ids it doesn't already hold.
- A new ADR recorded documenting this decision as partially superseding
  ADR 2, the same way
  [ADR 14](../../../docs/adr/0014-session-authenticates-secret-object-access.md)
  is recorded against
  [ADR 3](../../../docs/adr/0003-single-bearer-token-write-path.md).

Out of scope for this change (tracked separately, per
[alrayyes/hush-hush#438](https://github.com/alrayyes/hush-hush/issues/438)'s
sibling issues): hard-delete/purge of dead tokens (#439), the web UI token
management UI (#440, #441), the CLI (hush-hush-cli#133), the GitHub Action
(hush-hush-action#31), and the client SDKs (4 follow-up issues).

## Capabilities

This repo has no synced `openspec/specs/` yet (`openspec list --specs`
reports none, and `openspec show tokens|secret-objects` confirms neither
path exists) - every prior change's specs live only as deltas under
`openspec/changes/*/specs/`, never archived into a canonical main spec.
Both capabilities below reuse the exact path names earlier changes already
established (`tokens` in `tokens-last-used-at/specs/tokens/` and
`web-ui/specs/tokens/`; `secret-objects` in
`secrets-object-store/specs/secret-objects/`), but since neither has a
canonical spec yet, this change's deltas are necessarily `ADDED`, scoped to
only the behaviour this change actually introduces - not a backfill of every
requirement those capabilities already have in shipped code. Backfilling
the full unarchived history into a canonical spec is separate, pre-existing
scope this change doesn't take on.

### New Capabilities

- `tokens`: adds a second token kind (consumer read tokens, consumer-scoped)
  alongside the existing write bearer tokens, with its own create/list/
  rotate/revoke requirements. (Reuses an existing path name from prior
  unarchived deltas; see note above.)
- `secret-objects`: `GET /objects/{slug}`'s authorization requirement
  changes from none to bearer-token-or-session-or-scoped-consumer-token.
  (Same note.)

### Modified Capabilities

(none - see note above: nothing to modify against, since neither capability
has a canonical spec yet)

## Impact

- `internal/store/tokens.go`: new consumer-token table and CRUD methods,
  parallel to the existing `write_tokens` ones.
- `internal/api/tokens.go`, `internal/api/get.go`, `internal/api/server.go`:
  new handlers and routes; `GET /objects/{slug}` gains an auth check.
- `api/openapi.yaml`: new schemas and endpoints; `GET /objects/{slug}`'s
  security requirement changes.
- `docs/adr/`: one new ADR.
- `ARCHITECTURE.md`: the "Read path" bullet updated to match.
- No change to the age encryption model itself - this adds an access-control
  layer on top, it doesn't touch how objects are sealed or unsealed.
