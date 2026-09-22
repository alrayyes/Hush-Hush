# Design

## Context

See proposal.md - Why and What Changes. Three things shape this design:

- `cmd/hush-hush/web/src/app.css` already defines the app's real design
  tokens under Tailwind v4's `@theme` (`--color-accent`, `--color-danger`,
  `--radius`, etc.) and a `dark` custom variant keyed off
  `[data-theme="dark"]`. shadcn-svelte's generator writes its own
  `--primary`/`--destructive`/etc. variable names by default; this change
  maps shadcn's expected names onto the existing tokens rather than
  running two token systems side by side.
- `bits-ui` and `tailwindcss` are already dependencies; shadcn-svelte adds
  no new runtime package, only generated source under `src/lib/
components/ui/` plus its own dev-time CLI.
- `used_by (object_id NOT NULL REFERENCES objects, consumer)` has no way
  to record a consumer with no object referencing it - `object_id` is a
  `NOT NULL` foreign key. Adding a consumer directly (#324) needs
  somewhere to put a name that isn't yet used by anything.

## Goals / Non-Goals

**Goals:**

- One component system (shadcn-svelte, on the existing Bits UI + Tailwind
  v4 stack) across every page, with `@shadcn/lint`'s existing rules
  actually passing against real generated components.
- No regression in existing behaviour, tests, or the axe-core WCAG 2.1 AA
  scan on any migrated page.
- The two new pieces of user-facing behaviour this pass adds (audit-log
  select filters, direct consumer creation) go through this project's
  usual spec-first, TDD process, the same as any other backend change -
  not treated as free extras because they're bundled into a styling
  migration.

**Non-Goals:**

- No new pages, no navigation restructuring.
- No change to the WebAuthn, session, or bearer-token auth model.
- No general consumer identity model (ADR 0002's deferral stands) - a
  `consumers` table (below) is a name registry only, not an owner/ACL
  model.

## Decisions

**Token mapping: shadcn-svelte's `init` targets `src/app.css`, its
generated variable names layered onto the existing `@theme` tokens rather
than replacing them.** `components.json`'s `tailwind.css` points at
`src/app.css`; after `init` runs, the CLI-written `--primary`/
`--destructive`/etc. block is hand-edited to reference the app's own
`--color-accent`/`--color-danger`/etc. (`--primary: var(--color-accent)`),
so there is exactly one set of real color values and shadcn's expected
names are aliases onto it. Alternative considered: let `init` write its
own default palette and re-theme by eye. Rejected - that's how a second,
drifting token system starts.

**New `consumers` table, additive to the existing `used_by`-derived
listing, not a replacement for it.**

```sql
CREATE TABLE IF NOT EXISTS consumers (
    name TEXT PRIMARY KEY,
    created_at TEXT NOT NULL
);
```

`ListConsumers`/`ListConsumersPage` change from `SELECT DISTINCT consumer
FROM used_by` to a `UNION` of `consumers.name` and `used_by`'s distinct
names, with `secret_count` still computed by joining `used_by` (0 for a
name that only exists in `consumers`). `CreateObject`/`UpdateObject`'s
existing `INSERT INTO used_by` path is untouched - a consumer that
becomes used by a secret is not "moved" anywhere, it just now also has a
`used_by` row alongside its (or without needing its) `consumers` row.
Adding a consumer (`POST /consumers`) inserts into `consumers` only, and
rejects a name that already appears in `consumers` OR `used_by` (the
existing-name check spans both). Deleting a consumer (existing `DELETE
/consumers/{name}`) removes it from both `used_by` and `consumers`, so a
zero-secret consumer added this way can be deleted the same as any other.
Renaming likewise needs to touch `consumers` when the old name has no
`used_by` rows to update. Alternative considered: make `used_by.object_id`
nullable and insert a sentinel row. Rejected - a nullable half of a
composite primary key member is exactly the kind of schema smell that
outlives the feature that motivated it, and a dedicated table says what
it means.

**Audit log filter options: a new `GET /audit-log/filter-options`
endpoint, computed from `audit_log` itself, not from `objects`.** Returns
distinct `object_id`s, distinct `(actor_type, actor_id)` pairs (labelled
the same way `auditActorLabel` already renders them, including "none" for
an unauthenticated read), and distinct `caller`s that actually appear in
the log. Alternative considered: populate the object-id select from
`GET /objects`. Rejected - the audit log outlives object deletion by
design (`audit_log` has no FK to `objects`, on purpose), so a select
sourced from currently existing objects would silently drop the ability
to filter by a deleted object's id, which is exactly a case this feature
exists to cover. The endpoint is session-gated read access, same tier as
the existing `GET /audit-log` itself.

**Component-by-component migration order: scaffold first (#318), then
shared chrome (#319), then pages, independently mergeable.** Each page's
sub-issue (#320-#325) is its own PR once #318 lands, per CLAUDE.md's "one
feature per pull request" - the shadcn primitives are additive to what
exists, so migrating a given page doesn't block any other page's PR from
shipping in any order once the scaffold is in.

## Risks / Trade-offs

- Re-theming shadcn's generated components to the existing tokens is
  manual, ongoing work every time a new component is added later - not
  automated by the CLI. Accepted: still cheaper than maintaining the fully
  hand-rolled equivalent this replaces.
- `@shadcn/lint`'s rules are new to real project code (installed but
  unused until now); expect some churn in existing non-`ui/` markup
  (`shadcn/no-raw-colors`, `shadcn/no-arbitrary-values`) as pages migrate
  and the linter actually starts finding things.

## Migration Plan

No data migration for existing installs beyond the additive `consumers`
table (`CREATE TABLE IF NOT EXISTS`, applied on every `Open` like the rest
of `schema.sql` - a no-op against a database that already has it).
Frontend migration is page-by-page per the sub-issues; no page is broken
mid-migration since each PR replaces one page's markup in full within
that PR, not partially.

## Open Questions

None outstanding - the two decisions this change needed beyond "adopt
shadcn-svelte" (bare-consumer storage, audit-log filter sourcing) are
both settled above.
