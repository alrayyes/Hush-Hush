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
`.github/workflows/` instead of a self-hosted forge's workflow directory).

## Commands

```sh
go build ./... && go vet ./... && go test ./...
golangci-lint run                  # golangci-lint fmt is the fixer
bun run format:check               # bun run lint:md, lint:api, lint:prose, lint:mechanics too
```

Full list and what each one does: [CONTRIBUTING.md](CONTRIBUTING.md).

## Gotchas

- **Branch protection on `main`** requires a pull request before merging
  (`enforce_admins: false`, so the repo owner can still bypass in a genuine
  emergency - that's not licence to make bypassing routine).
- **`internal/api` holds the handler, `cmd/hush-hush` starts it.** Per
  `go.md`'s "a server keeps everything in `internal/` and its commands in
  `cmd/`" — there's nothing here worth exporting, since the API this
  service offers is its endpoints, not its Go packages.
- **The scaffold's `GET /widgets/{id}` placeholder is gone.** `api/openapi.yaml`
  now describes the real secret-object endpoints (`objects`, `audit-log`),
  implemented in `internal/api`.
- **`LICENSE` is GPL-3.0**, decided when this repo was created from the
  scaffold (the scaffold itself ships unlicensed on purpose - that's a
  decision each stamped project makes for itself).
- **GitHub's "Allow GitHub Actions to create and approve pull requests"
  setting is on** (`can_approve_pull_request_reviews: true` via the Actions
  permissions API) - release-please can't open its release PR without it,
  and it's off by default on a new repo.
- **Dependabot raises the dependency pull requests here**, not Renovate -
  GitHub-primary repos use GitHub-native Dependabot
  (`.github/dependabot.yml`); Renovate is the answer on a Forgejo instance
  instead.
