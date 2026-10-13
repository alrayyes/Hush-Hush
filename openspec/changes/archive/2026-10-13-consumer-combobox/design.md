# Design

## Context

`used_by` is `[]string` on `store.Object`, filterable via `GET
/objects?used_by=`, with no dedicated listing of what consumer values
exist (see proposal.md). The create/edit form
(`cmd/hush-hush/web/src/routes/(app)/+page.svelte`) parses it from a raw
text field (`parseUsedBy`) today.

## Goals / Non-Goals

**Goals:**

- Offer existing consumer names when editing `used_by`, without losing
  the ability to enter a genuinely new one.

**Non-Goals:**

- A management page for consumers (renaming, merging near-duplicates,
  seeing which objects reference one) - that's `consumer-directory`, a
  separate change building on this one's `GET /consumers` endpoint.
- Removing duplicate or normalizing existing `used_by` values already
  stored - this only changes how new values are entered.

## Decisions

- **The UI pattern is a "creatable combobox"**: the [ARIA APG combobox
  pattern](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Roles/combobox_role)
  (`role="combobox"` + `role="listbox"`/`role="option"`) as the base,
  with one added listbox option - "Add `<value>`" - appended only when
  the typed text matches no existing consumer. Selecting it behaves like
  selecting any other option (it sets the field's value); it isn't a
  second action bolted onto an existing row, which the APG combobox
  pattern doesn't support (each option performs exactly one action) - the
  "add" option is itself a normal, single-action option, just one the
  listbox synthesizes from the current text.
- **No client library added for this.** A combobox with one synthesized
  extra option is a small enough component to hand-write against the ARIA
  pattern directly, and `svelte.md`'s own MCP-first workflow
  (`list-sections`/`get-documentation` for the APIs relevant to a
  combobox, then
  `svelte-autofixer` on the result) is what verifies the implementation
  rather than a bundled combobox package.
- **`GET /consumers` is unpaginated.** A consumer list is bounded by how
  many distinct `used_by` values exist across all objects in a single
  secrets store - not a scale that needs pagination to populate this
  dropdown. `consumer-directory`'s own paginated view is for browsing, a
  different use case from populating this input.

## Risks / Trade-offs

- [Numerous distinct consumers make the dropdown itself unwieldy] →
  mitigated by the combobox's own type-to-filter behaviour;
  revisit unpaginated `GET /consumers` if a real deployment's consumer
  count ever makes the unfiltered response itself too large, which isn't
  expected at this store's scale.
