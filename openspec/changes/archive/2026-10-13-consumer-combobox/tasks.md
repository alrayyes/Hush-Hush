# Tasks

## 1. Store and API

- [ ] 1.1 Add a store query returning every distinct `used_by` value
      across all objects, sorted; verify with a store test covering a
      consumer used by multiple objects appearing once.
- [ ] 1.2 Add `handleListConsumers` and `GET /consumers`
      (`requireWriteAccess`, read-only) in `internal/api`; verify with a
      Go test asserting the response matches the store query's output.
- [ ] 1.3 Add `/consumers` to `api/openapi.yaml`; verify
      `bun run lint:api` passes.

## 2. Consumer combobox component

- [ ] 2.1 Before writing the component: run the Svelte MCP server's
      `list-sections` then `get-documentation` for the sections relevant
      to a combobox (per `svelte.md`).
- [ ] 2.2 Build a creatable combobox component implementing the ARIA APG
      pattern (role="combobox"/"listbox"/"option"), fetching
      `GET /consumers` for its options and synthesizing an "Add `<value>`"
      option when the typed text matches none; run
      `svelte-autofixer` until clean.
- [ ] 2.3 Replace the free-text `used_by` input in
      `(app)/+page.svelte`'s create and edit forms with the combobox;
      verify with a component test covering picking an existing consumer
      and adding a new one.
- [ ] 2.4 Add an axe-core assertion (WCAG 2.1 AA) to the journey test that
      already exercises the create/edit form; verify zero violations.

## 3. Verification

- [ ] 3.1 `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` pass.
- [ ] 3.2 `bun run test`, `bun run check`, `bun run lint`, and
      `bun run format:check` pass in `cmd/hush-hush/web`.
- [ ] 3.3 Manually verified against the running binary: creating a secret
      offers existing consumers and accepts a new one; the new consumer
      then appears as an option on the next secret's form.
