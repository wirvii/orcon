#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Fast local loop for one connector folder while iterating: admission lint,
# the connector's Go tests and the documentation drift check. Before handing
# back, run the full gate (scripts/verify.sh).
#
# Usage: sh .agents/skills/orcon-connector/scripts/connector-change-check.sh <connector folder>
#   e.g. internal/adapter/connectors/acme
#
# Builds bin/orcon first (make build-go) so the commands match the branch.

set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: connector-change-check.sh <connector folder>" >&2
  exit 2
fi

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "connector-change-check.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

dir=${1%/}
case "$dir" in
/*) dir=${dir#"$repo_root"/} ;;
esac
if [ ! -d "$repo_root/$dir" ]; then
  echo "connector-change-check.sh: not a directory: $dir" >&2
  exit 2
fi

cd "$repo_root"
make build-go
bin/orcon connector lint --admission "$dir"
go test -count=1 "./$dir/..."
bin/orcon connector docs --check "$dir"
