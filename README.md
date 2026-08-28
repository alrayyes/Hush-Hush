# hush-hush

[![CI](https://github.com/alrayyes/hush-hush/actions/workflows/ci.yml/badge.svg)](https://github.com/alrayyes/hush-hush/actions)
[![release](https://img.shields.io/github/v/release/alrayyes/hush-hush?sort=semver)](https://github.com/alrayyes/hush-hush/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/alrayyes/hush-hush.svg)](https://pkg.go.dev/github.com/alrayyes/hush-hush)
[![licence](https://img.shields.io/badge/licence-GPL--3.0-blue)](LICENSE)

A lightweight, standalone secrets object store: a writer only ever needs the
service's public key to add or rotate a value, and a consumer only ever
needs an object key to fetch it. Public-key encryption (age) end to end -
the server stores and serves sealed ciphertext and never computes or
returns plaintext.

**This repo is still being built.** The full design - encryption model,
read-path and write-path auth, storage backend, and the reasoning behind
each - is in
[`openspec/changes/secrets-object-store/design.md`](openspec/changes/secrets-object-store/design.md).
The API this README describes below is still the bootstrap scaffold's
placeholder example, not the real secret-object endpoints yet.

## Requirements

- **Go 1.25 or newer.**
- **[bun](https://bun.sh)**, for the tooling that isn't Go — commitlint,
  Prettier, markdownlint, [Redocly](https://redocly.com/docs/cli), and the
  [lefthook](https://lefthook.dev) that runs the git hooks. There's a
  `package.json`, but nothing here is JavaScript; it exists only so those
  tools resolve and stay pinned.
- **[golangci-lint](https://golangci-lint.run)**, pinned in
  [CONTRIBUTING.md](CONTRIBUTING.md#getting-set-up).
- No external services. Storage is a local SQLite database once the real
  implementation lands.

## Installation

```sh
git clone https://github.com/alrayyes/hush-hush.git
cd hush-hush
go build ./cmd/hush-hush
```

## Usage

```sh
./hush-hush
```

Listens on `:8080` by default; set `ADDR` to change it.

```sh
curl localhost:8080/healthz
curl localhost:8080/widgets/hammer
```

`api/openapi.yaml` is the contract both endpoints are held to — read it
first if you're replacing the example resource with a real one.

### Docker

Every tagged release publishes a multi-arch (`linux/amd64`, `linux/arm64`)
image to GitHub Container Registry:

```sh
docker pull ghcr.io/alrayyes/hush-hush:latest
docker run --rm -p 8080:8080 -e WRITER_TOKEN=change-me -e DB_PATH=:memory: \
  --cap-drop=ALL --security-opt=no-new-privileges --read-only \
  --memory=64m --cpus=0.5 \
  ghcr.io/alrayyes/hush-hush:latest
curl localhost:8080/healthz
```

Pin an exact version (`ghcr.io/alrayyes/hush-hush:0.7.0`) rather than
`latest` for anything other than trying it out. `--read-only` needs
`DB_PATH=:memory:` or a volume mounted at wherever `DB_PATH` points -
the default `hush-hush.db` has nowhere to write on a read-only file
system.

The [Dockerfile](Dockerfile) builds a static binary into the same
distroless, non-root image - build it yourself the same way goreleaser
does:

```sh
docker build -t hush-hush .
docker run --rm -p 8080:8080 -e WRITER_TOKEN=change-me -e DB_PATH=:memory: \
  --cap-drop=ALL --security-opt=no-new-privileges --read-only \
  --memory=64m --cpus=0.5 \
  hush-hush
curl localhost:8080/healthz
```

[`compose.yaml`](compose.yaml) wraps the same flags:

```sh
cp .env.example .env  # then set a real WRITER_TOKEN
docker compose up          # pulls the published image
docker compose up --build  # or builds the local Dockerfile instead
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the toolchain, the hooks, and how
a change gets reviewed and released.

## Licence

[GPL-3.0](LICENSE).
