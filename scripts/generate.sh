#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make generate / make generate-check (SPEC-008 E.1, E.2, api-codegen.md,
# data-access.md): runs sqlc and oapi-codegen when their inputs exist, then
# gives the generated files their SPDX header.
#
#   generate.sh          regenerate
#   generate.sh --check  regenerate and fail when that changed any generated
#                        file, so a stale or hand-edited file is caught
#
# Inputs that do not exist yet are skipped with a notice. Nothing is invented
# to make a step run.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

check=0
case "${1:-}" in
  "") ;;
  --check) check=1 ;;
  *)
    echo "usage: generate.sh [--check]" >&2
    exit 2
    ;;
esac

sqlite_out=internal/adapter/sqlite/sqlcgen
postgres_out=internal/adapter/postgres/sqlcgen
api_out=internal/adapter/httpapi/apigen
generated="$sqlite_out $postgres_out $api_out"

# fingerprint prints one checksum line per generated file (existing or not).
fingerprint() {
  for dir in $generated; do
    [ -d "$dir" ] || continue
    find "$dir" -type f -name '*.go' -print0 | xargs -0 cksum 2>/dev/null
  done | sort
}

before=""
if [ "$check" -eq 1 ]; then
  before=$(fingerprint)
fi

# The tools module needs a newer Go than the oldest supported release.
tool() {
  GOWORK=off GOTOOLCHAIN=auto go tool -modfile="$root/tools/go.mod" "$@"
}

if ls internal/adapter/sqlite/queries/*.sql internal/adapter/postgres/queries/*.sql >/dev/null 2>&1; then
  if [ ! -x .bin/sqlc ]; then
    echo "generate: .bin/sqlc is missing; run 'make tools'" >&2
    exit 1
  fi
  .bin/sqlc generate
else
  echo "sqlc: no queries yet"
fi

bundle=packages/openapi/v1/openapi.bundle.yaml
if [ -f "$bundle" ]; then
  tool oapi-codegen -config "$api_out/oapi-codegen.yaml" "$bundle"
else
  echo "oapi-codegen: $bundle does not exist yet; skipped"
fi

# sqlc expands a slice parameter by rewriting the statement text at run time, so
# the generated call is not a constant. Mark that one reviewed call (owner
# approval in dynamic_sql_allowlist.txt) every time sqlc rewrites the file.
slice_file="$sqlite_out/engine.sql.go"
if [ -f "$slice_file" ] && ! grep -q 'orcon:dynamic-sql' "$slice_file"; then
  awk '
    /rows, err := q\.db\.QueryContext\(ctx, query, queryParams\.\.\.\)/ && !done {
      print "\t//orcon:dynamic-sql sqlc expands the fairness_keys slice parameter in the statement text"
      done = 1
    }
    { print }
  ' "$slice_file" >"$slice_file.tmp" && mv "$slice_file.tmp" "$slice_file"
fi

# Generated files carry the SPDX header like every other file (CFG-6).
existing=""
for dir in $generated; do
  [ -d "$dir" ] && existing="$existing $dir"
done
if [ -n "$existing" ]; then
  # shellcheck disable=SC2086 # the directory list is word-split on purpose
  find $existing -type f -name '*.go' | .bin/spdxcheck -fix
fi

if [ "$check" -eq 1 ]; then
  after=$(fingerprint)
  if [ "$before" != "$after" ]; then
    echo "generate-check: generated files are out of date; run 'make generate' and commit the result" >&2
    exit 1
  fi
  echo "generate-check: ok"
fi
