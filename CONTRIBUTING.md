# Contributing

This file is for whoever changes this codebase. The [README](README.md) is
for whoever runs it.

## Getting set up

- **Go 1.26 or newer.**
- **[bun](https://bun.sh)** for the tooling that isn't Go — commitlint,
  Prettier, markdownlint, [Redocly](https://redocly.com/docs/cli), and the
  [lefthook](https://lefthook.dev) that runs the git hooks. There's a
  `package.json`, but nothing here is JavaScript; it exists only so those
  tools resolve and stay pinned.
- **[golangci-lint](https://golangci-lint.run) v2.12.2**, which the
  pre-commit hook runs from your `PATH` while CI runs it pinned. Install
  that version rather than whichever is current: when the two disagree, the
  hook passes and the pipeline fails, and the reason isn't obvious from the
  failure.
- **[Vale](https://vale.sh)** on your `PATH`, for the style tier of the
  prose lint:

  ```sh
  go install github.com/errata-ai/vale/v3/cmd/vale@latest
  ```

  `ltex-cli-plus` needs nothing installed: the hook fetches and caches it
  on first use.

One command installs the linters and the git hooks:

```sh
bun install
```

An uninstalled hook silently does nothing, which is worse than not having
one, so the `prepare` script runs `lefthook install` for you. You find out
at the pipeline otherwise, not at the commit.

## Everyday commands

Every one of these is what a hook or CI runs — see `lefthook.yml` and
`.github/workflows/*.yml` for exactly which.

```sh
go build ./...
go vet ./...
go test ./...
go test -coverprofile=coverage.out -coverpkg=./... ./... && go tool cover -func=coverage.out
golangci-lint run
golangci-lint fmt          # the fixer; `run` stays the check

bun run format:check       # prettier --check, add --write to fix
bun run lint:md
bun run lint:api           # redocly lint, bare — no path argument
bun run lint:prose         # vale
bun run lint:mechanics     # ltex-cli-plus
```

## How it fits together

`internal/api` holds the handler, and `cmd/hush-hush/main.go` is the
composition root that wires it up and starts the server — per `go.md`'s "a
server keeps everything in `internal/` and its commands in `cmd/`", since
there's nothing here worth exporting. No finer `internal/domain`/
`internal/adapter` split on top of that: that shape earns its keep the day a
second resource needs it, not day one.

## The contract

`api/openapi.yaml` describes the API and is handwritten, not generated from
the handlers — see the header comment for why. `redocly lint` checks the
document is valid OpenAPI; `internal/api/openapi_test.go` checks the
handlers still match it — a real HTTP round trip through the actual mux,
per documented operation, validated against the spec's own schema in both
directions. Part of the ordinary `go test ./...` run, no separate command.

## Contract tests

`internal/client` (the HTTP client every consumer, including the future
CLI, speaks through) and `internal/api` carry a
[Pact](https://pact.io) contract behind the `pact` build tag - a local pact
file, no broker, per
[`design.md`](openspec/changes/secrets-object-store/design.md)'s decision.
Pact's Go binding statically links a native library into the test binary at
link time, so it isn't part of the ordinary `go test ./...` run: install it
once, then run the two suites explicitly, consumer before provider - the
provider test reads the pact file the consumer test writes, and running
them together as one `go test ./...` starts them as independent binaries
with no ordering guarantee between them.

```sh
go install github.com/pact-foundation/pact-go/v2@v2.7.1
sudo "$(go env GOPATH)"/bin/pact-go install   # writes to /usr/local/lib
export PACT_DO_NOT_TRACK=true                 # opts out of Pact's own telemetry

go test -tags=pact ./internal/client/... -v   # writes pacts/*.json
go test -tags=pact ./internal/api/... -run TestPactProviderVerification -v
```

`pacts/*.json` is committed - the provider test reads it from disk, not
from whatever the consumer test most recently produced in the same run.
Regenerate and commit it in the same pull request as any change to
`internal/client`'s request or response handling.

## Container integration test

`integration/` boots the actual built Docker image with
[testcontainers-go](https://golang.testcontainers.org) and proves it serves
real requests - the layer none of `internal/api`'s or `internal/store`'s
tests reach, since they run the server's Go code directly rather than the
packaged distroless artifact someone actually pulls and runs. Behind the
`integration` build tag (needs a Docker daemon, not part of the ordinary
`go test ./...` run):

```sh
go test -tags=integration ./integration/... -v
```

## Docker images

Two Dockerfiles, deliberately not one:

- **`Dockerfile`** compiles from source - what `docker build .`, the
  preceding container integration test, and `docker compose up --build`
  all use. hadolint lints it same as any other.
- **`Dockerfile.release`** only `COPY`s an already-cross-compiled binary -
  goreleaser's own `dockers:` block uses it exclusively, never built by
  hand. It exists because a Dockerfile that ran `go build` for a
  multi-arch image pays for it on every non-native architecture: that
  `RUN` step runs under QEMU emulation, 5-20x slower than the native
  cross-compile goreleaser's `builds:` step already did. Real tradeoff,
  not a free win: the released image no longer reproduces byte-for-byte
  from a bare `docker build .` the way building from source in one
  Dockerfile would - worth it for the speed on a repo shipping multi-arch
  images on every release. hadolint lints this one too.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org/):
`type(scope): description`, types `feat`/`fix`/`docs`/`style`/`refactor`/
`perf`/`test`/`build`/`ci`/`chore`/`revert`. Subject under 50 characters,
lowercase, no trailing full stop. commitlint enforces the shape at
commit-msg and again in CI; the length and case rules are tighter than
what it checks, so hold to them anyway.

## Branching, review, and release

Every change goes through a pull request — nothing is pushed straight to
`main`. GitHub's branch protection is free on a public repo like this one;
turn it on under Settings → Branches rather than relying on discipline alone.

The pull request **title** has to be a valid Conventional Commit too —
`pr-title.yml` checks it. commitlint only ever reads commit objects, and a
squash merge defaults its commit message to the pull request title, so this
is the only check standing between a badly titled pull request and a bad
message on `main`.

Once a pull request's checks are green, squash-merge it and delete the
branch. [release-please](https://github.com/googleapis/release-please)
reads the Conventional Commits on `main` and keeps a release pull request
open carrying the next version and changelog entry — merging that pull
request tags the release. [goreleaser](https://goreleaser.com) then builds
the binaries onto the release release-please just cut. Nobody picks a
version by hand.
