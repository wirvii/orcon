#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Full local API gate for Orcon: runs make api-check (lint, compatibility,
# changelog self-test, reference build) and make api-examples (the examples
# against the real binary on SQLite), exactly what the CI job api-contract runs.
#
# Usage: sh .agents/skills/orcon-api/scripts/verify.sh
#
# Exit status: the exit status of make (0 when every target passes).

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "verify.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

exec make -C "$repo_root" api-check api-examples
