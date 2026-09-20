# Proposal

## Why

Registered passkeys already show when each one was last used
(`webauthn_credentials.last_used_at`, `Credential.LastUsedAt` in
`api/openapi.yaml`), which is exactly what lets an admin tell a passkey
that's actually in use from one that's been sitting idle. Write bearer
tokens have no equivalent - `TokenMetadata` has `created_at` and
`expires_at` but nothing that says whether, or when, a token has ever
actually authenticated a request. An admin auditing tokens today can't
tell an unused token from one in daily use, which is exactly the signal
that matters when deciding whether to revoke one.

## What Changes

- Track a write token's most recent successful authentication as a
  `last_used_at` timestamp, updated the same way
  `UpdateCredentialUsage` already updates a passkey's.
- Expose `last_used_at` on `TokenMetadata` (`GET /tokens`, and the
  creation response), absent/empty for a token that's never
  authenticated a request.
- Settings page's token list shows the last-used time alongside
  created-at and expires-at, matching how the passkey list already shows
  it.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `tokens`: adds last-used tracking to token metadata. (No main spec
  exists for `tokens` yet - `openspec/changes/web-ui/specs/tokens/spec.md`
  is still an in-flight delta - so this is declared as an addition here
  rather than a modification of an archived requirement.)

## Impact

- `internal/store/tokens.go`: new `last_used_at` column (via the existing
  `addColumnIfMissing` migration helper, same pattern as `owner` and
  `revoked_at`) and a new `UpdateWriteTokenUsage` method.
- `internal/api/server.go`: `requireWriteAccess` calls
  `UpdateWriteTokenUsage` after a successful token authentication.
- `internal/api/tokens.go`: `TokenMetadata` gains `LastUsedAt`.
- `api/openapi.yaml`: `TokenMetadata` schema gains `last_used_at`.
- `cmd/hush-hush/web/src/routes/(app)/settings/+page.svelte` (or wherever
  the token list renders): shows the new field.
