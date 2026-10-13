#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Fails when .golangci.yml does not cover the package layout: every directory
# of internal/adapter needs its adapter-private-<name> depguard rule, and every
# business area (a directory of internal/ other than adapter and platform) must
# be denied in the platform-leaf rule. See architecture-layers.md AL-11.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
config="$root/.golangci.yml"
status=0

if [ ! -f "$config" ]; then
  echo "arch-coverage: $config not found" >&2
  exit 1
fi

# Lines of the platform-leaf rule: from its key to the next rule at the same
# indentation (eight spaces).
platform_leaf=$(awk '
  /^        platform-leaf:/ { inside = 1; next }
  inside && /^        [A-Za-z]/ { inside = 0 }
  inside { print }
' "$config")

for dir in "$root"/internal/adapter/*/; do
  [ -d "$dir" ] || continue
  name=$(basename "$dir")
  if ! grep -q "^        adapter-private-$name:" "$config"; then
    echo "arch-coverage: internal/adapter/$name has no adapter-private-$name rule in .golangci.yml" >&2
    status=1
  fi
done

for dir in "$root"/internal/*/; do
  [ -d "$dir" ] || continue
  name=$(basename "$dir")
  case "$name" in
    adapter | platform) continue ;;
  esac
  if ! printf '%s\n' "$platform_leaf" | grep -q "pkg: github.com/wirvii/orcon/internal/$name\$"; then
    echo "arch-coverage: area internal/$name is missing from the platform-leaf rule in .golangci.yml" >&2
    status=1
  fi
done

if [ "$status" -eq 0 ]; then
  echo "arch-coverage: ok"
fi
exit "$status"
