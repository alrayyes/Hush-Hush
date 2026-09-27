# 21. Each user holds one escrowed writer identity, wrapped per PRF-capable passkey

## Status

Accepted. Extends [ADR 1](0001-per-object-age-encryption.md) rather than
revising it.

## Context

[ADR 1](0001-per-object-age-encryption.md) established that an object is
sealed to recipient public keys chosen at write time, with the server never
holding a decryption key. Once the web UI seals client-side (rather than
accepting an already-sealed value), the admin writing a secret needs a real
age identity of their own - both to optionally read back a secret they
opted into as a recipient, and as the thing a `users` row actually
represents ([ADR 13](0013-cookie-session-not-jwt.md)'s session authenticates
a person, but a person didn't yet have a cryptographic identity of their
own). The question was how to give a browser-based user a private key that
survives a lost device without the server ever holding it in the clear.

## Decision

Each user's account holds exactly one escrowed writer identity: an age
keypair generated client-side, once, at first registration. Its private
key is never sent to the server unwrapped - the server only ever stores
wrapped copies of it, plus its public key (safe to store; needed so other
code can resolve "this user's own key" as a decrypt recipient later).

Each of a user's WebAuthn credentials that supports the PRF extension
additionally derives a PRF secret at registration time and uses it to wrap
a copy of the escrowed identity, stored alongside that credential. Losing
one credential, while at least one other PRF-capable credential (or the
break-glass recovery phrase) remains, does not strand the identity -
`DELETE /credentials/{id}` already refuses to remove the last remaining
credential, which is what prevents a self-inflicted total lockout through
that action specifically.

A recovery phrase, generated at the same first-registration moment and
shown exactly once, covers the case a PRF-capable credential can't: an
authenticator that never supported PRF, or every PRF-capable credential
lost outside the app's own delete flow (theft, destruction). The server
retains no copy of the phrase itself after that one display.

The alternative considered was a single passphrase-derived key, entered on
every login. Rejected: it reduces to "remember a password," which passkeys
exist specifically to avoid, and ties recovery to something a user is
exactly as likely to lose or forget as the passkeys themselves.

## Consequences

- A secret's owner is never implicitly a decrypt recipient - opting a
  secret's owner in is a separate, explicit choice at write time, so a
  secret meant only for consumers never depends on any one user's key
  lifecycle. This is the direct consequence of not wanting the escrowed
  identity above to become a single point of failure for the whole store.
- An account whose owner never had a PRF-capable authenticator, and never
  exported the recovery phrase, has a genuinely unrecoverable identity if
  every credential is lost outside the app's own flow. This is a feature
  of the design, not a gap: the same guarantee that keeps the server from
  ever being a decryption oracle also means it can't rescue this case.
- Adding a user's own key as a decrypt recipient (the opt-in above) ties
  that specific secret's long-term readability to that one person's
  continued custody of a passkey or the recovery phrase - worth choosing
  deliberately per secret, not as a default reflex.
- Multi-user account handoff (a different person taking over an existing
  account) is out of scope for this decision and remains a manual,
  cooperative re-sealing exercise if it ever needs solving - see
  `openspec/changes/client-side-encryption/design.md`'s deferred-scope
  section.
