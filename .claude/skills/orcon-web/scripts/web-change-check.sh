#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Fast local subset of the web gate, for use while iterating: runs lint, type
# checking and tests only for the workspace packages changed since origin/main
# (and the packages that depend on them). Before handing back, run the full
# gate (scripts/verify.sh).
#
# Usage: sh .agents/skills/orcon-web/scripts/web-change-check.sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "web-change-check.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

cd "$repo_root"
for script in lint typecheck test; do
  pnpm --filter "...[origin/main]" --if-present run "$script"
done
