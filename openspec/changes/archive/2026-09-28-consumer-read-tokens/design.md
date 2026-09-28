# Design

## Context

`GET /objects/{slug}` (`internal/api/get.go`) currently has no auth
wrapper at all - see proposal.md for why that changes. The schema already
gives this design what it needs, no extra cost: `objects.id` and `used_by
(object_id, consumer)` exist, and `Store.GetObject` already returns
`Object.UsedBy` (`internal/store/objects.go:111`) alongside the
ciphertext in one query - the scoping check needs no extra store round
trip.

The write bearer token system (`internal/store/tokens.go`,
`internal/api/tokens.go`, `write_tokens` table) is the template: hashed
`token_hash`, separate random-hex `id` vs. 32-byte-hex secret, TTL,
rotate-in-place, `revoked_at` soft-delete, `last_used_at`. Reference:
[API key management best
practices](https://api7.ai/blog/best-practices-for-api-key-management) -
this shape (hash-at-rest, id/secret split, inventory with owner/scope/
created/last-used) is already the recommended one; consumer tokens follow
it rather than inventing a second shape.

`requireWriteAccess` (`internal/api/server.go:151`) can't be reused
as-is: it authenticates against the request alone, but a consumer token's
authorization also depends on the _specific object_ being requested
(whether that object's `used_by` includes the token's bound consumer).
That check has to happen after the object is fetched, inside the handler,
not in generic route middleware.

## Goals / Non-Goals

**Goals:**

- Add a consumer-scoped read token type with full create/list/rotate/
  revoke parity with write tokens.
- Gate `GET /objects/{slug}` on write-token-or-session-or-scoped-consumer-
  token, replacing today's open access.
- Preserve existing write-token and session read access unchanged (both
  already read any object today via other endpoints; this doesn't
  restrict them).

**Non-Goals:**

- `GET /objects/{slug}/used-by` and `GET /objects` (listing) stay exactly
  as they are today - unauthenticated and write-token-gated respectively.
  Only the object-body read changes here.
- Hard-delete/purge of tokens - separate change (alrayyes/hush-hush#439).
- Any web UI, CLI, Action, or SDK work - separate tracked issues (see
  proposal.md's Impact section).
- Changing anything about how objects are sealed or unsealed.

## Decisions

**New `consumer_tokens` table, not a `consumer` column bolted onto
`write_tokens`.** A consumer token is a materially different credential
(scoped, bound to a name that isn't even guaranteed to exist as a
`consumers` row) rather than a write token with a filter. Mirrors how
`write_tokens` and `sessions` are already separate tables for separate
credential kinds, rather than one polymorphic table.

```sql
CREATE TABLE IF NOT EXISTS consumer_tokens (
    id TEXT PRIMARY KEY,
    consumer TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    last_used_at TEXT
);
```

No foreign key from `consumer` to `consumers.name` - `used_by.consumer`
already has none (a consumer can exist only via `used_by` references, per
the existing `consumers` table's own doc comment in `schema.sql`), and a
consumer token should be issuable before a consumer has any object
referencing it yet, same as `AddConsumer` already allows.

**Endpoints on a separate `/consumer-tokens` resource**, not nested under
`/tokens`:

- `POST /consumer-tokens` - create
- `GET /consumer-tokens` - list
- `POST /consumer-tokens/{id}/rotate` - rotate
- `DELETE /consumer-tokens/{id}` - revoke

All session+CSRF gated, exactly like the existing `/tokens` endpoints
(`requireSession(s, requireCSRF(...))`). Originally nested under
`/tokens/consumer` - Go 1.22 `ServeMux` resolves that against
`/tokens/{id}/rotate` without ambiguity (a literal segment beats a
wildcard at the same position), but `redocly lint`'s `no-ambiguous-paths`
rule flags it anyway, since not every router shares Go's specific
tie-breaking rule. A separate top-level resource sidesteps the question
entirely rather than relying on one particular router's resolution order.

Alternative considered: a `kind` field on the existing `/tokens`
endpoints. Rejected - it would make every existing write-token response
carry a field that's now conditionally meaningful, and callers of the existing
endpoints (the CLI, the web UI) would all need to start filtering by kind
for behaviour that hasn't changed for them.

**`GET /objects/{slug}` authorizes inside the handler, not via a
boolean-flag middleware like `requireWriteAccess`.** A new
`readAuth(r, s) (kind readAuthKind, consumerTokenID, consumer string, err
error)` helper tries write-token, then session, then consumer-token
authentication in turn (mirroring `bearerTokenID`/`validSession`'s
existing shape) and returns which kind matched, if any. `handleGetObject`
calls it once, rejects with 401 if none matched, fetches the object
(existing 404-if-unknown-slug path unchanged), and - only for the
`consumer` kind - checks `slices.Contains(obj.UsedBy, consumer)` before
returning the ciphertext; a scoped-but-out-of-scope request gets exactly
the same 404 as an unknown slug, both to satisfy the enumeration
requirement and because it's the smallest change to the existing
not-found path.

**Actor attribution extends the existing `tokenContextKey`/
`sessionContextKey` pattern with a `consumerTokenContextKey`.**
`actorFrom` (`internal/api/server.go:237`) gains a third case, returning
`("consumer_token", id)` - so a read authenticated by a consumer token is
audit-logged with real actor attribution instead of the `"", ""` every
read currently gets (there was never an actor to attribute before, since
reads were unauthenticated).

**Rotate rejects a revoked/expired token**, same as `RotateWriteToken`
(ADR 23) - consistency with the existing token, not a new decision.

## Risks / Trade-offs

- [A consumer token and a write token share one `Authorization: Bearer`
  header shape, so a caller could try to use a consumer token against a
  write endpoint] → `AuthenticateWriteToken` only matches rows in
  `write_tokens`; a consumer token's hash won't be found there, so it
  falls through to 401 on write endpoints exactly like any other unknown
  bearer value. No new ambiguity - just a second table with the same
  never-collide-in-practice hash-lookup property `write_tokens` already
  has.
- [`GET /objects/{slug}` now runs an extra `slices.Contains` per request
  for consumer-token reads] → O(`len(used_by)`) on data already fetched in
  the same query; not a new round trip, not worth optimizing further at
  this scale.
- [Existing anonymous integrations relying on today's open read path
  break the moment this ships] → **BREAKING**, called out in proposal.md.
  CLI/Action/SDK follow-up issues exist precisely because of this; there
  is no deprecation window designed here since Ryan asked for the closed
  behaviour directly, not a transition period.

## Migration Plan

- `schema.sql`'s `CREATE TABLE IF NOT EXISTS consumer_tokens` runs the
  same way every other table does - idempotent, applied on every `Open`,
  no separate migration step (matches this repo's existing schema
  convention: no migration framework, per `schema.sql`'s own header
  comment).
- No backfill needed - there's nothing to migrate from; this is a new
  table and a newly enforced check, not a column added to existing rows.
- Rollback is reverting the deploy; the new table is harmless to leave
  behind unused if that happens.

## Open Questions

None - the scoping model, the delete semantics, and the rollout boundary
were all settled with Ryan before this change was proposed (see
proposal.md's Why).
