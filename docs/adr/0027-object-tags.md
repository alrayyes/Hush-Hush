# 27. Tag objects with free-form labels

## Status

Accepted. Adds to the object model; changes nothing in the encryption
model ([ADR 1](0001-per-object-age-encryption.md)).

## Context

The web UI's secrets overview wants to group and filter secrets, for
example `homelab` against `prod`. An object carries a slug, a consumer
list and a description, and none of those is something to group on.
[Issue #500](https://github.com/alrayyes/hush-hush/issues/500) asked for
the backend half.

## Decision

An object has an optional set of tags, stored in a `tags` table keyed by
the internal object id, the same shape as `used_by`. `POST /objects` and
`PUT /objects/{slug}` accept `tags`; `GET /objects` accepts a repeatable
`tag` query parameter and returns only objects carrying every tag given.
Every object-metadata response carries `tags`, an empty array when none.

Tags are free-form rather than a fixed set, so a new environment needs no
server change. They are converted to lowercase, and repeats are dropped.
Each is 1 to 32 characters from `a-z 0-9 . _ / -`, with at most 10 per
object. Anything else is a 400.

On update, an omitted `tags` leaves them alone and an empty array clears
them, the same rule `used_by` follows
([#299](https://github.com/alrayyes/hush-hush/issues/299)). In the store
that's a `WithTags` option on `CreateObject` and `UpdateObject`, so the
nil-versus-empty split survives and existing call sites don't change.

A new table needs no column migration: `CREATE TABLE IF NOT EXISTS` adds
it on the next start, and objects that predate it simply have no rows.

## Consequences

- Tags are metadata. They are never part of the sealed value and never
  affect who can decrypt it.
- Like the description and `used_by`, tags are readable by anyone allowed
  to list objects. They aren't a place for anything sensitive.
- No tag directory exists: a tag is whatever some object carries, and it
  disappears with its last object.
- The MCP `inject`, `update` and `list` tools take the same fields.
