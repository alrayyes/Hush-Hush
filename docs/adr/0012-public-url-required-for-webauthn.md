# 12. Require `PUBLIC_URL`; never derive WebAuthn's origin from headers

## Status

Accepted

## Context

WebAuthn needs a fixed relying-party ID and origin per deployment. The
service's `api/openapi.yaml` `servers` block deliberately has no canonical
host - `scheme` and `host` are variables, because the service runs
anywhere - so the relying-party origin had to be configured the same
deployment-agnostic way, without reintroducing a hardcoded host.

## Decision

A new `PUBLIC_URL` environment variable (for example
`https://secrets.example.com`), required once any WebAuthn registration is
attempted. `RPID` and `RPOrigins` are derived from it, not configured
separately. If unset, WebAuthn endpoints respond with a clear configuration
error rather than guessing a host from request headers.

Guessing the origin from `Host`/`X-Forwarded-Proto` was considered and
rejected: it's a known WebAuthn phishing risk, since an attacker-controlled
`Host` header could otherwise shift the relying party. This is one new
required setting for whoever enables the web UI, consistent with the rest
of the service's environment-variable configuration; it does not change
`ADDR`'s existing meaning (the bind address, unrelated to the public origin
a browser sees).

## Consequences

- Enabling the web UI requires one new, explicit piece of configuration -
  not automatic the way the rest of the service's host-agnostic design
  otherwise is.
- Changing `PUBLIC_URL` after credentials are registered invalidates them,
  since WebAuthn binds credentials to an origin - see the risk this design
  accepted and documented in the `web-ui` change's design notes, and
  documented prominently in the README next to the variable.
- No header-based origin inference exists anywhere in the WebAuthn path,
  closing off that phishing vector by construction rather than by review.
