#!/usr/bin/env bash
# Playwright's webServer for e2e - the app has no SSR and every page's own
# +layout.ts fetches GET /healthz at load, so a bare `vite preview` 500s on
# every route. This builds and runs the real Go binary instead, against a
# throwaway database, the same as the app runs in production.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../../.." && pwd)"

bin_dir="$(mktemp -d)"
go build -C "$repo_root" -o "$bin_dir/hush-hush-e2e" ./cmd/hush-hush

# E2E_PORT lets two runs on one machine use different servers. WebAuthn pins
# the origin, so the public URL follows the port.
port="${E2E_PORT:-4173}"
export PUBLIC_URL="http://localhost:$port"
export ADDR="127.0.0.1:$port"
export DB_PATH="$(mktemp -u).db"
# The top bar shows this label (#481/#512); e2e needs one to assert it.
export INSTANCE_LABEL="e2e / local"

exec "$bin_dir/hush-hush-e2e"
