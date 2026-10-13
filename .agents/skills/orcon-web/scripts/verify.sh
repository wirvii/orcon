#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Full local web gate for Orcon: runs make web-check, make web-e2e and
# make web-perf, the same steps as the ui-quality, web-e2e and web-perf CI
# jobs, stopping at the first failure.
#
# Usage: sh .agents/skills/orcon-web/scripts/verify.sh
#
# Exit status: 0 when every target passes, otherwise the status of the first
# failing target.

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "verify.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

make -C "$repo_root" web-check
make -C "$repo_root" web-e2e
make -C "$repo_root" web-perf
