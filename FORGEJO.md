# Hosting on Forgejo instead of GitHub

This template defaults to GitHub-primary tooling: release-please,
Dependabot, `.github/workflows/`. If a project stamped from it ends up
hosted on a Forgejo instance instead, three pieces need swapping.
Nothing else does: `api/openapi.yaml`, the Go layout, `golangci.yml`,
`.goreleaser.yml`, the hooks in `lefthook.yml` — all of it is
forge-agnostic already.

Every example below is copied or adapted from a real, working
semantic-release setup running against Forgejo and a real, working
Forgejo Actions CI workflow — not reconstructed from what the tools'
docs say they should look like. Domain names and account names in the
examples are placeholders; swap in whatever your own instance actually
uses.

## 1. Release automation: release-please → semantic-release

`release-please` is GitHub-only (`rules/releases.md`, and `go.md`'s own
release section says the same: "There's no Forgejo port"). On Forgejo
this repo wants **semantic-release** in front of goreleaser instead. The
split doesn't change: one tool reads Conventional Commits and decides
the version and the tag, and the other builds and attaches the
binaries.
Only which tool does the first half changes — `.goreleaser.yml` stays
exactly as it is. Only `release-please-config.json`,
`.release-please-manifest.json` and `.github/workflows/release.yml` get
replaced.

### What to remove

```sh
rm release-please-config.json .release-please-manifest.json
rm .github/workflows/release.yml
```

### What to add: `.commitlintrc.json`-adjacent devDependencies

This repo already has a `package.json` purely for pinned JS-adjacent
tooling (commitlint, Prettier, markdownlint, Redocly) — nothing here is
JavaScript. semantic-release fits in the same place. Pin the same exact
versions verified working below, unless a newer set has since been
checked:

```json
{
  "devDependencies": {
    "@ribbon-studios/semantic-release-forgejo": "0.1.3",
    "@semantic-release/changelog": "7.0.0",
    "@semantic-release/commit-analyzer": "13.0.1",
    "@semantic-release/git": "11.0.1",
    "@semantic-release/release-notes-generator": "14.1.1",
    "conventional-changelog-conventionalcommits": "10.2.1",
    "semantic-release": "25.0.9"
  }
}
```

Add a `release` script: `"release": "semantic-release"`.

### `release.config.mjs`

JavaScript, not `.releaserc.json` — the Forgejo plugin's token can't be
written into JSON, only read from `process.env` at config-evaluation
time.

```js
// semantic-release replaces release-please, which is GitHub-only
// (rules/releases.md, go.md). goreleaser's job is unchanged: it still
// only attaches binaries to a release that already exists. Only the
// tool that decides the version and cuts the tag is different.
//
// tagFormat: this repo's release-please config used `include-v-in-tag:
// true`, so its existing tags are v-prefixed (v0.1.0). semantic-release
// already defaults to `v${version}`, so nothing needs overriding here.
// Do NOT copy some other repo's `tagFormat: "${version}"` blind — bare
// tags with no `v` show up wherever a project's tags predate
// semantic-release entirely (an older tool wrote them that way before
// the move to Forgejo). Check `git tag` on whatever repo you're
// actually porting before deciding; matching the existing tag scheme
// is the whole point of the setting, and getting it wrong publishes
// 1.0.0 over a repo that has already released.
const forgejoUrl =
  process.env.FORGEJO_SERVER_URL ?? "https://forgejo.example.com";

