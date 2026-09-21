# Tasks

## 1. Store and API

- [x] 1.1 Extend the distinct-consumer store query with an optional name
      filter, offset/limit, a total count, and a per-consumer secret
      count; verify with a store test covering filtering, pagination
      boundaries, and count accuracy against a fixture with several
      consumers and overlapping `used_by` lists.
- [x] 1.2 Add `page`/`page_size`/`q` query parameters to
      `handleListConsumers`, defaulting to the full unpaginated listing
      when none are given; verify with a Go test covering all three
      parameters and the no-parameters default.
- [x] 1.3 Update `api/openapi.yaml`'s `/consumers` path with the new
      parameters and response shape; verify `bun run lint:api` passes.

## 2. Consumers page

- [x] 2.1 Add `cmd/hush-hush/web/src/routes/(app)/consumers/+page.svelte`
      listing consumers with their secret count, a name filter input, and
      page-number navigation; verify with a component test covering
      filtering and paging.
- [x] 2.2 Selecting a consumer navigates to the secrets overview with
      `used_by` set to that consumer; verify with a component test.
- [x] 2.3 Add a nav link to the consumers page; verify it's reachable from
      every authenticated page's nav.
- [x] 2.4 Add an axe-core assertion (WCAG 2.1 AA) to a journey test
      covering the consumers page; verify zero violations.

## 3. Compatibility check

- [x] 3.1 Confirm `consumer-combobox`'s no-parameters call against
      `GET /consumers` still returns the shape it expects after this
      change; update that call site if the response wrapping changed.

## 4. Verification

- [x] 4.1 `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` pass.
- [x] 4.2 `bun run test`, `bun run check`, `bun run lint`, and
      `bun run format:check` pass in `cmd/hush-hush/web`.
- [x] 4.3 Manually verified against the running binary: the consumers
      page paginates and filters correctly, and selecting a consumer opens
      the secrets overview filtered to it.
