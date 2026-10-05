# 33. One name can hold a different value per consumer, each under a UUID

## Status

Accepted. Changes two things in [ADR 22](0022-object-id-slug-split.md): the
object's id is now a UUID that the API returns, and the slug is no longer
unique. Narrows how [ADR 24](0024-consumer-scoped-read-tokens.md) finds the
object a consumer token may read. Leaves [ADR 1](0001-per-object-age-encryption.md)
alone: every value is still sealed to its own consumers' keys.

## Context

The slug was the unique key, so a second value needed a second name, and every
consumer's configuration had to know which name was its own. The need
([issue #668](https://github.com/alrayyes/hush-hush/issues/668)) is one name,
`release_token`, that `consumer_a`, `consumer_b` and `consumer_c` read as
`tokena` while `consumer_d` reads it as `tokenb`. The unique id should be a
UUID, and the name should be free to repeat.

## Decision

- **An object's id is a UUID**, returned as `id` on every object the API
  describes. Existing hex ids are replaced by UUID ids when the database
  opens, with the `used_by` and `tags` rows that reference them.
- **The slug (the name) is not unique.** Objects that share one are _variants_.
  Each is an ordinary object with its own sealed value and its own `used_by`
  list. A plain index replaces the unique one.
- **A consumer is in at most one variant of a name.** That invariant is what
  lets a consumer ask for a name and get one value. It is checked in the
  transaction that writes, so concurrent writes can't both break it.
- **A consumer token reading `GET /objects/{name}`** gets the variant whose
  `used_by` names its consumer. A name with no variant for that consumer is a
  404, the same as an unknown name, so nothing leaks about which variants exist.
- **A write token or a session** has no consumer. `GET`, `PUT`, `DELETE` and
  `GET .../used-by` take an optional `id` query parameter naming the variant;
  it must belong to that name, or the answer is a 404. Without it, a name with
  one variant behaves as before and a name with several is a 409.
- **`POST /objects` on a taken name** makes a new variant when none of its
  `used_by` consumers already has one, and is a 409 otherwise, as is a repeat
  with an empty `used_by`. An update or a consumer rename that would put one
  consumer in two variants is a 409 and changes nothing.
- The MCP tools follow the same rules, taking an optional `id`.

Rejected: a `consumer` query parameter instead of `id`. It would work for a
write token, but a UUID names exactly one variant even when its consumer list
later changes, and it gives the web UI something to key a row on.

## Consequences

- **The id is exposed after all.** ADR 22 kept it internal so a slug could be
  renamed. That still holds, because nothing else references the slug, and a
  UUID is safe to show.
- **An audit entry names its variant.** `object_id` stays the name, so an
  entry still groups by name, and a new `variant_id` holds the object's UUID
  ([issue #684](https://github.com/alrayyes/hush-hush/issues/684)). An entry
  written before that has none.
- **Clients keyed by name see a name twice in a list.** `GET /objects` returns
  every variant. The web UI keys its lists by slug, so showing variants is its
  own change.
- **A rename can now be refused.** Renaming consumer `a` to an existing `d`
  merges them, which would put `d` in two variants of one name, so it is a 409.
- A consumer that needs two different values under one name can't have them:
  it is one consumer, so it reads one.
