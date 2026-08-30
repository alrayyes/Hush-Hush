# hush-hush

[![CI](https://github.com/alrayyes/hush-hush/actions/workflows/ci.yml/badge.svg)](https://github.com/alrayyes/hush-hush/actions)
[![release](https://img.shields.io/github/v/release/alrayyes/hush-hush?sort=semver)](https://github.com/alrayyes/hush-hush/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/alrayyes/hush-hush.svg)](https://pkg.go.dev/github.com/alrayyes/hush-hush)
[![licence](https://img.shields.io/badge/licence-GPL--3.0-blue)](LICENSE)

A lightweight, standalone secrets object store: a writer only ever needs the
service's public key to add or rotate a value, and a consumer only ever
needs an object id to fetch it. Public-key encryption
([age](https://github.com/FiloSottile/age)) end to end - the server stores
and serves sealed ciphertext and never computes or returns plaintext.

The full design - encryption model, read-path and write-path auth, storage
backend, and the reasoning behind each - is in
[`openspec/changes/secrets-object-store/design.md`](openspec/changes/secrets-object-store/design.md).

## Requirements

- **Go 1.26 or newer** to build from source, or **Docker** to run the
  published image instead - see [Docker](#docker) below.
- **[age](https://github.com/FiloSottile/age)**, to generate the keypairs a
  writer and a consumer each need. Not a dependency of hush-hush itself -
  the server and CLI only ever handle already-sealed ciphertext, never a
  private key or plaintext value.
- No external services. Storage is a local SQLite database.

## Installation

```sh
git clone https://github.com/alrayyes/hush-hush.git
cd hush-hush
go build ./cmd/hush-hush        # the server
go build ./cmd/hush-hush-cli    # the client every writer and consumer uses
```

## Usage

### Start the server

```sh
WRITER_TOKEN=change-me ./hush-hush
```

Listens on `:8080` by default. `WRITER_TOKEN` is the only required
setting - it's the single bearer token every create, update, and delete
call needs; v1 has no per-consumer scoping. See
[Configuration](#configuration) below for the rest of the server's
environment variables.

```sh
curl localhost:8080/healthz
```

### Use the CLI

A consumer needs an age keypair to receive a secret -
[`age-keygen`](https://github.com/FiloSottile/age) generates one:

```sh
age-keygen -o consumer.key
# Public key: age1...
```

Inject a secret, sealed to one or more recipients:

```sh
export HUSH_HUSH_SERVER=http://localhost:8080
export HUSH_HUSH_TOKEN=change-me

echo -n "hunter2" | hush-hush-cli inject mattermost_deploy_webhook \
  --recipients age1... --used-by homelab/vps-docker
```

Fetch and decrypt it - only whoever holds a matching private key can.
`--identity` takes the bare key, so pull it out of `age-keygen`'s comment
header first:

```sh
hush-hush-cli get mattermost_deploy_webhook --identity "$(tail -1 consumer.key)"
```

Rotate the value, then remove the object once nothing needs it any more:

```sh
echo -n "new-value" | hush-hush-cli update mattermost_deploy_webhook \
  --recipients age1...
hush-hush-cli delete mattermost_deploy_webhook
```

`inject` and `update` both read the new plaintext from stdin rather than a
flag or argument, so it never ends up in shell history or a process
listing.

[`api/openapi.yaml`](api/openapi.yaml) is the full contract, including the
audit-log query endpoint the CLI doesn't wrap - query it directly:

```sh
curl "localhost:8080/audit-log?object_id=mattermost_deploy_webhook"
```

### Configuration

The server reads environment variables only:

| Variable       | Default        | Meaning                                |
| -------------- | -------------- | -------------------------------------- |
| `ADDR`         | `:8080`        | Listen address.                        |
| `DB_PATH`      | `hush-hush.db` | SQLite database file.                  |
| `WRITER_TOKEN` | _(required)_   | Bearer token for create/update/delete. |

The CLI takes each setting as a flag or the matching environment variable -
a flag always wins:

| Flag           | Environment variable   | Meaning                                                                 |
| -------------- | ---------------------- | ----------------------------------------------------------------------- |
| `--server`     | `HUSH_HUSH_SERVER`     | Server base URL. Default `http://localhost:8080`.                       |
| `--token`      | `HUSH_HUSH_TOKEN`      | Bearer token, for `inject`/`update`/`delete`.                           |
| `--caller`     | `HUSH_HUSH_CALLER`     | Self-presented identity recorded in the audit log. Optional.            |
| `--recipients` | `HUSH_HUSH_RECIPIENTS` | Comma-separated age recipients, for `inject`/`update`.                  |
| `--identity`   | `HUSH_HUSH_IDENTITY`   | Comma-separated age private keys, for `get`.                            |
| `--used-by`    | -                      | Consumers of the secret (repeatable or comma-separated), `inject` only. |

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

## SDKs

- [hush-hush-go](https://github.com/alrayyes/hush-hush-go) - a typed Go
  client generated from [`api/openapi.yaml`](api/openapi.yaml), for
  programs that want the full API surface without hand-rolling HTTP calls.
- [hush-hush-python](https://github.com/alrayyes/hush-hush-python) - the
  same, generated for Python.
- [hush-hush-node](https://github.com/alrayyes/hush-hush-node) - the same,
  generated for Node.js/TypeScript.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the toolchain, the hooks, and how
a change gets reviewed and released.

## Licence

[GPL-3.0](LICENSE).
