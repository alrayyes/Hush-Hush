# Tasks

## 1. Consumers page: Tokens column

- [ ] 1.1 Add `tokensHref(consumer: string): string` to `src/lib/consumers.ts`
      alongside `secretsOverviewHref`, and verify a unit test asserts it
      returns `/settings?consumer=<encoded-name>`.
- [ ] 1.2 In `consumers/+page.ts`, fetch `listConsumerTokens()` alongside the
      existing `listConsumersPage()` call and reduce it into a
      `Map<string, number>` (`tokenCounts`) returned from `load`; verify
      `bun run check` passes with the new return type wired through.
- [ ] 1.3 In `consumers/+page.svelte`, add a "Tokens" column rendering the
      count from `data.tokenCounts` as a link via `tokensHref` when > 0, and
      as plain `0` text otherwise; verify manually against a consumer with
      tokens and one without.

## 2. Settings: consumer-scoped filter

- [ ] 2.1 In `settings/+page.svelte`, derive a filtered Consumer tokens list
      from `page.url.searchParams.get('consumer')`, mirroring the
      audit-log page's existing filter-chip pattern (removable chip, `goto`
      with the parameter stripped and `replaceState: true` to clear); verify
      a unit or manual check that the table only shows the filtered
      consumer's rows when the parameter is present.
- [ ] 2.2 Give the Consumer tokens section heading the id `consumer-tokens`
      and add a `$effect` that calls `scrollIntoView()` on mount when the
      `consumer` parameter is present; verify manually that following a
      `/settings?consumer=<name>` link lands scrolled to that section.

## 3. End-to-end coverage

- [ ] 3.1 Extend `e2e/journey.spec.ts` with a test that creates a consumer
      token, visits `/consumers`, follows the Tokens count link for that
      consumer, and asserts the Settings page shows only that consumer's
      token with the filter chip visible; verify `bun run test:e2e` passes
      (after `bun run build`).
- [ ] 3.2 Add an axe-core scan on both the Consumers page and the filtered
      Settings page state in the same test, per `rules/a11y.md`; verify no
      violations are reported.

## 4. Docs and lint

- [ ] 4.1 Run `bun run lint:tailwind`, `bun run check`, `bun run test`,
      `./scripts/lint-mechanics.sh`, `./scripts/lint-prose.sh` and fix any
      findings; verify all exit 0.
- [ ] 4.2 Update `cmd/hush-hush/web/README.md` if the Settings page's
      query-parameter behaviour needs documenting there; verify the file
      still reads true against the shipped behaviour.
