# 37. A variant can be picked by the consumer it names

## Status

Accepted. Extends the selector of [ADR 33](0033-one-name-many-variants.md),
which only took a UUID.

## Context

A write token or a session has to say which variant of a name it means once a
name has several. ADR 33 gave it `?id=<uuid>` and nothing else. A person adding
consumers to one variant of a shared name knows who reads the value, not its
UUID, and the CLI had no way to show them one.

## Decision

- **`GET`, `PUT`, `PATCH`, `DELETE /objects/{slug}` and `GET
/objects/{slug}/used-by` also take `?consumer=<name>`.** It picks the one
  variant whose consumers include that name. The store already resolved
  this for consumer tokens, so the API only exposes it.
- **`id` and `consumer` together are a 400.** They say the same thing two ways
  and could disagree.
- **A consumer in none of the name's variants is a 404**, even when the name has
  a single variant. The selector is strict, so a typo can't act on a variant
  the caller didn't mean. ADR 33's invariant (a consumer is in at most one
  variant) is what makes the answer unique.
- **A name with several variants still needs one of the two.** Without either
  it stays a 409.
- **A consumer token's own consumer wins** over the query, so a token can't
  read another consumer's variant by naming it.
- **A query parameter, not a path.** The id names a variant; a consumer only
  finds one, because a variant has many. Consumer names contain slashes
  (`owner/repo`), which a path segment would have to encode. Vault's KV API
  also selects a version with `?version=` on the same path
  ([Vault KV v2](https://developer.hashicorp.com/vault/api-docs/v1.16.x/secret/kv/kv-v2)).

Rejected: `/objects/{slug}/variants/{id}`. It suits listing and addressing by
id, but needs the id first, which is the problem being solved.

## Consequences

- `consumer` is limited to 128 characters like the other query strings, a 422
  beyond that.
- In an update, `consumer` names a member of the variant before the change. A
  body that changes `used_by` does not change which variant was picked.
- The SDKs and CLI get the parameter when the clients regenerate.
