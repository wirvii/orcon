#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Fast local subset of the Go gate, for use while iterating: formats the code,
# then runs the linters and the tests (make fmt lint test). It rewrites files
# whose formatting is off. Before handing back, run the full gate
# (scripts/verify.sh, which is make check).
#
# Usage: sh .agents/skills/orcon-go/scripts/go-change-check.sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "go-change-check.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

exec make -C "$repo_root" fmt lint test
