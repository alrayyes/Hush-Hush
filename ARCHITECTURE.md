# Architecture

A lightweight, standalone secrets object store: public-key ([age](https://github.com/FiloSottile/age))
encryption end to end, so the server stores and serves sealed ciphertext and
never computes or returns plaintext. A single Go binary embeds its own web
UI, and everything - CLI, CI, and the browser - talks to the same API.

- **Storage**: SQLite (`modernc.org/sqlite`), one process, one writer.
- **Identity**: every account is a real `users` row (one exists today;
  the schema and security model already account for more than one). Each
  user holds a single escrowed writer identity - an age keypair generated
  client-side once - wrapped separately under each of their
  PRF-capable WebAuthn passkeys, so losing one passkey never strands it
  while another remains, plus a break-glass recovery phrase for when none
  do. The server only ever stores wrapped copies of the private key and
  the identity's public key, never the private key in the clear.
- **Write path**: a bearer token, or a web-UI session, both audit logged.
  The web UI seals a secret's value client-side - age-encrypted in the
  browser to its recorded consumers' registered public keys, before the
  request ever leaves it - rather than accepting plaintext or requiring
  the writer to seal it by hand outside the browser first; a bearer-token
  write still sends ciphertext already sealed by whatever produced it,
  since the API itself never seals or unseals anything either way. A
  secret's owner is recorded as accountability metadata, not an implicit
  decrypt recipient - including the owner's own key as a recipient is an
  explicit, per-secret, opt-in choice.
  The API does read an uploaded value's age header: a value that isn't a
  well-formed age file, or whose header names no recipient, is refused with
  a 422. It still never decrypts anything
  ([ADR 28](docs/adr/0028-server-reads-the-age-header-and-nothing-more.md)).
- **Read path**: a write bearer token, a web-UI session, or a consumer
  read token scoped to that object's recorded consumers - confidentiality
  still comes from who holds a matching age private key, not from this
  check, but a credential is now required to even request the
  ciphertext.
- **Tokens**: a write bearer token and a consumer read token each carry a
  lifetime, 90 days unless the request says otherwise and at most 365,
  enforced by the API and stated in the spec
  ([ADR 29](docs/adr/0029-token-lifetime-default-and-maximum.md)). A token is
  rotated in place, revoked and, once dead, purged
  ([ADR 23](docs/adr/0023-token-rotation-in-place.md),
  [ADR 17](docs/adr/0017-token-revocation-soft-delete.md),
  [ADR 25](docs/adr/0025-token-purge-hard-delete-for-dead-tokens.md)), and a
  write token can mint consumer read tokens
  ([ADR 26](docs/adr/0026-write-token-mints-consumer-tokens.md)).
- **Tags**: an object carries free-form labels for grouping and filtering,
  stored beside its consumer list, not inside the sealed value
  ([ADR 27](docs/adr/0027-object-tags.md)).
- **Addressing**: `objects.id` is an internal identifier; every
  caller-facing path (the API, the CLI, the audit log) addresses an
  object by its `slug` instead, decoupled so a slug can be renamed later
  without touching anything that references the object internally.
- **Web UI**: SvelteKit, built static and embedded into the Go binary with
  `go:embed`; passkey (WebAuthn) authentication.
- **Operations**: one structured JSON log line per request, never carrying a
  query string, a header, a cookie or a body. `/readyz` bounds its database
  check and caches the answer for a few seconds, and goes 503 the moment a
  shutdown starts. The server then keeps serving for a few seconds so a load
  balancer notices, stops accepting, and waits for in-flight requests before
  the process exits.
- **Rate limiting**: none in the server. The reverse proxy in front of it
  limits the unauthenticated and token-checked routes
  ([ADR 30](docs/adr/0030-rate-limiting-is-the-reverse-proxys-job.md)).

## Why it's built this way

The reasoning behind each of those choices - what was tried and rejected,
and why - is recorded as it's decided, in
[`docs/adr/`](docs/adr/README.md), which has an index. Read the ADRs there for
the "why"; this file is only the current shape.

An OpenSpec change's own `design.md` (under `openspec/changes/`) is that
one change's working paper, not a durable record - it gets archived once
the change ships. A decision worth remembering after that gets its own ADR
here, not left to be dug out of the archive.
