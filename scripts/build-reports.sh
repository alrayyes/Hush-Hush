#!/usr/bin/env bash
# Assembles the reports the CI `pages` job publishes at
# apis.ryankes.eu/Hush-Hush/reports/ (rules/published-reports.md).
#
# Usage: build-reports.sh <dest-dir>
#
# <dest-dir> is the Pages artifact root. A project site is already served
# under /<repo>/, so the reports land in <dest-dir>/reports/ with no repo
# name of their own.
#
# Inputs are the directories the pages job downloads, each overridable
# through the environment:
#   GO_REPORTS          junit.xml, coverage.html, coverage.xml, coverage.out
#   WEB_REPORTS         reports/unit.xml and coverage/ (vitest, with
#                       cobertura-coverage.xml)
#   E2E_REPORTS         e2e.xml
#   LIGHTHOUSE_REPORTS  <page>.report.html and <page>.report.json (optional)
#
# GitHub Pages cannot list a directory, so every report directory gets an
# index.html that links its files: the catalogue reads those, and it
# recognises a Lighthouse run by its .report.html name. A report the repo
# did not produce is left out rather than published empty.
set -euo pipefail

dest="${1:?usage: build-reports.sh <dest-dir>}"
go_dir="${GO_REPORTS:-_go-reports}"
web_dir="${WEB_REPORTS:-_web-reports}"
e2e_dir="${E2E_REPORTS:-_e2e-reports}"
lh_dir="${LIGHTHOUSE_REPORTS:-}"
reports="$dest/reports"

mkdir -p "$reports/tests" "$reports/coverage/web"

page() { # page <title> <body>
  cat <<HTML
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>$1</title></head>
<body><main><h1>$1</h1>
$2
</main></body>
</html>
HTML
}

# Tests: one JUnit file per runner, named for it.
cp "$go_dir/junit.xml" "$reports/tests/junit.xml"
cp "$web_dir/reports/unit.xml" "$reports/tests/web-unit.xml"
cp "$e2e_dir/e2e.xml" "$reports/tests/e2e.xml"
page "Test results" '<ul>
<li><a href="junit.xml">junit.xml</a> (Go tests)</li>
<li><a href="web-unit.xml">web-unit.xml</a> (web unit tests)</li>
<li><a href="e2e.xml">e2e.xml</a> (Playwright journeys)</li>
</ul>
<p><a href="../">All reports</a></p>' >"$reports/tests/index.html"

# Coverage: the Go report keeps the directory's own names, the web one sits
# under web/.
cp "$go_dir/coverage.html" "$reports/coverage/index.html"
cp "$go_dir/coverage.xml" "$go_dir/coverage.out" "$reports/coverage/"
cp -r "$web_dir/coverage/." "$reports/coverage/web/"
cp "$reports/coverage/web/cobertura-coverage.xml" "$reports/coverage/web/coverage.xml"

index_links='<li><a href="tests/">Test results</a> (JUnit XML)</li>
<li><a href="coverage/">Go coverage</a> (<a href="coverage/coverage.xml">Cobertura XML</a>, <a href="coverage/coverage.out">Go profile</a>)</li>
<li><a href="coverage/web/">Web coverage</a> (<a href="coverage/web/coverage.xml">Cobertura XML</a>)</li>'

# Lighthouse: the authenticated audit is warn-only and can be missing, so
# link only the runs that were actually produced.
if [ -n "$lh_dir" ] && compgen -G "$lh_dir/*.report.html" >/dev/null; then
  mkdir -p "$reports/lighthouse"
  cp "$lh_dir"/*.report.html "$lh_dir"/*.report.json "$reports/lighthouse/"
  lh_links=""
  for html in "$reports"/lighthouse/*.report.html; do
    name="$(basename "$html" .report.html)"
    lh_links="$lh_links<li><a href=\"$name.report.html\">$name</a> (<a href=\"$name.report.json\">JSON</a>)</li>
"
  done
  page "Lighthouse reports" "<ul>
$lh_links</ul>
<p><a href=\"../\">All reports</a></p>" >"$reports/lighthouse/index.html"
  index_links="$index_links
<li><a href=\"lighthouse/\">Lighthouse reports</a></li>"
fi

commit="${GITHUB_SHA:-unknown}"
page "Hush-Hush reports" "<p>Commit ${commit:0:7}, built $(date -u +%Y-%m-%d).</p>
<ul>
$index_links
</ul>" >"$reports/index.html"