export default {
  branches: ["main"],
  plugins: [
    ["@semantic-release/commit-analyzer", { preset: "conventionalcommits" }],
    [
      "@semantic-release/release-notes-generator",
      { preset: "conventionalcommits" },
    ],
    ["@semantic-release/changelog", { changelogTitle: "# Changelog" }],
    // Creates the Forgejo release. forgejoToken is passed explicitly rather
    // than left to the environment: the runner injects its own FORGEJO_TOKEN
    // into every job automatically, with repository write, and the plugin
    // reads env.FORGEJO_TOKEN when given no token of its own — so leaving it
    // to the environment means the release is authenticated by whichever of
    // the two the runner happens to leave in place. Passing forgejoToken
    // settles it: it wins over the environment in the plugin's own
    // resolution, and RELEASE_TOKEN is a name Forgejo neither reserves nor
    // sets (see the CI gotcha on reserved secret prefixes below).
    [
      "@ribbon-studios/semantic-release-forgejo",
      { forgejoUrl, forgejoToken: process.env.RELEASE_TOKEN },
    ],
    // Last of the prepare/publish plugins on purpose: it commits what the
    // changelog plugin wrote, and semantic-release tags the commit it made.
    // No package.json bump here, unlike a JS project's config — a Go binary
    // has no manifest to bump (go.md); goreleaser reads the version off the
    // tag instead, via -X main.version={{.Tag}} in .goreleaser.yml.
    [
      "@semantic-release/git",
      {
        assets: ["CHANGELOG.md"],
        message: "chore(release): ${nextRelease.version} [skip ci]",
      },
    ],
  ],
};
```

### `.forgejo/workflows/release.yml`

Adapted from a real, tested semantic-release workflow running against
Forgejo, with a goreleaser step appended for this repo's Go binary.
semantic-release itself creates and pushes the tag as part of its own
run — no separate `git tag` step — so goreleaser only needs to run
afterwards, gated on a tag now sitting on `HEAD`.

```yaml
name: release

on:
  push:
    branches: [main]

jobs:
  release:
    runs-on: docker
    container:
      # bun installs the dependencies, because that's what this repo installs
      # with — but semantic-release has to run under a real `node`. bun 1.3
      # reports itself as node v24.3.0, and semantic-release checks the
      # version it's running under at startup; that's not a check to find a
      # compatibility gap on, on the one job that pushes to main
      # (javascript.md). The apk install below puts a real node on PATH.
      image: oven/bun:1.3.14-alpine@sha256:5acc90a93e91ff07bf72aa90a7c9f0fa189765aec90b47bdbf2152d2196383c0
      options: --entrypoint ""
    steps:
      - run: apk add --no-cache ca-certificates git nodejs

      # fetch-depth: 0 is not optional — semantic-release walks back to the
      # last tag to work out what to release, and a shallow clone has none.
      # persist-credentials: false so there's exactly one credential in
      # play: the run's own automatic token doesn't get left in
      # .git/config alongside GIT_CREDENTIALS below.
      - uses: https://code.forgejo.org/actions/checkout@v4
        with:
          fetch-depth: 0
          persist-credentials: false

      - run: bun install --frozen-lockfile

      # `bun run release` rather than the binary, so this job runs the same
      # command a workstation dry-run would. bun runs it as a shell command;
      # the shebang in node_modules/.bin/semantic-release is
      # `#!/usr/bin/env node`, so node ends up executing it, not bun.
      - name: Release
        id: release
        env:
          # How semantic-release itself pushes the tag and changelog commit.
          # It has no Forgejo-specific auth — GH_TOKEN and GL_TOKEN are the
          # names it knows — so this is the generic form,
          # "<username>:<password>", and Forgejo takes an access token as
          # the password half.
          GIT_CREDENTIALS: "${{ secrets.RELEASE_USER }}:${{ secrets.RELEASE_TOKEN }}"
          RELEASE_TOKEN: ${{ secrets.RELEASE_TOKEN }}
        run: |
          bun run release
          echo "tag=$(git describe --tags --exact-match HEAD 2>/dev/null || true)" >> "$GITHUB_OUTPUT"

      - name: Set up Go
        if: steps.release.outputs.tag != ''
        uses: https://code.forgejo.org/actions/setup-go@v5
        with:
          go-version-file: go.mod

      # goreleaser's role is unchanged from the GitHub version of this
      # workflow: attach binaries to a release that already exists. What's
      # new on Forgejo is telling it where that release lives — goreleaser's
      # GitHub-shaped defaults don't apply here, so it needs an explicit
      # `gitea:` block naming the API host (Forgejo speaks the Gitea release
      # API) and a GITEA_TOKEN rather than GITHUB_TOKEN. goreleaser-action
      # itself isn't mirrored to code.forgejo.org — `git ls-remote` against
      # it 404s — so this installs the binary directly instead.
      - name: goreleaser
        if: steps.release.outputs.tag != ''
        env:
          GITEA_TOKEN: ${{ secrets.RELEASE_TOKEN }}
        run: |
          go install github.com/goreleaser/goreleaser/v2@v2.17.1
          goreleaser release --clean
