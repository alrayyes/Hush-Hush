# 18. Resolve the /audit-log route collision with Sec-Fetch-Dest, not a new path

## Status

Accepted

## Context

[ADR 15](0015-no-api-prefix-for-web-ui-routing.md) kept every API route at
its existing, unprefixed path and predicted the consequence: "any
brand-new API surface added later has to pick a path that doesn't collide
with a plausible SPA route (or vice versa)." `/audit-log` is both the
`GET /audit-log` API endpoint and a SvelteKit page route
(`(app)/audit-log`), which already existed before that consequence was
written down. A hard navigation to it - a refresh, a bookmark, a
Playwright `page.goto` - is a real HTTP request, and Go's `ServeMux`
routes the exact path match to the API handler before the SPA's static
fallback ever sees it, so the browser got the raw JSON array instead of
the page (alrayyes/hush-hush#272).

Renaming or prefixing the API endpoint was the first option considered
and the one ADR 15 already rejected for the same reason: `/audit-log` is
the published contract every generated SDK and `hush-hush-cli` call.
Renaming the SvelteKit page route instead doesn't fix the reported bug -
the acceptance criteria require a hard navigation to `/audit-log`
specifically to render the page, and moving the page elsewhere leaves
that exact path serving JSON.

## Decision

`/audit-log` keeps one registered path, served by a router that decides
per request rather than the mux deciding once at registration time. The
decision uses the `Sec-Fetch-Dest` [Fetch Metadata request
header](https://developer.mozilla.org/en-US/docs/Glossary/Fetch_metadata_request_header):
a browser sets it to `document` only for a top-level navigation (address
bar, refresh, a bookmark, `page.goto`) and never for a `fetch()` call
(the SPA's own client-side load), and page script can't override it. A
request with `Sec-Fetch-Dest: document` gets the SPA's `index.html`.
Every other request gets the existing JSON handler, including one with no
`Sec-Fetch-Dest` at all: every non-browser caller (curl, a generated SDK,
`hush-hush-cli`).

## Consequences

- The published API contract is untouched: any caller that isn't a
  browser performing a real navigation gets exactly the response it got
  before this change.
- The fix is contained to `/audit-log`'s own registration - no new prefix,
  no change to any other route.
- The next page/API path collision (a plausible one: `#252`'s consumer
  directory page vs. a future `/consumers/{id}` API route) has a proven
  pattern to reach for instead of re-litigating renaming-vs-prefixing
  from scratch, though picking a non-colliding path up front is still
  simpler where that's still an option.
- This relies on browser support for Fetch Metadata request headers,
  universal in evergreen browsers but not sent by an old one or by a
  browser automation tool that doesn't set it - such a client falls
  back to the JSON response on a hard navigation to `/audit-log`, the
  same behaviour this ADR fixes for everyone else. No API endpoint
  degrades because of this: the fallback is today's existing behaviour,
  not a new failure mode.
