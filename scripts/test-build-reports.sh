#!/usr/bin/env bash
# Tests scripts/build-reports.sh against fixture inputs shaped like the CI
# artifacts the `pages` job downloads. The invariant that matters: every
# link on every generated index page points at something that exists, with
# and without the optional Lighthouse reports.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# Fixtures, named like the artifacts in ci.yml's pages job.
mkdir -p "$tmp/in/go" "$tmp/in/web/reports" "$tmp/in/web/coverage" "$tmp/in/e2e" "$tmp/in/lh"
echo '<testsuites/>' >"$tmp/in/go/junit.xml"
echo '<html>go coverage</html>' >"$tmp/in/go/coverage.html"
echo '<?xml version="1.0"?><coverage line-rate="1"/>' >"$tmp/in/go/coverage.xml"
echo 'mode: atomic' >"$tmp/in/go/coverage.out"
echo '<testsuites/>' >"$tmp/in/web/reports/unit.xml"
echo '<html>web coverage</html>' >"$tmp/in/web/coverage/index.html"
echo '<?xml version="1.0"?><coverage line-rate="1"/>' >"$tmp/in/web/coverage/cobertura-coverage.xml"
echo '<testsuites/>' >"$tmp/in/e2e/e2e.xml"
for page in login secrets-overview; do
  echo "<html>$page</html>" >"$tmp/in/lh/$page.report.html"
  echo '{}' >"$tmp/in/lh/$page.report.json"
done

build() { # build <dest> [lighthouse dir]
  GO_REPORTS="$tmp/in/go" WEB_REPORTS="$tmp/in/web" E2E_REPORTS="$tmp/in/e2e" \
    LIGHTHOUSE_REPORTS="${2:-$tmp/in/none}" \
    "$here/build-reports.sh" "$1" >/dev/null
}

# Every relative href in an index must resolve to a file, or to a directory
# that has its own index.html.
check_links() {
  local index="$1" dir href target
  dir="$(dirname "$index")"
  while IFS= read -r href; do
    case "$href" in
      http*|\#*) continue ;;
    esac
    target="$dir/${href%%[?#]*}"
    if [ -d "$target" ]; then target="$target/index.html"; fi
    [ -f "$target" ] || fail "$index links $href, which does not exist"
  done < <(grep -o 'href="[^"]*"' "$index" | sed 's/^href="//; s/"$//')
}

# 1. With Lighthouse reports.
build "$tmp/with" "$tmp/in/lh"
r="$tmp/with/reports"
for f in tests/junit.xml tests/web-unit.xml tests/e2e.xml tests/index.html \
  coverage/coverage.xml coverage/coverage.out coverage/index.html \
  coverage/web/coverage.xml coverage/web/index.html \
  lighthouse/login.report.html lighthouse/login.report.json \
  lighthouse/secrets-overview.report.html lighthouse/index.html index.html; do
  [ -f "$r/$f" ] || fail "missing $f"
done
for index in "$r/index.html" "$r/tests/index.html" "$r/lighthouse/index.html"; do
  check_links "$index"
done
grep -q 'secrets-overview.report.html' "$r/lighthouse/index.html" || fail "lighthouse index misses the overview"
grep -q 'lighthouse/' "$r/index.html" || fail "reports index does not link lighthouse/"

# 2. Without Lighthouse reports: nothing to link, nothing published.
build "$tmp/without"
r="$tmp/without/reports"
[ ! -e "$r/lighthouse" ] || fail "published an empty lighthouse directory"
if grep -q 'lighthouse' "$r/index.html"; then fail "reports index links lighthouse with none published"; fi
check_links "$r/index.html"
check_links "$r/tests/index.html"

echo "ok: build-reports.sh"
