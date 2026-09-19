# 11. WebAuthn: `go-webauthn` server-side, `@simplewebauthn/browser` client-side

## Status

Accepted

## Context

The web UI needed passkey-based authentication for its single admin
account, requiring both a server-side WebAuthn relying-party
implementation and a client-side library to drive
`navigator.credentials`.

## Decision

`go-webauthn/webauthn` on the server, `@simplewebauthn/browser` in the
Svelte frontend.

`go-webauthn/webauthn` was confirmed as the maintained Go option - the
successor to the archived `duo-labs/webauthn`, FIDO2-conformant, and
actively maintained. `@simplewebauthn/browser` is a thin wrapper over
`navigator.credentials`, framework-agnostic and callable directly from
Svelte component code; no Svelte-specific WebAuthn binding was found worth
adding as a dependency.

## Consequences

- The relying-party ID/origin configuration this library needs is what
  drives [ADR 12](0012-public-url-required-for-webauthn.md)'s
  `PUBLIC_URL` requirement.
- Sign-counter verification, credential storage shape (credential id, COSE
  public key, sign counter, AAGUID) and cloned-authenticator detection all
  follow `go-webauthn`'s own verification steps rather than being
  hand-rolled.
- A future move away from either library means re-verifying FIDO2
  conformance and re-auditing the registration/authentication ceremony
  code against whatever replaces it - not a drop-in swap.
