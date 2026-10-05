# 30. Rate limiting is the reverse proxy's job

## Status

Accepted. Adds to [ADR 2](0002-unauthenticated-read-path-with-audit-log.md)'s
view of the unauthenticated routes: the audit log is the control there, and
slowing a caller down is not.

## Context

The server has no rate limiting. The routes that matter are the four
unauthenticated routes under `/auth/register` and `/auth/login`, and every
route that checks a bearer or consumer token, since a token is all that stands
between a caller and the data
([issue #618](https://github.com/alrayyes/hush-hush/issues/618)). Tokens are
long random values, so guessing one isn't practical today, and the audit log
records every failed check. This is hardening, not a live defect.

The Spectral OWASP ruleset flags it as `owasp:api4:2023-rate-limit` and
`rate-limit-responses-429`.

## Decision

The reverse proxy in front of the server rate limits. The server doesn't, and
the spec doesn't document a 429 or rate-limit headers it never sends.
`.spectral.yaml` waives both rules for good, with that reason beside them.

Why the proxy:

- A per-address limit needs the real client address. Behind a proxy the server
  sees the proxy's, so it would have to trust a forwarded header and know which
  hops to believe, which is its own source of bugs.
- The proxy already terminates TLS and is where a deployment tunes limits, so
  an operator changes one setting in one place for every service behind it.
- The server holds no per-client state today. A limiter would add some, and a
  restart would reset it.

## Consequences

- **A deployment that exposes the server directly has no rate limit.** The
  README says to put a proxy in front of it and limit `/auth/*` and the
  token-checked routes.
- The 429 and the rate-limit headers stay out of the spec. If the server ever
  limits itself, this ADR is superseded and both rules go back to errors.
- Request size is different. The server enforces its own body limits, because
  the proxy's cap can't know what a sealed value is allowed to weigh
  ([issue #617](https://github.com/alrayyes/hush-hush/issues/617)).
