# Tasks

## 1. Migrate remaining raw form elements

- [x] 1.1 Replace `ConsumerCombobox.svelte`'s raw remove-consumer
      `<button>` with the shadcn `Button` component (ghost/icon variant)
      and verify the existing e2e coverage of adding/removing a consumer
      chip in `e2e/journey.spec.ts` still passes.
- [x] 1.2 Replace `ConsumerCombobox.svelte`'s raw text `<input>` with the
      shadcn `Input` component, preserving its combobox ARIA attributes
      (`role`, `aria-expanded`, `aria-controls`, `aria-activedescendant`,
      `aria-autocomplete`) and `bind:this`/keyboard handlers, and verify
      the combobox e2e journey (typing, arrow-key selection, add-new)
      still passes.
- [x] 1.3 Replace `audit-log/+page.svelte`'s two `datetime-local`
      `<input>` filters with the shadcn `Input` component and verify the
      audit-log date-range filter e2e coverage still passes.

## 2. Remove dead CSS

- [x] 2.1 Delete `.overlay`, `.dialog` (`@layer components`) and
      `button.danger`/`button.danger:hover` (`@layer base`) from
      `src/app.css`, and verify with `grep -rn '\.overlay\|\.dialog\|button\.danger' cmd/hush-hush/web/src` returning nothing. Also drops the
      now-orphaned `--color-overlay` design token (all four definitions:
      base `@theme` plus the dark/light `@media`/`[data-theme]`
      overrides), since it existed only to back `.overlay`, and the
      README's token list mention of it.
- [x] 2.2 Delete the bare `button`, `input, textarea` tag-selector rules
      (and their `:hover`/`:focus-visible`/`:disabled` states) from
      `src/app.css`'s `@layer base`, now that no raw `<button>`/`<input>`
      remain outside `src/lib/components/ui/`; verify with
      `grep -rn '<button\b\|<input\b' cmd/hush-hush/web/src --include='*.svelte' | grep -v components/ui/`
      returning nothing.

## 3. Update styling docs

- [x] 3.1 Update `cmd/hush-hush/web/README.md`'s "Styling" section and
      `cmd/hush-hush/web/AGENTS.md`'s "Styling convention" note to drop
      the `.overlay`/`.dialog` mention, listing only `.responsive-table`
      and `.changelog-content` as the remaining hand-written CSS
      (each already carries its own "no Tailwind utility covers this"
      rationale - leave those as-is).

## 4. Verification

- [ ] 4.1 `bun run lint:tailwind`, `bun run check`, `bun run test`, and
      (after a fresh `bun run build`) `bun run test:e2e` all pass in
      `cmd/hush-hush/web`.
