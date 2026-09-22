# Tasks

## 1. Scaffold (alrayyes/hush-hush#318)

- [ ] 1.1 `npx shadcn-svelte@latest init` in `cmd/hush-hush/web`, then
      hand-edit the generated token block in `src/app.css` so shadcn's
      variable names alias the existing `--color-*`/`--radius` tokens
      (design.md's "Token mapping" decision); verify no new color values
      were introduced by diffing `app.css` for anything other than the
      new alias block.
- [ ] 1.2 `npx shadcn-svelte@latest add button input dialog alert-dialog select table card label`;
      verify `bun run build` succeeds and `bun run lint:tailwind` reports
      no new warnings against the generated `ui/` files.
- [ ] 1.3 Add a dated note to `openspec/changes/web-ui/design.md`'s
      "Component approach" decision superseding it, pointing at this
      change and alrayyes/hush-hush#317.

## 2. Shared chrome (alrayyes/hush-hush#319)

- [ ] 2.1 Migrate `+layout.svelte`, `Footer.svelte`, `ThemeToggle.svelte`,
      and the `(app)` nav onto the generated `Button` component; verify
      `theme.spec.ts` still passes unchanged and the journey test's nav
      assertions still pass.
- [ ] 2.2 Verify the axe-core scan on a page using the migrated chrome
      still reports zero violations.

## 3. Consumers table and endpoint (alrayyes/hush-hush#324, backend half)

- [ ] 3.1 Add the `consumers` table to `schema.sql` (design.md); verify
      migrations apply cleanly to a fresh and an existing database.
- [ ] 3.2 Change `ListConsumers`/`ListConsumersPage` to `UNION` `consumers`
      with `used_by`'s distinct names (TDD - failing test first covering
      a consumer present only in `consumers` with a zero secret count).
- [ ] 3.3 Add `api/openapi.yaml`'s `POST /consumers` (name), linted with
      Redocly; implement the handler + store insert, rejecting a name
      already present in `consumers` or `used_by`; verify with a test per
      case.
- [ ] 3.4 Update `RenameConsumer`/`DeleteConsumer` to also touch
      `consumers` when the name has no `used_by` rows; verify a consumer
      added via 3.3 with zero secrets can be renamed and deleted the same
      as any other.
- [ ] 3.5 Add a Pact contract test for `POST /consumers`.

## 4. Consumers page UI (alrayyes/hush-hush#324, frontend half)

- [ ] 4.1 Add an "Add consumer" control and dialog to
      `src/routes/(app)/consumers/+page.svelte`, wired to 3.3's endpoint;
      migrate the existing rename/delete dialogs onto the generated
      `Dialog`/`AlertDialog`; verify against a running server, including
      the duplicate-name rejection surfacing as a visible error.
- [ ] 4.2 Verify the axe-core scan on the migrated consumers page reports
      zero violations at a 320px viewport.

## 5. Audit-log filter-options endpoint (alrayyes/hush-hush#323, backend half)

- [ ] 5.1 Add `GET /audit-log/filter-options` to `api/openapi.yaml`
      (design.md's "Audit log filter options" decision), linted with
      Redocly; implement the handler + store query (`SELECT DISTINCT`
      over `audit_log`); verify it returns object ids/actors/callers that
      actually appear in a seeded log, including a "none" actor entry for
      an unauthenticated read.
- [ ] 5.2 Add a Pact contract test for the new endpoint.

## 6. Audit-log page UI (alrayyes/hush-hush#323, frontend half)

- [ ] 6.1 Migrate the audit-log page's table/pagination/chips onto the
      generated `Table`/`Button` components.
- [ ] 6.2 Replace the object-id and actor/caller free-text filters with
      `Select` components populated from 5.1's endpoint; verify choosing a
      value refetches immediately (no "Apply" step, matching the existing
      chip behaviour) and that "none" is selectable for actor.
- [ ] 6.3 Verify the axe-core scan on the migrated page reports zero
      violations.

## 7. Remaining page migrations

- [ ] 7.1 Login page (alrayyes/hush-hush#320): migrate onto
      `Input`/`Button`/`Label`; verify the journey test's login flow and
      axe-core scan still pass.
- [ ] 7.2 Secrets overview (alrayyes/hush-hush#321): migrate table and
      create/edit/delete dialogs; verify `consumers.spec.ts` and the
      journey test's CRUD flow still pass.
- [ ] 7.3 Settings (alrayyes/hush-hush#322): migrate passkeys and tokens
      sections; verify the journey test's settings flow and the last-
      credential-refusal case still pass.
- [ ] 7.4 Static pages - changelog/disclaimer/privacy
      (alrayyes/hush-hush#325): migrate the content wrapper onto `Card`;
      verify `markdown.spec.ts` is unchanged and the axe-core scan on each
      page reports zero violations.

## 8. Verification

- [ ] 8.1 `bun run test`, `bun run check`, `bun run lint`,
      `bun run format:check`, `bun run lint:tailwind` pass in
      `cmd/hush-hush/web`.
- [ ] 8.2 `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` pass.
- [ ] 8.3 `bun run lint:api` passes against the updated OpenAPI spec.
- [ ] 8.4 Full Playwright `e2e` suite (including every axe-core scan)
      passes against a real built binary.
