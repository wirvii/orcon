#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Local gate of the contribution workflow: the commitlint self-test, the pull
# request checks, the workflow lint and the release configuration check
# (make contrib-check), plus the skill check (make skill-check).
#
# Usage: sh .agents/skills/orcon-contributing/scripts/verify.sh
#
# Exit status: the exit status of make (0 when every target passes).

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "verify.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

exec make -C "$repo_root" contrib-check skill-check
