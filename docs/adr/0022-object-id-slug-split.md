# 22. `objects.id` is an internal identifier, decoupled from a user-facing slug

## Status

Accepted. Partly changed by [ADR 33](0033-one-name-many-variants.md): the id
is now a UUID the API returns, and the slug is no longer unique.

## Context

Before this decision, `objects.id` was both the database primary key and
the literal string a caller chose and addressed the object by (a URL path
segment, a CLI argument, an audit-log reference). `used_by` already had to
work around this: renaming a consumer meant rewriting every `used_by` row
that named it, because nothing separated "what a human calls this" from
"what a foreign key points at." Once objects gained real ownership
(`owner_id`) and consumer resolution keyed by name, the same coupling
would have meant a slug rename cascading through every relation that ever
referenced the object.

## Decision

`objects.id` becomes an internal identifier, generated the same way this
service already generates other internal ids (`randomHex`), never
exposed to or addressable by a caller. A new `slug` column holds what a
caller actually types - the API path (`/objects/{slug}`), the create/
update request field, and the CLI argument all address an object by
slug, resolved to the internal id server-side. `used_by.object_id`
(and, going forward, anything else that references an object
internally) points at the stable internal id, not the slug, so a future
slug rename doesn't need to rewrite those rows.

Existing rows migrate by keeping their current id value as their slug
verbatim, and getting a freshly generated internal id in its place -
`used_by` rows are updated to point at the new internal id in the same
migration transaction.

## Consequences

- **BREAKING**: the create/update request's `id` field, the response's
  `id` field, and the `/objects/{id}` path are all renamed to `slug`.
  Every existing client (the CLI, the generated SDKs, the MCP tools) has
  to update to the new field name - tracked per client
  (`alrayyes/hush-hush-cli#125` and the generated-SDK regeneration
  dispatch, `alrayyes/Hush-Hush#348`).
- A slug can be renamed later without touching `used_by` or any other
  relation that references the object - the whole point of the split.
  No rename operation exists yet; this decision only makes one possible
  without a larger migration when it's eventually built.
- The internal id is never returned by any endpoint as something a
  caller can address the object by - if it appears anywhere in a
  response, it's informational only, never a documented request path.
- The audit log records the slug, not the internal id, in its object
  reference - a human reviewing an incident needs the readable value,
  and the internal id is server-internal bookkeeping.
