# 20. WebMCP tools, native API only, read-only and metadata-only

## Status

Accepted

## Context

The embedded web UI is a page a human clicks through, with nothing on it
exposed to an in-browser agent. WebMCP lets a page declare tools an agent
running alongside the browser can call directly - for an agent helping
someone audit or clean up stored objects, that means querying the page's
own state instead of screen-scraping it (alrayyes/hush-hush#347).

Researching the actual state of WebMCP as of 2026-09-25 turned up a real
complication: `document.modelContext.registerTool()` is a Chrome/Edge
**origin-trial** API (through 2026-11-17; "ship by default" is only
_proposed_, not committed, for Chrome M157 around 2026-11-03), undocumented
on MDN, with no Firefox or Safari commitment. Origin-trial tokens are
issued per registered domain - fine for a single SaaS site, but hush-hush
is a self-hosted app with an arbitrary deployer-chosen domain, so there's
no token the project itself could ship that would work everywhere it
runs. A third-party polyfill (`@mcp-b/webmcp-polyfill` and friends) exists
but is explicitly "not an official W3C or MCP project," and its fuller
sibling `@mcp-b/global` bridges to the full server-side MCP protocol - a
bigger commitment than "declare a tool for an in-page agent."

The platform's own security model is also thinner than it might look:
registering a tool isn't gated by any browser consent prompt (any script
on the page can call `registerTool`), and the closest thing to a
destructive-action confirmation gate -
`annotations.consequentialHint` plus `client.requestUserInteraction()` -
is advisory, enforced by the tool's own code, not by the browser.

## Decision

**Build it now, feature-detected, no token and no polyfill.**
`document.modelContext?.registerTool(...)` - a silent no-op in every
browser without the API (which is every browser today, short of a manual
developer flag). No origin-trial token: none would work across every
self-hosted deployment. No `@mcp-b` dependency: it's unofficial, and its
fuller form is more machinery than this ticket needs. The code is inert
until a browser ships this by default, at zero ongoing cost or risk in
the meantime.

**Read-only, metadata-only, exactly two tools**: `list_objects` and
`get_object_metadata`, both wrapping the existing `listObjects()` call in
`cmd/hush-hush/web/src/lib/api.ts` - no new backend endpoint, no new
`api.ts` wrapper, no path to a secret's sealed value. This sidesteps the
platform's thin consent model entirely: since registration itself isn't
gated and there's no destructive tool here, there's nothing that needs a
`requestUserInteraction()` confirmation gate yet. That question is real
but deferred, same as the ticket's own "Approach" section defers
inject/rotate/delete pending an explicit confirmation-UX decision.

**Registered in the authenticated route group's layout**
(`src/routes/(app)/+layout.svelte`), not the root layout. Both tools
401 without a session; registering them somewhere already gated on one
existing avoids a tool that's discoverable but useless before login.

## Consequences

- Nothing here is user-visible or functional in a shipped browser today.
  The value is entirely forward-looking - ready the moment (if) a browser
  ships `document.modelContext` by default, with no code to write later.
- If WebMCP's shape changes again before it stabilizes (it already moved
  once, from `navigator.modelContext` to `document.modelContext`, per
  [`webmachinelearning/webmcp#184`](https://github.com/webmachinelearning/webmcp/issues/184)),
  this file is the one place that needs updating - two tools, one
  registration site, no dependency to bump.
- A destructive tool added later needs its own decision on
  `requestUserInteraction()`-based confirmation; this ADR doesn't answer
  that, and the next one that adds a destructive WebMCP tool should.
- No automated test can currently exercise the real
  `document.modelContext` path - CI's browser has no more WebMCP support
  than any other Chrome without the origin-trial flag. Verification of
  the real registration path is manual, against a flag-enabled Chrome or
  the Model Context Tool Inspector extension.
