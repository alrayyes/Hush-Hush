# 35. `PATCH /objects/{slug}` edits metadata, and a rename moves every variant

## Status

Accepted. Builds the rename [ADR 22](0022-object-id-slug-split.md) made
possible and said didn't exist yet, under the variant rules of
[ADR 33](0033-one-name-many-variants.md).

## Context

An object's description was fixed at creation, and nothing could change its
slug. `PUT /objects/{slug}` replaces the sealed value and so needs the
plaintext again, which is the wrong cost for fixing a typo in a note or
tidying a name. The web UI needs to edit all three of description, tags and
slug.

## Decision

- **`PATCH /objects/{slug}` takes any of `description`, `tags` and `slug`**,
  and at least one. A field left out stays; an empty description or tag list
  clears. The sealed value is never touched, so no re-seal and no recipient
  lookup.
- **A rename moves every variant of the name.** The slug is the name, and a
  name with several variants is one thing to the consumers reading it. Each
  variant keeps its id, value and `used_by`. Nothing else stored the slug
  (ADR 22), so a rename is one update.
- **A taken slug is a 409, a malformed one a 400.** The new name must have no
  objects at all, so a rename can't merge two names and can't put a consumer
  in two variants of one name (ADR 33's invariant holds without a check).
- **Description and tags apply to one variant**, the one `id` names, resolved
  the way `PUT` resolves it. A name with several variants and no `id` is a
  conflict. The response describes that variant, so a rename of a
  multi-variant name also takes `id`, which picks the variant to describe.
- **The request is all or nothing**, in one transaction.
- **The audit log gets one `update` entry under the new slug**, naming the
  variant. Entries written earlier keep the name they were written under.

Rejected: letting `PUT` take description and slug too. It would make the value
optional on a call whose whole point is a new value, and a client that meant
to rotate could rename by accident.

## Consequences

- A rename leaves earlier audit entries under the old name, so a name's
  history is split at the rename. The variant id on each entry still ties the
  two halves together.
- Consumers addressing the old slug get a 404 straight away. Nothing redirects.
- `used_by` stays editable only through `PUT`, which re-seals, because changing
  who can read a value has to change who it is sealed to.
