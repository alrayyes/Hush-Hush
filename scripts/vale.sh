#!/usr/bin/env bash
# Runs Vale: the binary on PATH when there is one, otherwise the official
# jdkato/vale image, the same one CI's `style` job runs. A contributor needs
# either Vale or Docker, and never `go install`, which needs a Go toolchain and
# breaks when Vale moves its module path (rules/markdown.md, "Running Vale").
#
# The pre-commit hook and lint-prose.sh both go through this, so neither
# assumes Vale is installed. The tag and digest are pinned together, with the
# comment Renovate reads, and prose.yml's `style` job names the same image.
set -euo pipefail

VALE_IMAGE=jdkato/vale:v3.17.1@sha256:7dba3c9104ba366f172d119022c4ec53a005f7d14dc1b80e285421a3f0b71657 # renovate: datasource=docker depName=jdkato/vale

if command -v vale >/dev/null 2>&1; then
  exec vale "$@"
fi

if command -v docker >/dev/null 2>&1; then
  # --user keeps `vale sync`'s downloads in styles/ owned by the invoking user,
  # not root. The image's entrypoint is /bin/vale, so the arguments are Vale's.
  exec docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" -w /work "$VALE_IMAGE" "$@"
fi

echo "vale: neither vale nor docker is on PATH. Install one of them (see CONTRIBUTING.md)." >&2
exit 127
