#!/usr/bin/env bash
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Runs the end-to-end journeys against the real binary (SPEC-016 SH8). It needs `make build`.
# Usage: apps/web/scripts/e2e-live.sh [binary] [port] [playwright args...]
#
# A throwaway data directory gets a master key (and its backup acknowledgement, as scripts/smoke.sh
# does), one account and one organization; the server runs as --role=standalone and is stopped, by its
# exact process id, when the journeys end.
set -euo pipefail

root="$(cd "$(dirname "$0")/../../.." && pwd)"
bin="${1:-$root/bin/orcon}"
port="${2:-18181}"
shift $(($# < 2 ? $# : 2))
base="http://127.0.0.1:${port}"
email="journey@example.com"
secret="journey-password-1234"
org="acme"

if [ ! -x "$bin" ]; then
  echo "e2e-live: $bin not found; run 'make build' first" >&2
  exit 1
fi

data="$(mktemp -d)"
export ORCON_DATA_DIR="$data"
# Sign-in checks the origin of the request against the public URL of the installation.
export ORCON_PUBLIC_URL="$base"
mkdir -p "$data/keys"
head -c 32 /dev/urandom | base64 >"$data/keys/master.key"
chmod 600 "$data/keys/master.key"

pid=""
stop_server() {
  if [ -n "$pid" ]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    pid=""
  fi
}
trap 'stop_server; rm -rf "$data"' EXIT

refusal="$("$bin" serve --role=standalone --http-address="127.0.0.1:${port}" 2>&1 || true)"
fingerprint="$(printf '%s\n' "$refusal" | sed -n 's/.*master key fingerprint \([0-9a-f]*\).*/\1/p' | head -n 1)"
if [ -z "$fingerprint" ]; then
  echo "e2e-live: the first start did not name the master key fingerprint; it said:" >&2
  printf '%s\n' "$refusal" >&2
  exit 1
fi
export ORCON_MASTER_KEY_BACKUP_ACK="$fingerprint"

wait_ready() {
  for _ in $(seq 1 60); do
    if curl -fsS "$base/healthz" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "e2e-live: the server did not become healthy" >&2
  return 1
}

start_server() {
  "$bin" serve --role=standalone --http-address="127.0.0.1:${port}" >"$data/server.log" 2>&1 &
  pid=$!
  wait_ready
}

# The first start creates the keys; accounts and organizations are made with the server stopped
# (SQLite holds the file), then the server starts again for the journeys.
start_server
stop_server
printf '%s\n' "$secret" | "$bin" accounts create --email "$email" --name "Journey User" >/dev/null
"$bin" orgs create --slug "$org" --name "Acme" --admin "$email" >/dev/null
start_server

if [ "${ORCON_E2E_SERVE_ONLY:-}" = "1" ]; then
  # For looking at the app by hand: the server stays up until this script is stopped.
  echo "e2e-live: serving at $base as $email / $secret, organization $org"
  wait "$pid"
  exit 0
fi

cd "$root/apps/web"
ORCON_JOURNEY_BASE_URL="$base" ORCON_JOURNEY_EMAIL="$email" ORCON_JOURNEY_PASSWORD="$secret" ORCON_JOURNEY_ORG="$org" \
  pnpm exec playwright test -c playwright.journeys.config.ts "$@"
