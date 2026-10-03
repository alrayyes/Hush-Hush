# 28. The server reads an uploaded value's age header, and nothing more

## Status

Accepted. Narrows one assumption in [ADR 1](0001-per-object-age-encryption.md)
without changing its guarantee: the server still never sees plaintext.

## Context

The client seals a value before upload, so the server stores ciphertext it
can't read. That left it unable to refuse a broken value: an empty string, a
plaintext note or a truncated file was stored as readily as a real age file,
and every client had to be trusted to have sealed correctly. Any client can
get that wrong, the Go CLI and the SDKs as much as the web UI, so the check
is a rule the API should own
([issue #538](https://github.com/alrayyes/hush-hush/issues/538)).

The server can't know how many recipients a value was sealed to in a way it
could judge, or whether any recipient's key is the right one, so "at least one
recipient" in the dialogs is the client's check and stays there.

## Decision

`POST /objects`, `PUT /objects/{slug}` and the MCP `inject` and `update` tools
refuse, with a 422, a value that isn't base64 of a well-formed age file or
whose header lists no recipient stanza. Nothing is stored, and an update
leaves the old value alone.

The check is age's own header parser, run through an identity that never
matches. It reads the header and stops. No key is involved and no payload is
read, so nothing is decrypted. The parser is `filippo.io/age`, the reference
implementation, rather than a parse of its own for a format that has its own
versioning.

## Consequences

- A client that uploaded placeholder values no longer works. That is the
  point, and it cost a fixture update across the server's tests and the
  frozen CLI pact snapshot.
- The server now depends on `filippo.io/age`, for parsing only.
- A value can still be sealed to the wrong recipient, or to a recipient
  nobody holds the key for. The server can't see that and doesn't try. The
  dialogs' recipient guard is unchanged.
- Age files in the text encoding (`-----BEGIN AGE ENCRYPTED FILE-----`) are
  rejected. Every client here seals the binary form, and the wire format has
  always been base64 of that.
- age has no typed error for "no recipients", so that one case is told apart
  by its message. A test pins it, so a library upgrade that rewords it fails
  loudly instead of quietly turning the message into the generic one.
