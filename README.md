# Hush Hush

[![CI](https://github.com/alrayyes/hush-hush/actions/workflows/ci.yml/badge.svg)](https://github.com/alrayyes/hush-hush/actions)
[![Codecov](https://codecov.io/gh/alrayyes/hush-hush/graph/badge.svg)](https://codecov.io/gh/alrayyes/hush-hush)
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

See [`INSTALL.md`](INSTALL.md) for install options - AUR, `.deb`, `.rpm`,
and from source. The server (`hush-hush`) is container-only, see
[Docker](#docker) below.

## Usage

### Start the server

```sh
./hush-hush
```

Listens on `:8080` by default, storing objects in `./hush-hush.db`. See
[Configuration](#configuration) below for the rest of the server's
environment variables.

```sh
curl localhost:8080/healthz
```

Every create, update, and delete call needs a bearer token - issue one by
running the same binary again, directly against that database file:

```sh
./hush-hush token issue --description "homelab/vps-docker deploy"
# id:    a1b2c3d4e5f6a7b8
# token: 9f8e7d6c...
#
# The token is shown once - store it now, it can't be recovered later.
```

Any number of tokens can be valid at once, each with its own description
and expiry (`--ttl`, default 90 days) - `hush-hush token list` shows what's
issued, and `hush-hush token revoke <id>` invalidates one without touching
the others. There's no HTTP endpoint for any of this: minting the
credential that authenticates the write path can't itself need a token to
call over the network, so it's direct store access instead, same as the
server's own `DB_PATH`.

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
export HUSH_HUSH_TOKEN=9f8e7d6c...  # from `hush-hush token issue` above

echo -n "hunter2" | hush-hush-cli inject mattermost_deploy_webhook \
  --recipients age1... --used-by homelab/vps-docker \
  --description "prod deploy webhook"
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

Both binaries take settings in the same order, each layer overriding the
one before it: **flags > environment variables > config file > defaults**.
Neither one requires the file - environment variables alone are enough for
a CI job or a container - but each writes one for you the first time it
matters:

```sh
hush-hush init          # writes ~/.config/hush-hush/config.yaml
hush-hush-cli init      # writes ~/.config/hush-hush-cli/config.yaml
```

(`$XDG_CONFIG_HOME` instead of `~/.config` if it's set.) Run either command
with nothing configured yet - no config file, no relevant environment
variable - on an interactive terminal, and it offers to write that starter
file itself before continuing; `--yes` skips the prompt and writes it
unconditionally, for a script that wants the file without a person to
answer for it. `--force` on `init` itself overwrites an existing file,
which the prompt never does.

The server reads:

| Variable  | config key | Default        | Meaning               |
| --------- | ---------- | -------------- | --------------------- |
| `ADDR`    | `addr`     | `:8080`        | Listen address.       |
| `DB_PATH` | `db_path`  | `hush-hush.db` | SQLite database file. |

`hush-hush token issue`/`list`/`revoke` read the same `DB_PATH`, so they
have to be run against the file (or, in a container, inside the container)
the server they're managing tokens for is actually using.

The CLI takes each setting as a flag, environment variable, or config key -
a flag always wins:

| Flag              | Environment variable      | config key      | Meaning                                                                                  |
| ----------------- | ------------------------- | --------------- | ---------------------------------------------------------------------------------------- |
| `--server`        | `HUSH_HUSH_SERVER`        | `server`        | Server base URL. Default `http://localhost:8080`.                                        |
| `--token`         | `HUSH_HUSH_TOKEN`         | `token`         | Bearer token, for `inject`/`update`/`delete`.                                            |
| `--token-command` | `HUSH_HUSH_TOKEN_COMMAND` | `token_command` | Command whose trimmed stdout is the token instead - wins over `--token` if both are set. |
| `--caller`        | `HUSH_HUSH_CALLER`        | `caller`        | Self-presented identity recorded in the audit log. Optional.                             |
| `--recipients`    | `HUSH_HUSH_RECIPIENTS`    | `recipients`    | Comma-separated age recipients, for `inject`/`update`.                                   |
| `--identity`      | `HUSH_HUSH_IDENTITY`      | `identity`      | Comma-separated age private keys, for `get`.                                             |
| `--used-by`       | -                         | -               | Consumers of the secret (repeatable or comma-separated), `inject` only.                  |
| `--description`   | -                         | -               | Free-text label, fixed at creation, `inject` only.                                       |

### Docker

Every tagged release publishes a multi-arch (`linux/amd64`, `linux/arm64`)
image to GitHub Container Registry:

```sh
docker pull ghcr.io/alrayyes/hush-hush:latest
docker run --rm -d --name hush-hush -p 8080:8080 \
  -v hush-hush-data:/data -e DB_PATH=/data/hush-hush.db \
  --cap-drop=ALL --security-opt=no-new-privileges --read-only \
  --memory=64m --cpus=0.5 \
  ghcr.io/alrayyes/hush-hush:latest
curl localhost:8080/healthz
docker exec hush-hush /hush-hush token issue --description "trying it out"
```

Pin an exact version (`ghcr.io/alrayyes/hush-hush:0.7.0`) rather than
`latest` for anything other than trying it out. `--read-only` needs a
volume mounted at wherever `DB_PATH` points - the default `hush-hush.db`
has nowhere to write on a read-only file system, and so does a token
issued against `:memory:`, which `docker exec`'s own separate process
could never actually reach. The image's `/data` already comes owned by
its non-root user, so a freshly created named volume mounted there, the
same way the preceding command does, needs no separate chown step.

The [Dockerfile](Dockerfile) builds a static binary into the same
distroless, non-root image the published one runs - build it yourself:

```sh
docker build -t hush-hush .
docker run --rm -d --name hush-hush -p 8080:8080 \
  -v hush-hush-data:/data -e DB_PATH=/data/hush-hush.db \
  --cap-drop=ALL --security-opt=no-new-privileges --read-only \
  --memory=64m --cpus=0.5 \
  hush-hush
curl localhost:8080/healthz
docker exec hush-hush /hush-hush token issue --description "trying it out"
```

[`compose.yaml`](compose.yaml) wraps the same flags, including the volume:

```sh
docker compose up          # pulls the published image
docker compose up --build  # or builds the local Dockerfile instead
docker compose exec hush-hush /hush-hush token issue --description "trying it out"
```

## SDKs

- [hush-hush-go](https://github.com/alrayyes/hush-hush-go) - a typed Go
  client generated from [`api/openapi.yaml`](api/openapi.yaml), for
  programs that want the full API surface without hand-rolling HTTP calls.
- [hush-hush-python](https://github.com/alrayyes/hush-hush-python) - the
  same, generated for Python.
- [hush-hush-node](https://github.com/alrayyes/hush-hush-node) - the same,
  generated for Node.js/TypeScript.
- [hush-hush-php](https://github.com/alrayyes/hush-hush-php) - the same,
  generated for PHP.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the toolchain, the hooks, and how
a change gets reviewed and released.

## Licence

[GPL-3.0](LICENSE).