```

Add `gitea:` to `.goreleaser.yml` so it knows which API to call. The
Forgejo runner also exports its own GITHUB_TOKEN for GitHub-Actions
compatibility, alongside the GITEA_TOKEN above — goreleaser refuses to
guess between them ("multiple tokens found, but only one is allowed")
without `force_token`:

```yaml
gitea_urls:
  api: https://forgejo.example.com/api/v1
  download: https://forgejo.example.com
force_token: gitea
```

Secrets to set under Settings → Actions → Secrets: `RELEASE_USER` (the
username) and `RELEASE_TOKEN` (a Forgejo token for that account with
read/write on the repository). Neither can be named `FORGEJO_TOKEN` —
see the CI gotchas below.

## 2. Dependency updates: Dependabot → Renovate

Dependabot is GitHub-native and doesn't run against a Forgejo remote.
Renovate is the replacement, and which config it needs depends on how
the target Forgejo instance is set up:

**If the instance already runs a shared Renovate bot with
`autodiscover: true`, no per-repo `renovate.json` is needed at all.**
The bot discovers every repository its token can push to and applies
its own shared config to each one automatically; giving the bot's
account push access to the repo is the entire setup. A per-repo
`renovate.json` only matters if you want to extend or override that
shared config for this one repository specifically.

A shared config that does this (adapted from a real, working one —
correcting one thing worth flagging):

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "platform": "gitea",
  "autodiscover": true,
  "extends": [
    "config:best-practices",
    ":enableVulnerabilityAlertsWithLabel('security')"
  ],
  "packageRules": [
    {
      "matchUpdateTypes": [
        "major",
        "minor",
        "patch",
        "pin",
        "pinDigest",
        "digest",
        "lockFileMaintenance",
        "rollback",
        "bump",
        "replacement"
      ],
      "automerge": true
    }
  ]
}
```

The flag: Renovate has no `platform: "forgejo"`. Forgejo is a Gitea
fork and speaks the same API shape Renovate already understands, so
the value is `"gitea"`. That's confirmed by the README of one real
Renovate-bot repo this doc draws from, which says outright "Renovate
needs `platform: gitea`" — while its own checked-in config file still
read `"platform": "gitlab"` at the time of writing, a leftover from
before that instance's move off GitLab that hadn't been updated yet.
Don't copy a stale `"gitlab"` value found lying around from a
pre-migration config; use `"gitea"`.

