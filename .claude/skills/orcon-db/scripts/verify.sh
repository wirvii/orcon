#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Full local database gate for Orcon: runs make check, exactly what CI runs, in
# CI order, then make test-postgres, which runs the adapter and database rule
# tests on PostgreSQL 14 and 17 (needs a container runtime). Stops at the first
# failure.
#
# Usage: sh .agents/skills/orcon-db/scripts/verify.sh
#
# Exit status: 0 when make check and make test-postgres both pass.

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "verify.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

make -C "$repo_root" check
exec make -C "$repo_root" test-postgres
