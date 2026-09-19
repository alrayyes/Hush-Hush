# Architecture

A lightweight, standalone secrets object store: public-key ([age](https://github.com/FiloSottile/age))
encryption end to end, so the server stores and serves sealed ciphertext and
never computes or returns plaintext. A single Go binary embeds its own web
UI, and everything - CLI, CI, and the browser - talks to the same API.

- **Storage**: SQLite (`modernc.org/sqlite`), one process, one writer.
- **Write path**: a bearer token, or a web-UI session, both audit logged.
- **Read path**: unauthenticated - confidentiality comes from who holds a
  matching age private key, not from a server-side check.
- **Web UI**: SvelteKit, built static and embedded into the Go binary with
  `go:embed`; passkey (WebAuthn) authentication for the single admin
  account.

## Why it's built this way

The reasoning behind each of those choices - what was tried and rejected,
and why - is recorded as it's decided, in
[`docs/adr/`](docs/adr/). Read the ADRs there for the "why"; this file is
only the current shape.

An OpenSpec change's own `design.md` (under `openspec/changes/`) is that
one change's working paper, not a durable record - it gets archived once
the change ships. A decision worth remembering after that gets its own ADR
here, not left to be dug out of the archive.
