#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make db-fixtures (SPEC-009 4.4.6): proves that every database rule detects
# what it is meant to detect. It runs the planted violations and the good
# fixtures and exits 0 only if every bad case is detected and every good one
# passes:
#
#   29 static cases   tools/dbcheck/testdata/{bad,good}, run through .bin/dbcheck
#   23 schema cases   dbrules/testdata/{bad,good}, SQLite and PostgreSQL
#    2 plan cases     a full scan is detected in both engines; indexed queries pass
#    1 pragma case    an opener without foreign_keys is detected
#    2 upgrade cases  a migration that breaks seeded data is detected; a correct pair passes
#
# The PostgreSQL cases need a container runtime (testcontainers) or
# ORCON_TEST_POSTGRES_DSN; ORCON_TEST_POSTGRES_IMAGE picks the image. Pass
# --sqlite-only to run only what needs no server; the script then says plainly
# that the PostgreSQL cases did not run and `make test-postgres` must.
#
# Environment: BIN (default .bin; holds dbcheck, built by `make tools`).
set -eu

sqlite_only=0
case "${1:-}" in
  "") ;;
  --sqlite-only) sqlite_only=1 ;;
  *)
    echo "usage: check-db-fixtures.sh [--sqlite-only]" >&2
    exit 2
    ;;
esac

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
BIN=${BIN:-$root/.bin}
dbcheck=$BIN/dbcheck
policy=internal/adapter/storetest/dbrules/policy

if [ ! -x "$dbcheck" ]; then
  echo "db-fixtures: $dbcheck not found; run 'make tools'" >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM
status=0

# The planted cases have queries of their own: the entries of the real
# dialect_only_queries.txt name queries of the real adapters and would be
# reported as stale, so the cases run against a copy of the policy without them.
cp -R "$policy" "$work/policy"
grep '^#' "$policy/dialect_only_queries.txt" > "$work/policy/dialect_only_queries.txt"
policy=$work/policy

# --- static cases -----------------------------------------------------------
cases=tools/dbcheck/testdata
bad=0
for dir in "$cases"/bad/*/; do
  name=$(basename "$dir")
  bad=$((bad + 1))
  want=$(sort <"$dir/expect.txt" | tr '\n' ' ')
  if "$dbcheck" -sqlite "$dir/sqlite" -postgres "$dir/postgres" -policy "$policy" -base none all >"$work/out.txt" 2>"$work/err.txt"; then
    echo "db-fixtures: static case $name was NOT detected (dbcheck passed)" >&2
    status=1
    continue
  fi
  got=$(grep -E '^[^ ]+:[0-9]+: ' "$work/out.txt" | awk '{ print $2 }' | sort -u | tr '\n' ' ')
  if [ "$got" != "$want" ]; then
    echo "db-fixtures: static case $name reports rules [$got], expected [$want]" >&2
    status=1
  fi
done
if [ "$bad" -ne 29 ]; then
  echo "db-fixtures: found $bad static cases, SPEC-009 4.4.6 requires 29" >&2
  status=1
fi
for dir in "$cases"/good/*/; do
  name=$(basename "$dir")
  if ! "$dbcheck" -sqlite "$dir/sqlite" -postgres "$dir/postgres" -policy "$policy" -base none all >"$work/out.txt" 2>&1; then
    echo "db-fixtures: good static case $name does not pass:" >&2
    cat "$work/out.txt" >&2
    status=1
  fi
done
echo "db-fixtures: $bad static cases run through dbcheck"

# --- database cases ---------------------------------------------------------
# require_pass <test-name>...: every named test must run and pass; a test that
# is skipped or missing is a failure, never a pass.
require_pass() {
  log=$1
  shift
  for t in "$@"; do
    if ! grep -q -- "^--- PASS: $t " "$log"; then
      echo "db-fixtures: $t did not pass (missing, skipped or failed)" >&2
      status=1
    fi
  done
}

sqlite_tests="TestSchemaLintGoodSQLite TestSchemaLintPlantedSQLite TestForeignKeysOnEveryConnection TestOpenerWithoutForeignKeysIsDetected TestPlansIndexedQueryPassesSQLite TestUpgradeDetectsAMigrationThatBreaksSeededData TestUpgradeGoodPairHasNoFindings TestEnumValueRule"
pattern=$(echo "$sqlite_tests" | tr ' ' '|')
# shellcheck disable=SC2086 # the pattern has no spaces
if ! go test -count=1 -v -run "^($pattern)\$" ./internal/adapter/storetest/dbrules/ >"$work/sqlite.log" 2>&1; then
  cat "$work/sqlite.log" >&2
  echo "db-fixtures: the SQLite cases failed" >&2
  status=1
fi
# shellcheck disable=SC2086 # the test names are separate words
require_pass "$work/sqlite.log" $sqlite_tests
echo "db-fixtures: SQLite schema, pragma, plan and upgrade cases run"

if [ "$sqlite_only" -eq 1 ]; then
  echo "db-fixtures: PostgreSQL cases NOT run (--sqlite-only); run 'make test-postgres' and 'make db-fixtures' for them" >&2
else
  pg_tests="TestSchemaLintGoodPostgres TestSchemaLintPlantedPostgres TestPostgresSession TestPlansDetectFullScan TestPlansDetectSort TestEngineFunctionPlansPostgres TestUpgradeFromSeededPreviousVersion TestUpgradeDetectsAMigrationThatBreaksSeededData TestUpgradeGoodPairHasNoFindings TestUpgradeRequiresTwoOrganizationsInSeeds TestRowsAreVisibleThroughTheOrganizationContext"
  pattern=$(echo "$pg_tests" | tr ' ' '|')
  if ! ORCON_TEST_POSTGRES=1 go test -count=1 -v -run "^($pattern)\$" ./internal/adapter/storetest/dbrules/ >"$work/postgres.log" 2>&1; then
    cat "$work/postgres.log" >&2
    echo "db-fixtures: the PostgreSQL cases failed" >&2
    status=1
  fi
  # shellcheck disable=SC2086 # the test names are separate words
  require_pass "$work/postgres.log" $pg_tests
  if grep -q -- '--- SKIP' "$work/postgres.log"; then
    echo "db-fixtures: a PostgreSQL case was skipped" >&2
    status=1
  fi
  echo "db-fixtures: PostgreSQL schema, session, plan and upgrade cases run"
fi

if [ "$status" -eq 0 ]; then
  echo "db-fixtures: ok"
fi
exit "$status"
