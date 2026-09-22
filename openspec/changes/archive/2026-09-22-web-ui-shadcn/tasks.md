# Tasks

## 1. Scaffold (alrayyes/hush-hush#318)

- [x] 1.1 `npx shadcn-svelte@latest init` in `cmd/hush-hush/web`, then
      hand-edit the generated token block in `src/app.css` so shadcn's
      variable names alias the existing `--color-*`/`--radius` tokens
      (design.md's "Token mapping" decision); verify no new color values
      were introduced by diffing `app.css` for anything other than the
      new alias block. `init`'s own interactive design-system-preset
      picker (shadcn-svelte v1.7) has no non-interactive skip, so
      `components.json` was handwritten instead and `add` run directly
      against it - `init` itself was never actually run.
- [x] 1.2 `npx shadcn-svelte@latest add button input dialog alert-dialog select table card label`;
      verify `bun run build` succeeds and `bun run lint:tailwind` reports
      no new warnings against the generated `ui/` files. `textarea` and
      `checkbox` were added later too, once the secrets-overview and
      settings migrations (#321/#322) actually needed them.
- [x] 1.3 Add a dated note to `openspec/changes/web-ui/design.md`'s
      "Component approach" decision superseding it, pointing at this
      change and alrayyes/hush-hush#317.

## 2. Shared chrome (alrayyes/hush-hush#319)

- [x] 2.1 Migrate `+layout.svelte`, `Footer.svelte`, `ThemeToggle.svelte`,
      and the `(app)` nav onto the generated `Button` component; verify
      `theme.spec.ts` still passes unchanged and the journey test's nav
      assertions still pass. `Footer.svelte` itself stayed hand-rolled -
      plain inline text links, not buttons. Nav links needed a compact
      size override (`h-auto px-1 py-1`); `Button`'s default padding
      wrapped the topbar into three rows at 320px instead of two.
- [x] 2.2 Verify the axe-core scan on a page using the migrated chrome
      still reports zero violations.

## 3. Consumers table and endpoint (alrayyes/hush-hush#324, backend half)

- [x] 3.1 Add the `consumers` table to `schema.sql` (design.md); verify
      migrations apply cleanly to a fresh and an existing database.
- [x] 3.2 Change `ListConsumers`/`ListConsumersPage` to `UNION` `consumers`
      with `used_by`'s distinct names (TDD - failing test first covering
      a consumer present only in `consumers` with a zero secret count).
- [x] 3.3 Add `api/openapi.yaml`'s `POST /consumers` (name), linted with
      Redocly; implement the handler + store insert, rejecting a name
      already present in `consumers` or `used_by`; verify with a test per
      case.
- [x] 3.4 Update `RenameConsumer`/`DeleteConsumer` to also touch
      `consumers` when the name has no `used_by` rows; verify a consumer
      added via 3.3 with zero secrets can be renamed and deleted the same
      as any other.
- [x] 3.5 Add a Pact contract test for `POST /consumers`. Not applicable
      once actually looked at: `pacts/hush-hush-cli-hush-hush-server.json`
      is the CLI-server bearer-token contract (Pact's own "consumer" is
      the CLI, unrelated to this domain's "consumer" concept), and the
      CLI never calls `/consumers` at all. `internal/api/openapi_test.go`
      gained the equivalent request/response-shape coverage instead,
      matching every other endpoint's own contract test there.

## 4. Consumers page UI (alrayyes/hush-hush#324, frontend half)

- [x] 4.1 Add an "Add consumer" control and dialog to
      `src/routes/(app)/consumers/+page.svelte`, wired to 3.3's endpoint;
      migrate the existing rename/delete dialogs onto the generated
      `Dialog`/`AlertDialog`; verify against a running server, including
      the duplicate-name rejection surfacing as a visible error.
- [x] 4.2 Verify the axe-core scan on the migrated consumers page reports
      zero violations at a 320px viewport.

## 5. Audit-log filter-options endpoint (alrayyes/hush-hush#323, backend half)

- [x] 5.1 Add `GET /audit-log/filter-options` to `api/openapi.yaml`
      (design.md's "Audit log filter options" decision), linted with
      Redocly; implement the handler + store query (`SELECT DISTINCT`
      over `audit_log`); verify it returns object ids/actors/callers that
      actually appear in a seeded log, including a "none" actor entry for
      an unauthenticated read. `AuditLogFilter.Actor` also gained the
      `"none"` sentinel itself (matching `actor_type IS NULL`), since the
      existing filter had no way to select an unauthenticated read at
      all before this.
- [x] 5.2 Add a Pact contract test for the new endpoint. Not applicable,
      same reasoning as 3.5 - covered by `openapi_test.go` instead.

## 6. Audit-log page UI (alrayyes/hush-hush#323, frontend half)

- [x] 6.1 Migrate the audit-log page's table/pagination/chips onto the
      generated `Table`/`Button` components. The table itself stayed
      hand-rolled `.responsive-table` - same reasoning as #321/#322's own
      tables (shadcn's `Table` wraps in horizontal scroll with nowrap
      cells, a desktop pattern that regresses the mobile-first row-to-
      card collapse).
- [x] 6.2 Replace the object-id and actor/caller free-text filters with
      `Select` components populated from 5.1's endpoint; verify choosing a
      value refetches immediately (no "Apply" step, matching the existing
      chip behaviour) and that "none" is selectable for actor. bits-ui's
      `Select.Trigger` renders as a plain `<button>` with
      `aria-haspopup="listbox"`, not `role="combobox"` - that role
      belongs to a separate searchable-combobox variant this doesn't use.
- [x] 6.3 Verify the axe-core scan on the migrated page reports zero
      violations. This page had no interactive test coverage of its
      filters at all before this - added real Playwright coverage
      (opening each select, choosing a value including "none", checking
      the entries list and the scan) rather than just checking the visual
      migration.

## 7. Remaining page migrations

- [x] 7.1 Login page (alrayyes/hush-hush#320): migrate onto
      `Input`/`Button`/`Label`; verify the journey test's login flow and
      axe-core scan still pass. No `Input`/`Label` needed in the end -
      this page has no form fields, passkey auth doesn't take one; only
      `Button`.
- [x] 7.2 Secrets overview (alrayyes/hush-hush#321): migrate table and
      create/edit/delete dialogs; verify `consumers.spec.ts` and the
      journey test's CRUD flow still pass. The View dialog's own explicit
      "Close" button was dropped - shadcn's `DialogContent` already
      renders an accessible close control (visually an X, accessible
      name "Close") by default, and the two together were a real
      duplicate, caught by the journey test's own
      `getByRole('button', { name: 'Close' })` resolving to two elements.
- [x] 7.3 Settings (alrayyes/hush-hush#322): migrate passkeys and tokens
      sections; verify the journey test's settings flow and the last-
      credential-refusal case still pass.
- [x] 7.4 Static pages - changelog/disclaimer/privacy
      (alrayyes/hush-hush#325): migrate the content wrapper onto `Card`;
      verify `markdown.spec.ts` is unchanged and the axe-core scan on each
      page reports zero violations. Each page's own `<h1>` stayed outside
      the card as a real heading, not shadcn's `CardTitle` (a `<div>`,
      wrong as a page's only h1). Adding the axe-core scan here (none of
      these three pages had one before) found a real, pre-existing bug:
      Tailwind's own preflight resets `a` to `text-decoration: inherit`,
      silently cancelling the underline `app.css`'s base `a` rule already
      assumed it had from the browser default - every link in the app had
      been missing its underline since Tailwind was adopted (#311), fixed
      here since the new test needed it to pass.

## 8. Verification

- [x] 8.1 `bun run test`, `bun run check`, `bun run lint`,
      `bun run format:check`, `bun run lint:tailwind` pass in
      `cmd/hush-hush/web`.
- [x] 8.2 `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` pass.
- [x] 8.3 `bun run lint:api` passes against the updated OpenAPI spec.
- [x] 8.4 Full Playwright `e2e` suite (including every axe-core scan)
      passes against a real built binary.
