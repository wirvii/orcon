#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make cover: every business area package (a package under internal/ other than
# internal/adapter and internal/platform) with statements must have at least
# the given statement coverage (go-style.md GS-T, about 80 % in the domain).
# Packages without statements (types and interfaces only) have nothing to cover.
#
# Usage: check-coverage.sh [minimum-percent]   (default 80)
set -eu

min=${1:-80}
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM

packages=$(go list ./internal/... | grep -v '/internal/adapter\(/\|$\)' | grep -v '/internal/platform\(/\|$\)' || true)
if [ -z "$packages" ]; then
  echo "cover: no area packages"
  exit 0
fi

# shellcheck disable=SC2086 # the package list is word-split on purpose
if ! go test -count=1 -cover $packages >"$work/out.txt" 2>&1; then
  cat "$work/out.txt" >&2
  echo "cover: tests failed" >&2
  exit 1
fi

status=0
while IFS= read -r line; do
  case "$line" in
    *"[no test files]"*) continue ;; # no statements: types and interfaces only
  esac
  pkg=$(printf '%s\n' "$line" | awk '{ print ($1 == "ok" || $1 == "FAIL") ? $2 : $1 }')
  pct=$(printf '%s\n' "$line" | sed -n 's/.*coverage: \([0-9.]*\)% of statements.*/\1/p')
  if [ -z "$pct" ]; then
    continue
  fi
  if awk -v got="$pct" -v want="$min" 'BEGIN { exit !(got + 0 < want + 0) }'; then
    echo "cover: $pkg has $pct% coverage, need at least $min%" >&2
    status=1
  else
    echo "cover: $pkg $pct%"
  fi
done <"$work/out.txt"

if [ "$status" -eq 0 ]; then
  echo "cover: ok (minimum $min%)"
fi
exit "$status"
