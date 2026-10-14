#!/usr/bin/env bash
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Smoke test of the built binary. Requires `make build` (it checks the real
# web build for id="root"). Usage: scripts/smoke.sh [binary] [port]
set -euo pipefail

bin="${1:-bin/orcon}"
port="${2:-18080}"
base="http://127.0.0.1:${port}"

if [ ! -x "$bin" ]; then
  echo "smoke: $bin not found; run 'make build' first" >&2
  exit 1
fi

data="$(mktemp -d)"
export ORCON_DATA_DIR="$data"
"$bin" serve --role=standalone --http-address="127.0.0.1:${port}" >/dev/null 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; rm -rf "$data"' EXIT

ready=0
for _ in $(seq 1 30); do
  if curl -fsS "$base/healthz" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [ "$ready" -ne 1 ]; then
  echo "smoke: server did not become healthy" >&2
  exit 1
fi

curl -fsS "$base/healthz" | grep -q '^ok$'
curl -fsS "$base/" | grep -q 'id="root"'

code=0
"$bin" serve --role=invalid >/dev/null 2>&1 || code=$?
if [ "$code" -ne 5 ]; then
  echo "smoke: invalid role exit code = $code, want 5" >&2
  exit 1
fi

echo "smoke: ok"
