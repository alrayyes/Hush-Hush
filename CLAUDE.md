# hush-hush

Built from `~/.config/claude/CLAUDE.md` and `~/.config/claude/rules/*.md` —
read those for the "why" behind everything below. This file only says
what's specific to this repo.

## What this is

A standalone secrets object store: public-key (age) writes, consumers
decrypt locally, the server never sees plaintext. The full design -
encryption model, read-path and write-path auth, storage backend, and why
each was chosen over the alternatives - lives in
`openspec/changes/secrets-object-store/design.md`. The task breakdown for
implementing it is in that same change's `tasks.md`.

Bootstrapped from `alrayyes/scaffold-go-api`, then ported from that
scaffold's self-hosted-forge form to GitHub-primary tooling (release-please
instead of semantic-release, Dependabot instead of Renovate,
`.github/workflows/` instead of a self-hosted forge's workflow directory) -
see `FORGEJO.md` for the reasoning behind each swap, kept here in case this
repo ever needs to move the other way.

## Commands

```sh
go build ./... && go vet ./... && go test ./...
golangci-lint run                  # golangci-lint fmt is the fixer
bun run format:check               # bun run lint:md, lint:api, lint:prose, lint:mechanics too
```

Full list and what each one does: [CONTRIBUTING.md](CONTRIBUTING.md).

## Gotchas

- **Branch protection isn't turned on yet**, though it's free on this repo
  (public, GitHub). Until it is, the PR-only discipline is enforced by
  nobody but whoever's committing — never push straight to `main`.
- **`internal/api` holds the handler, `cmd/hush-hush` starts it.** Per
  `go.md`'s "a server keeps everything in `internal/` and its commands in
  `cmd/`" — there's nothing here worth exporting, since the API this
  service offers is its endpoints, not its Go packages.
- **The `GET /widgets/{id}` example resource is still the scaffold's
  placeholder**, not this service's real API. It gets replaced by the
  `secret-objects` capability's actual endpoints as
  `openspec/changes/secrets-object-store/tasks.md` is worked through - spec
  first, per `rules/api.md`, using that change's `specs/` as the contract.
- **`LICENSE` is GPL-3.0**, decided when this repo was created from the
  scaffold (the scaffold itself ships unlicensed on purpose - that's a
  decision each stamped project makes for itself).
- **`release-please-config.json` pins `"release-as": "0.0.1"`** so the first
  release lands there instead of wherever default semver bumping would put
  it. Remove that pin once the first release has actually shipped - it's a
  one-time override, not permanent config.
- **Dependabot raises the dependency pull requests here**, not Renovate -
  GitHub-primary repos use GitHub-native Dependabot
  (`.github/dependabot.yml`); Renovate is the answer on a Forgejo instance
  instead, per `FORGEJO.md`.
