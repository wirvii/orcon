#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make licenses (SPEC-008 CFG-4, dependencies.md DEP-2): lists the licenses of
# every dependency linked into each module (the workspace modules and tools)
# with go-licenses and fails on any license outside Apache-2.0, MIT,
# BSD-2-Clause, BSD-3-Clause and ISC, unless the exact module@version has an
# owner-approved line in tools/licenses/exceptions.txt (tools/licenses decides).
# Test-only dependencies are included.
#
# go-licenses is a Go tool pinned in tools/go.mod. Its module needs a newer Go
# than the oldest supported release, so it runs with GOTOOLCHAIN=auto.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
checker="$root/.bin/licensecheck"
exceptions="$root/tools/licenses/exceptions.txt"

if [ ! -x "$checker" ]; then
  echo "licenses: $checker is missing; run 'make tools'" >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM
# One row per dependency module: module, version, license (tab separated).
printf '{{range .}}{{.Name}}\t{{.Version}}\t{{.LicenseName}}\n{{end}}' >"$work/rows.tmpl"

status=0
dirs="$(cd "$root" && go list -m -f '{{.Dir}}') $root/tools"
for dir in $dirs; do
  module=$(cd "$dir" && GOWORK=off go list -m)
  echo "licenses: $module"
  if ! (cd "$dir" && GOWORK=off GOTOOLCHAIN=auto go tool -modfile="$root/tools/go.mod" go-licenses report ./... --include_tests --template "$work/rows.tmpl" >"$work/rows.txt" 2>"$work/report.log"); then
    cat "$work/report.log" >&2
    echo "licenses: go-licenses failed for $module" >&2
    status=1
    continue
  fi
  if ! "$checker" -exceptions "$exceptions" -ignore "$module" <"$work/rows.txt"; then
    status=1
  fi
done

if [ "$status" -eq 0 ]; then
  echo "licenses: ok"
fi
exit "$status"
