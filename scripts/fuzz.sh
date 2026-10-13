#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make fuzz: runs every Fuzz* target of every workspace module for FUZZTIME
# (default 10s per target). Nightly CI uses FUZZTIME=5m.
#
# Usage: fuzz.sh [fuzztime]
set -eu

fuzztime=${1:-${FUZZTIME:-10s}}
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

status=0
count=0
for dir in $(go list -m -f '{{.Dir}}'); do
  for pkg in $(cd "$dir" && go list ./...); do
    targets=$(cd "$dir" && go test -list '^Fuzz' "$pkg" 2>/dev/null | grep '^Fuzz' || true)
    for target in $targets; do
      count=$((count + 1))
      echo "fuzz: $pkg $target ($fuzztime)"
      if ! (cd "$dir" && go test -run '^$' -fuzz "^${target}\$" -fuzztime "$fuzztime" "$pkg"); then
        status=1
      fi
    done
  done
done

if [ "$count" -eq 0 ]; then
  echo "fuzz: no Fuzz targets yet"
fi
exit "$status"
