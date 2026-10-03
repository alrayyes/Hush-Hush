# 29. A token lasts 90 days by default and at most 365

## Status

Accepted. Adds a limit to the token model of
[ADR 23](0023-token-rotation-in-place.md) and
[ADR 24](0024-consumer-scoped-read-tokens.md); changes nothing about how a
token is checked.

## Context

`ttl_seconds` had a minimum of 1 and nothing else. The server rejected zero or
less and accepted anything above, so every client invented its own default (the
web UI copied 90 days into four dialogs) and nothing capped how long a leaked
token could stay valid. A default and a maximum are retention policy, and a CLI
or an SDK needs the same answer as the UI
([issue #537](https://github.com/alrayyes/hush-hush/issues/537), which also
holds the comparison with other systems).

## Decision

For write tokens and consumer read tokens, on both create and rotate:

- the default is **90 days** (7776000 seconds), applied when a request leaves
  `ttl_seconds` out;
- the maximum is **365 days** (31536000 seconds), and a longer value is a 422
  whose message names the limit;
- zero or less is still a 400.

The limit applies when a token is minted or rotated. A token issued before it
existed keeps working until it expires, so an upgrade breaks nothing, and
rotating one applies the new maximum. The local `hush-hush token issue` and
`rotate` commands, which write straight to the store, hold the same limit. The
constants live in `internal/api`, the spec states both as `default` and
`maximum` on one shared `TokenTtlSeconds` schema, and a test fails if the two
disagree.

Why a year. These are machine credentials for a CI job or a host reading
secrets, and they have a rotate endpoint, so they have to outlive a rotation
cycle. A year is also GitHub's ceiling for a fine-grained token. A month, which
is where Vault's own guidance and common refresh-token practice land, fits
interactive or renewable credentials and would force a rotation job on every
consumer.

## Consequences

- **`ttl_seconds` stays `required` in the schema**, though the server also
  accepts a request that omits it. Making it optional is the obvious fix and
  was tried first, but it changes generated client types (Go's `int64` becomes
  `*int64`). An API diff tool calls that spec change non-breaking, so the SDKs
  would ship it as a patch release that stops compiling for callers such as
  the CLI. A client that omits the field anyway gets the default.
- A leaked token is now bounded at a year instead of at whatever its minter
  typed. That is a ceiling, not a recommendation: the 90-day default is the
  lifetime most tokens should have.
- Changing either number touches three places, the constants, the spec and the
  UI's generated limits, and the test that compares the first two is what keeps
  them honest.
