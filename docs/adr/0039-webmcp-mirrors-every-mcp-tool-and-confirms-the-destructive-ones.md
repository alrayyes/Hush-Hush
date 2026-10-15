# 39. WebMCP mirrors every MCP tool, and confirms the destructive ones in the page

## Status

Accepted. Supersedes [ADR 20](0020-webmcp-tools.md)

## Context

[ADR 20](0020-webmcp-tools.md) shipped WebMCP as two read-only metadata tools
and deferred everything else until someone decided how a destructive tool
should be confirmed. The rule since then (`rules/mcp.md`, alrayyes/hush-hush#970)
is that the HTTP API is the source of truth and every operation is reachable
through the MCP server and through WebMCP, so an agent is never stopped halfway
through a task by a capability only the API or the page has. The MCP side
already follows it: a parity test fails when an operation has no tool and no
written reason.

The confirmation question ADR 20 deferred has an answer that is less helpful
than it looked. Read against the current draft of the
[WebMCP spec](https://webmachinelearning.github.io/webmcp/) (October 15, 2026),
The `client.requestUserInteraction()` that record named is not in it. The draft
has `registerTool` taking a callback named `execute(input, { signal })`, and tool
annotations `readOnlyHint` and `consequentialHint`. The spec says the second
"acts as a signal to the client or agent", which can then "selectively enforce
mandatory user confirmation prompts". Nothing makes the browser, or an agent
runtime, do so. A runtime that ignores the hint runs the tool.

A WebMCP tool runs in the signed-in page, as the signed-in user, through the
page's own API client. That is a bigger reach than a scoped bearer token: the
session can rename and delete passkeys, mint write tokens and delete any secret.

## Decision

**Every API operation gets a WebMCP tool, or a written reason it doesn't.** The
same parity rule as MCP, checked by a unit test over the operation IDs in
`api/openapi.yaml`. A tool calls the page's existing `api.ts` function for its
operation, so it takes the same route, the same authorisation and the same
errors as the page itself. WebMCP keeps its own exclusion list, separate from
the spec's `x-mcp-excluded`, because it runs as a session and the MCP server
does not: the passkey ceremonies, logout, the probes and the `/mcp` endpoint
stay out, and credentials and owner tokens, which MCP excludes as session-only,
are in.

**Annotate every tool.** `readOnlyHint` on reads, `consequentialHint` on a tool
that deletes, overwrites, revokes, purges, renames a consumer, or mints or
rotates a token. That is the spec's own signal, so a client that honours it
gets the prompt it wants.

**Do not rely on the signal.** Each consequential tool also opens the app's own
confirmation dialog for the signed-in person before it acts, the same friction
as the page's delete and revoke flows. It rejects if the person cancels, or does
not answer before the call is aborted. The dialog is the page's, so it works
whatever the agent runtime does.

**Keep the registration where ADR 20 put it:** feature-detected on
`document.modelContext`, in the authenticated layout, a silent no-op in every
browser without the API.

## Consequences

- An in-browser agent can do everything the owner can do in the page, after a
  human click on anything consequential. Without the dialog the hint would have
  been all that stood between a runtime and `delete`.
- The dialog is a gate on the page, not on the API. A script that calls the API
  with a session cookie is unaffected, as it was before.
- The tool list grows from 2 to about 30. Tool-selection accuracy falls as a
  server lists more, and `rules/mcp.md` allows folding several operations behind
  one argument named `action`. This record does not: one tool per operation is the
  starting point, and a later record can fold them if an agent struggles.
- Nothing is user-visible until a browser ships `document.modelContext` by
  default, as in ADR 20. The tests run against a stand-in `modelContext`; the
  real registration path is still checked by hand against a flag-enabled Chrome.
- If the spec gains a browser-enforced confirmation, the dialog becomes
  redundant for clients that use it, and a new record can drop it.
