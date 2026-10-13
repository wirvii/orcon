#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Fast local subset of the database gate, for use while iterating: runs the
# static database checks (make db-check), then the Go tests of the database
# rule checks and of both storage adapters. Without ORCON_TEST_POSTGRES the
# PostgreSQL adapter tests skip, so this loop needs no container runtime and
# covers SQLite only. Before handing back, run the full gate (scripts/verify.sh).
#
# Usage: sh .agents/skills/orcon-db/scripts/db-change-check.sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "db-change-check.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

make -C "$repo_root" db-check
cd "$repo_root"
exec go test -race -count=1 \
  ./internal/adapter/storetest/... \
  ./internal/adapter/sqlite/... \
  ./internal/adapter/postgres/...
