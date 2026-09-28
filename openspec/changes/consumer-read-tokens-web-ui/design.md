# Design

## Context

`ConsumerCombobox.svelte` is a hand-written ARIA combobox (tag picker) used
today by the secret create/edit dialogs to build a multi-valued `used_by`
list - `value: string[]`, each pick appends a chip, `Backspace` on an
empty query pops the last one. The consumer-token create form needs the
same autocomplete-and-suggest interaction (issue #440: "selected the same
way a consumer is picked elsewhere in the UI"), but a consumer token binds
to exactly one consumer at creation - never edited afterward, never a
list.

See `proposal.md` for why this change exists.

## Goals / Non-Goals

**Goals:**

- Reuse `ConsumerCombobox`'s existing interaction (autocomplete, known-name
  suggestions, free-text "Add" fallback, public-key-status hint) for the
  consumer-token form, rather than writing a second combobox.
- Keep every existing call site (`+page.svelte`'s create/edit secret
  dialogs) working unchanged.

**Non-Goals:**

- A general-purpose "max N selections" combobox. This only ever needs
  exactly one.

## Decisions

**Add an optional `max?: number` prop to `ConsumerCombobox`, defaulting to
unlimited (today's behavior), rather than building a separate
single-value picker component.**

- `addConsumer` replaces `value` with `[name]` instead of appending once
  `value.length >= max` - for the only real case (`max={1}`), picking a
  new consumer swaps the selection rather than adding a second chip.
- The text input (and its listbox) renders only while
  `!max || value.length < max`, so a single-select instance hides its own
  input once a consumer is chosen - matching how a single-select combobox
  reads (the existing "×" remove button on the chip is what re-opens the
  choice, by clearing the selection).
- Alternatives considered: a second `ConsumerSingleCombobox` component was
  rejected - it would duplicate the ARIA wiring, the known-consumers
  fetch, and the public-key-status hint for one prop's worth of
  difference, and any future fix to the combobox pattern itself would
  need to land in two places.

**Also adds a `showKeyStatus?: boolean` prop, defaulting to `true`
(today's behavior).** The existing "(no key)" hint next to a chip is
about age-recipient status for sealing a secret - meaningless for the
consumer-token form, which only needs a consumer _name_, and would read
as a misleading warning if left on. The token-create form passes
`showKeyStatus={false}`; every other call site is unaffected by the
default.

**The create-consumer-token form's picker binds `value` as a
single-element array (`[]` or `[name]`) and reads `value[0]` for the
actual `consumer` field sent to `createConsumerToken`**, rather than
`ConsumerCombobox` itself exposing a scalar prop - keeps the component's
public contract (`value: string[]`) identical for every caller,
`max` is purely a behavioral constraint on top of it.

## Risks / Trade-offs

- A consumer token's `consumer` field is free text server-side (the
  consumer doesn't have to already exist yet, matching how `used_by`
  already works) - the single-select combobox's "Add" fallback already
  covers this, no extra handling needed.
