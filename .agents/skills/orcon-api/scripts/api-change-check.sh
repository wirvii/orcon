#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Fast local subset of the API gate, for use while iterating on the contract:
# lints it and checks compatibility against the latest release
# (make api-lint api-compat). Before handing back, run the full gate
# (scripts/verify.sh).
#
# Usage: sh .agents/skills/orcon-api/scripts/api-change-check.sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "api-change-check.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

exec make -C "$repo_root" api-lint api-compat
