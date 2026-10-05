# 34. A list is one page of 50 unless it asks for more

## Status

Accepted. Finishes what [ADR 31](0031-request-body-and-value-size-limits.md)
left waived for the four paged lists. A breaking change to the API.

## Context

`GET /objects`, `/tokens`, `/consumer-tokens` and `/credentials` returned every
row, so one call could return an array of any size
([issue #662](https://github.com/alrayyes/hush-hush/issues/662)). Paging was
added as opt-in first ([issue #649](https://github.com/alrayyes/hush-hush/issues/649)),
because the clients read the plain array and a smaller default would have cut
off what they saw. The web UI now reads every page
([#678](https://github.com/alrayyes/hush-hush/pull/678)) and so does the Go SDK
([hush-hush-go#231](https://github.com/alrayyes/hush-hush-go/pull/231), released
as v4.4.0). `hush-hush-cli` reads its lists through that SDK. The Node, PHP and
Python SDKs have no method for these lists.

## Decision

- A list request with no `limit` is **one page of 50 rows**, from `offset` 0.
  `limit` still takes 1 to 500, and `X-Total-Count` always says how many rows
  there are in all, so a client reads the rest by asking for the next
  `offset`.
- The array stays the response shape, so no generated SDK type changes.
- The spec states a `maxItems` of 500 on each of the four response arrays, and
  `.spectral.yaml` no longer waives them. Two arrays stay waived: the plain,
  unpaged `GET /consumers` response, which `page` and `page_size` already
  replace for a client that wants a bound, and the audit log's filter options,
  which are derived lists.

## Consequences

- **A client that doesn't page sees only the first 50 rows.** It doesn't fail,
  so the change is silent for it. That is why it shipped after the web UI and
  the Go SDK paged, and why it is a breaking change in the release notes. Any
  other caller has to read `X-Total-Count`.
- Paging slices the rows the handler has loaded, so it bounds the response and
  not the database read. That is fine at this scale. If it stops being fine,
  the fix is a query with `LIMIT` and `OFFSET`, behind the same API.
- The MCP `list` tool reads the store directly, so it pages on its own
  ([issue #685](https://github.com/alrayyes/hush-hush/issues/685)): the same
  `limit` and `offset`, 50 by default. Its structured result stays an array,
  and the total is in the result's text, since a tool result has no header to
  carry it. An agent that ignores the text sees only the first page.
