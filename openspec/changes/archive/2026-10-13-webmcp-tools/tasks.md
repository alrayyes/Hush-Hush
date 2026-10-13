# Tasks

## 1. Design

- [x] 1.1 Record the maturity tradeoff and the tool-shape decisions in
      `docs/adr/0020-webmcp-tools.md`.

## 2. Web UI

- [ ] 2.1 Add a minimal ambient `Document.modelContext` type declaration
      (`cmd/hush-hush/web/src/lib/webmcp.d.ts`) covering only the
      `registerTool` shape used here.
- [ ] 2.2 Failing tests first in `cmd/hush-hush/web/src/lib/webmcp.spec.ts`:
      `registerWebMCPTools()` is a no-op when `document.modelContext` is
      undefined; registers exactly `list_objects` and
      `get_object_metadata` when it exists; each tool's `execute` calls
      the mocked `listObjects` and returns the expected shape; a thrown
      `ApiError` surfaces as a clear `Error`.
- [ ] 2.3 Implement `cmd/hush-hush/web/src/lib/webmcp.ts`:
      `registerWebMCPTools()`, both tools wrapping `listObjects` from
      `./api.ts`. Make the tests from 2.2 pass.
- [ ] 2.4 Call `registerWebMCPTools()` from
      `cmd/hush-hush/web/src/routes/(app)/+layout.svelte`'s `onMount` -
      via the Svelte MCP server's documentation/autofixer workflow, per
      this repo's own `AGENTS.md`.
- [ ] 2.5 `cmd/hush-hush/web/README.md`: new section documenting the two
      tools, that they're inert without browser support today, and that
      no secret value or destructive operation is ever exposed.

## 3. Verification

- [ ] 3.1 `bun run test`, `bun run check`, `bun run lint`,
      `bun run lint:tailwind`, `bun run build` pass in
      `cmd/hush-hush/web`.
- [ ] 3.2 Manually verified with `chrome://flags/#enable-webmcp-testing`
      enabled (or the
      [Model Context Tool Inspector](https://github.com/beaufortfrancois/model-context-tool-inspector)
      extension): logged into a running `hush-hush` instance,
      `list_objects`/`get_object_metadata` are discoverable and return
      the expected metadata.
