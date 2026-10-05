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
  and the 413 example text. The spec test is what keeps them honest.
- The other request fields (slug, description, names) get their own limits in
  a follow-up, as part of the same ticket.
