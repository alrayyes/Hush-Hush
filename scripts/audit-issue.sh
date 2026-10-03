#!/usr/bin/env bash
# Opens an issue for a failed scheduled audit, or comments on the one already
# open, so a run nobody is watching still reaches someone. One issue per tool
# (the title is the key), so a week of the same advisory is one thread, not
# seven tickets.
#
# Usage: audit-issue.sh <tool> <output-file>
# Needs GH_TOKEN and GITHUB_REPOSITORY (both set in Actions) and issues: write.
# shellcheck disable=SC2016 # the backticks are Markdown, not command substitution
set -euo pipefail

tool=$1
output=$2
title="Scheduled audit failed: ${tool}"
run_url="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID:-0}"

# The tool's own words are the part worth reading; cap them so a runaway log
# can't hit GitHub's comment size limit.
body=$(
  printf 'The daily `%s` run failed: %s\n\n```\n' "$tool" "$run_url"
  tail -n 60 "$output"
  printf '```\n'
)

existing=$(gh issue list --state open --search "\"${title}\" in:title" \
  --json number,title --jq "map(select(.title == \"${title}\")) | .[0].number // empty")

if [ -n "$existing" ]; then
  gh issue comment "$existing" --body "$body"
else
  gh issue create --title "$title" --label bug --body "$(
    printf '## Description\n\nAs a maintainer, I want to know when an advisory lands against a pinned dependency, so that I can bump it before it ships.\n\n%s\n\n## Acceptance criteria\n\n- Given the advisory above, when the dependency is bumped or the advisory is shown not to apply, then the next scheduled `%s` run passes.\n\n## Definition of done\n\n- The audit is green on `main` and this issue is closed referencing the fix.\n' "$body" "$tool"
  )"
fi