**If the target instance has no shared runner** — or you want rules
this repo alone should follow — a per-repo `renovate.json` at the repo
root does the same thing `config:best-practices` gives everyone else,
scoped to just this one:

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["config:best-practices"]
}
```

`gomod` and the `bun`-managed `package.json` are both auto-detected —
nothing ecosystem-specific needs declaring. Per `rules/releases.md`,
give `gomod` and `bun` bumps a `fix:` commit prefix rather than
`chore:` — a dependency bump changes what ships and should cut a
release the same as any other fix. `github-actions`-equivalent bumps
(the `uses:` lines in `.forgejo/workflows/`) stay `ci:`, since they
change the pipeline, not the artefact.

## 3. CI workflows: `.github/workflows/` → `.forgejo/workflows/`

Forgejo Actions runs the same YAML shape GitHub Actions does — `on:`,
`jobs:`, `steps:`, `uses:`, `${{ }}` expressions all carry over
unchanged. Move the files from `.github/workflows/*.yml` to
`.forgejo/workflows/*.yml` and most of the content survives as-is. What
doesn't:

- **`if:` can read `secrets` at job level.** Forgejo Actions allows
  `if: secrets.CLOUDFLARE_API_TOKEN != ''` as a job-level condition.
  GitHub Actions explicitly forbids this — accessing `secrets` in a
  job-level `if` is documented as unsupported there. This is a genuine
  syntax difference between the two engines, not a portability nicety
  to route around with a step-level check instead. A real working
  `ci.yml` uses exactly this to keep deploy jobs dormant until a
  deploy credential exists on the repo:

  ```yaml
  deploy:
    if: github.event_name == 'push' && secrets.CLOUDFLARE_API_TOKEN != ''
  ```

- **Actions must be reachable from the runner.** `uses: actions/checkout@v4`
  resolves against `github.com`, and a Forgejo runner may have no route
  there. Forgejo's own official Actions catalogue at `code.forgejo.org`
  mirrors the common ones and is reachable by default on a standard
  Forgejo Actions setup:

  ```yaml
  - uses: https://code.forgejo.org/actions/checkout@v4
  - uses: https://code.forgejo.org/forgejo/upload-artifact@v4
  - uses: https://code.forgejo.org/forgejo/download-artifact@v4
  ```

  An action with no Forgejo mirror either needs its own copy hosted
  on the same forge, or has to be replaced with the raw shell commands
  it wraps.

- **Secret names can't start with `FORGEJO_`, `GITHUB_`, or `GITEA_`.**
  Forgejo reserves all three prefixes, the way GitHub reserves
  `GITHUB_`. The trap is `secrets.FORGEJO_TOKEN` still _resolves_ — the
  run's own automatic token is published under that name — so a secret
  you tried and failed to name `FORGEJO_TOKEN` silently reads the
  wrong credential instead of erroring. Name release/deploy tokens
  `RELEASE_TOKEN`, `DEPLOY_TOKEN`, or similar instead.

- **Bare container images need their prerequisites installed before
  `actions/checkout` runs.** `actions/checkout` is a JavaScript action
  and dies with `exec: "node": executable file not found` in a
  container with no `node` on it. And installing `git` without
  `ca-certificates` alongside it breaks the same step a different
  way — once `git` is present, checkout switches from a Node-fetched
  tarball to a real `git clone` over HTTPS, and `git` reads the system
  CA store, which a minimal image hasn't got:

  ```yaml
  steps:
    - run: apk add --no-cache ca-certificates git nodejs
    - uses: https://code.forgejo.org/actions/checkout@v4
  ```

- **Images are pinned by digest**, same rule as anywhere else
  (`dependencies.md`), and it applies as much here as to a GitHub
  Actions runner: `image: oven/bun:1.3.14-alpine@sha256:5acc90a9…`, not
  a bare tag that can move under you.

- **`services:` works, but on at least one real runner, service
  containers can't be handed a config file.** A Forgejo runner's
  `runner-config.yaml` can set `valid_volumes: []`, which means no
  volume can be mounted into a job or service container on that
  runner — a service that normally reads its setup from a mounted file
  has to be configured entirely through `env:` instead, or split into
  one container per configuration. This is a property of _that
  runner's own config_, not a Forgejo Actions limitation in general —
  check `valid_volumes` on whatever runner the target instance
  actually uses before assuming the same constraint applies.

## Summary

| GitHub-primary                          | Forgejo equivalent                                |
| --------------------------------------- | ------------------------------------------------- |
| `release-please-config.json` + manifest | `release.config.mjs` (semantic-release)           |
| `.github/workflows/release.yml`         | `.forgejo/workflows/release.yml`                  |
| `.github/dependabot.yml`                | shared Renovate bot (or per-repo `renovate.json`) |
| `.github/workflows/*.yml`               | `.forgejo/workflows/*.yml`                        |
| `.goreleaser.yml`                       | unchanged — still just attaches binaries          |
