#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make test-compat-floor (ADR-0015 section 4, SPEC-001 E-STO-5): the
# compatibility floor of every migration is proven, not declared.
#
#  1. Expand/contract control. The static checker rejects a migration that
#     drops or renames a column or table (or changes a type, or adds a NOT NULL
#     column without a default) outside a contract migration: the planted
#     violation is detected and the checker's own tests pass.
#  2. Floor verification. For every distinct floor F declared by a migration
#     header (other than "self"; written with or without a leading v), the checkout of release tag vF runs the smoke
#     subset of the storetest suite (TestFloorSmoke) against databases the
#     current code migrated, on SQLite and, with a container runtime, on
#     PostgreSQL. A floor without its release tag fails: the floor must name a
#     real release.
#  3. The machinery itself runs once with this tree playing the older release
#     (TestCompatFloorMachinery), so the target never passes without running the
#     procedure when no migration declares a floor yet.
#
# Environment: BIN (default .bin; holds dbcheck, built by `make tools`);
# ORCON_TEST_POSTGRES (default 1: PostgreSQL through testcontainers; set it to
# 0 to run SQLite only, and the script says so).
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
BIN=${BIN:-$root/.bin}
dbcheck=$BIN/dbcheck
policy=internal/adapter/storetest/dbrules/policy

if [ ! -x "$dbcheck" ]; then
  echo "test-compat-floor: $dbcheck not found; run 'make tools'" >&2
  exit 1
fi

work=$(mktemp -d)
worktrees=""
cleanup() {
  for dir in $worktrees; do
    git worktree remove --force "$dir" >/dev/null 2>&1 || true
  done
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

# --- 1. expand/contract control ------------------------------------------------
fixture=tools/dbcheck/testdata/bad/drop-column-in-additive
if "$dbcheck" -sqlite "$fixture/sqlite" -postgres "$fixture/postgres" -policy "$policy" -base none all >"$work/out.txt" 2>&1; then
  echo "test-compat-floor: the expand/contract control accepted a migration that drops a column in one release" >&2
  exit 1
fi
if ! grep -q 'migration-expand-contract\|MG-7' "$work/out.txt"; then
  echo "test-compat-floor: the planted violation failed, but not through the expand/contract rule:" >&2
  cat "$work/out.txt" >&2
  exit 1
fi
echo "test-compat-floor: the expand/contract control rejects a dropped column"
(cd tools && GOWORK=off GOTOOLCHAIN=auto go test -count=1 ./dbcheck -run 'TestExpandContract')

# --- 2. floors ------------------------------------------------------------------
floors=$(cat internal/adapter/sqlite/migrations/*.sql internal/adapter/postgres/migrations/*.sql |
  sed -n 's/^-- orcon:compatibility-floor //p' | sed 's/[[:space:]]*$//' | grep -v '^self$' | sed 's/^v//' | sort -u || true)
list=""
for floor in $floors; do
  tag="v$floor"
  if ! git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
    echo "test-compat-floor: a migration declares compatibility floor $floor, but there is no release tag $tag" >&2
    exit 1
  fi
  dir="$work/floor-$floor"
  git worktree add --detach "$dir" "$tag" >/dev/null
  worktrees="$worktrees $dir"
  list="$list${list:+;}$floor=$dir"
done

ORCON_TEST_POSTGRES=${ORCON_TEST_POSTGRES:-1}
export ORCON_TEST_POSTGRES
if [ "$ORCON_TEST_POSTGRES" != "1" ]; then
  echo "test-compat-floor: ORCON_TEST_POSTGRES is not 1; the PostgreSQL half did not run" >&2
fi
if [ -z "$list" ]; then
  echo "test-compat-floor: no migration declares a floor other than self; running the machinery only"
fi
ORCON_COMPAT_FLOORS=$list go test -count=1 -v -run 'TestCompatFloor|TestCompatFloorMachinery' ./internal/adapter/storetest/
echo "test-compat-floor: ok"
