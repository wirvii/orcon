#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Full local Go gate for Orcon: runs make check, exactly what CI runs, in CI
# order, stopping at the first failure.
#
# Usage: sh .agents/skills/orcon-go/scripts/verify.sh
#
# Exit status: the exit status of make check (0 when every target passes).

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "verify.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

exec make -C "$repo_root" check
