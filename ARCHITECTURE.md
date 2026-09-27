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
- **Read path**: unauthenticated - confidentiality comes from who holds a
  matching age private key, not from a server-side check.
- **Addressing**: `objects.id` is an internal identifier; every
  caller-facing path (the API, the CLI, the audit log) addresses an
  object by its `slug` instead, decoupled so a slug can be renamed later
  without touching anything that references the object internally.
- **Web UI**: SvelteKit, built static and embedded into the Go binary with
  `go:embed`; passkey (WebAuthn) authentication.

## Why it's built this way

The reasoning behind each of those choices - what was tried and rejected,
and why - is recorded as it's decided, in
[`docs/adr/`](docs/adr/). Read the ADRs there for the "why"; this file is
only the current shape.

An OpenSpec change's own `design.md` (under `openspec/changes/`) is that
one change's working paper, not a durable record - it gets archived once
the change ships. A decision worth remembering after that gets its own ADR
here, not left to be dug out of the archive.
