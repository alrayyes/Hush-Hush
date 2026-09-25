# Proposal

## Why

Hush-Hush ships an embedded web UI for managing objects and their
`used_by` metadata, but it's a page a human clicks through - nothing on
it is exposed to an in-browser agent. WebMCP lets a page declare tools an
agent running alongside the browser can call directly instead of driving
the DOM, which for a secrets-management UI means an agent helping someone
audit or clean up stored objects can query the page's own state directly
rather than screen-scraping it (alrayyes/hush-hush#347).

Research surfaced a real complication the ticket didn't anticipate:
`document.modelContext.registerTool()` is a Chrome/Edge **origin-trial**
API as of 2026-09-25 (through 2026-11-17; "ship by default" only
_proposed_, not committed, for Chrome M157), undocumented on MDN, with no
Firefox/Safari commitment. Origin-trial tokens are bound to one
registered domain, which doesn't fit a self-hosted app - there's no
token hush-hush itself could ship that would work for every deployer's
own domain. Decided: build it anyway, feature-detected
(`document.modelContext?.registerTool`, a silent no-op everywhere the API
doesn't exist yet) - no origin-trial token, no third-party polyfill
(`@mcp-b/*` is an unofficial, non-W3C project). Kept low-risk by staying
read-only and metadata-only.

## What Changes

- Two WebMCP tools registered in the authenticated web UI:
  `list_objects` and `get_object_metadata`, both wrapping the existing
  `listObjects()` call in `cmd/hush-hush/web/src/lib/api.ts` - no new
  backend endpoint, no new `api.ts` wrapper.
- Neither tool ever returns a secret's sealed value, and neither performs
  a destructive operation - inject/rotate/delete stay out of scope,
  deferred by the ticket to a future decision on confirmation UX.
- Registered from `src/routes/(app)/+layout.svelte`'s `onMount`, the
  authenticated route group's layout - unauthenticated pages never
  register tools that would just 401 on every call.

## Capabilities

### New Capabilities

- `webmcp`: WebMCP tool declarations mirroring the web UI's existing
  read-only object metadata views.

### Modified Capabilities

_None._

## Impact

- `cmd/hush-hush/web/src/lib/webmcp.ts` (new): `registerWebMCPTools()`,
  feature-detected, calling `listObjects` from `./api.ts`.
- `cmd/hush-hush/web/src/lib/webmcp.d.ts` (new): a minimal ambient
  `Document.modelContext` declaration covering only the `registerTool`
  shape this file calls - not in `lib.dom.d.ts` yet.
- `cmd/hush-hush/web/src/routes/(app)/+layout.svelte`: calls
  `registerWebMCPTools()` on mount.
- `cmd/hush-hush/web/src/lib/webmcp.spec.ts` (new): unit tests.
- `cmd/hush-hush/web/README.md`: documents the two tools.
- `docs/adr/0020-webmcp-tools.md`: records the maturity tradeoff and the
  native-API-only, read-only/metadata-only decisions.
- The implementation itself lands in a follow-up PR once this one merges
  (this repo's established pattern of an OpenSpec proposal landing before
  its implementation, for example #253 then #274, #254 then #279).
