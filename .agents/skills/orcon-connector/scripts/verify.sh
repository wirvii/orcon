#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Full local connector gate: runs make connectors-check and
# make connectors-version-check, exactly what the CI job `connectors` runs,
# stopping at the first failure.
#
# Usage: sh .agents/skills/orcon-connector/scripts/verify.sh [BASE]
#   BASE  git ref the version check compares against (default origin/main).
#
# Exit status: 0 when both targets pass.

set -eu

base=${1:-origin/main}

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "verify.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

make -C "$repo_root" connectors-check
exec make -C "$repo_root" connectors-version-check BASE="$base"
