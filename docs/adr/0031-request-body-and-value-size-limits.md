# 31. A request body is capped at 1.5 MiB and a sealed value at 1 MiB

## Status

Accepted. Adds a limit to the write path of
[ADR 1](0001-per-object-age-encryption.md) and
[ADR 3](0003-single-bearer-token-write-path.md); changes nothing about how a
value is sealed or checked.

## Context

Nothing bounded what the server would read. Ten handlers decoded a JSON body
with no cap, and a sealed `value` is base64 of any size. Four routes under
`/auth/register` and `/auth/login` take a body without authentication, so
anyone who could reach the server could make it read an unbounded amount
([issue #617](https://github.com/alrayyes/hush-hush/issues/617), OWASP API4,
unrestricted resource consumption). The server also set no `ReadTimeout`, so a
slow body was never cut off.

## Decision

- A **sealed value is at most 1 MiB**, counted as the ciphertext, not its
  base64 form. A secret is a token, a key or a small file, not a blob. Over it
  is a 413 on create and update, and on the MCP `inject` and `update` tools.
- **No route reads more than 1.5 MiB of request body.** That is a value at its
  maximum, a third larger once base64 encoded, plus room for the other fields.
  A body that announces a larger `Content-Length` is refused before a byte is
  read. A chunked one is cut off at the limit. Both answer 413 with the usual
  `{error}` body. The cap sits on every route, the four unauthenticated ones
  included, and a route added later gets it too.
- **Request fields have limits too**, declared as `maxLength` and `maxItems`
  struct tags on the request types, so the rule sits beside the field and one
  decode helper enforces them. Over a limit is a 422 naming the field and the
  limit, the same status as an over-long token lifetime. Characters are
  counted, not bytes, the way the spec's `maxLength` counts them.

  | Field                                           | Limit           |
  | ----------------------------------------------- | --------------- |
  | object `slug`, consumer name, `used_by` entry   | 128 characters  |
  | `used_by`                                       | 100 entries     |
  | object `description`                            | 1000 characters |
  | token `description`                             | 200 characters  |
  | credential `nickname`                           | 100 characters  |
  | consumer `public_key`, escrow `public_key`      | 512 characters  |
  | `wrapped_identity`, `recovery_wrapped_identity` | 4096 characters |

  Tags keep their own limits (10 of at most 32 characters). The MCP tools'
  inputs are held to the same ones.

- **Query parameters and headers have limits.** A string query parameter
  (`q`, `used_by`, `object_id`, `caller`, `actor`) is at most 128 characters,
  `from` and `to` 64, a filter takes at most 10 `tag`s, and `X-Caller` and
  `X-CSRF-Token` are at most 128. `X-Caller` is stored in the audit log, so
  it was the one header that mattered most. Over any of them is a 422, or a
  400 for the tag count and the integers (`page` at most 2147483647, `after`
  at least 0), which already answered 400 for a bad value.
- **Everything the server sends is bounded in the spec too**, so every string,
  array and integer has a limit and the three Spectral limit rules are errors
  again. A response string takes the limit of the request field it echoes, and
  a generated value (an id, a timestamp, a token) its fixed length. The one
  exception is the arrays of the list endpoints that return every row
  (`/objects`, `/consumers` without a page, `/credentials`, `/tokens`,
  `/consumer-tokens` and the audit log's filter options): a `maxItems` there
  would be a number the server doesn't keep, so `.spectral.yaml` waives them by
  path. Four of those lists became one page of 50 by default, so their
  arrays no longer need the waiver ([ADR 34](0034-lists-are-one-page-by-default.md)).
- The server sets a **30 second `ReadTimeout`** and a **64 KiB
  `MaxHeaderBytes`**, so a caller that dribbles a body or a header is dropped.
- The spec documents the 413 on every operation that takes a body, and states
  the value limit as `maxLength` on the two `value` schemas. A test fails if
  the spec and the constants disagree.

The reverse proxy's own cap can't stand in for this: it doesn't know what a
sealed value is allowed to weigh, and a deployment without one would have no
limit at all. Rate limiting is different, and is the proxy's job
([ADR 30](0030-rate-limiting-is-the-reverse-proxys-job.md)).

## Consequences

- A value over 1 MiB can no longer be stored. Nothing stored today is that
  large through the web UI or the CLI; a client that needs more has to split
  the secret.
- Changing the value limit touches the constant, the spec's two `maxLength`s
  and the 413 example text. Changing a field limit touches its tag and the
  spec. A test compares each tag with the spec and fails if they disagree.
- A limit is a number a client has already stored data against. These sit well
  above what the web UI and the CLI produce, so nothing existing breaks, but
  raising one later is free and lowering one is not.
