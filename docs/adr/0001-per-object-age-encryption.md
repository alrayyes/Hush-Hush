# 1. Encrypt each object with age, sealed to its own recipients

## Status

Accepted

## Context

The service's core goal is that it never handles or persists plaintext, at
any point. That rules out a design where the server does any decryption on
a reader's behalf - the question is how a single stored object reaches
possibly more than one legitimate reader without the server mediating.

## Decision

Each object is sealed with [age](https://age-encryption.org/), per-object,
to one or more recipient public keys chosen at write time. A secret shared
across several consumers gets multiple recipients on the same object,
rather than the server managing that fan-out itself.

The alternative considered was a single universal service keypair, with
the server decrypting and re-encrypting per reader on each fetch. Rejected:
that turns the server into a live decryption oracle, gated only by
request-time authorization logic - a materially larger blast radius than a
store that structurally cannot produce plaintext at all, even under a bug
or a compromised process.

## Consequences

- The write-time recipient list is the only way to grant a new reader
  access to an existing secret; there's no server-side sharing mechanism to
  extend later without touching the object itself.
- A reader with no matching private key gets ciphertext, not an access
  error - confidentiality holds independent of any server-enforced check.
- Rotating who can read a secret means re-sealing the object to a new
  recipient list, not flipping a permission.
