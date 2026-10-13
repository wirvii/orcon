#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make release-pins-check: the tool versions written in .github/workflows/release.yml
# (GoReleaser and cosign, installed there by actions) equal the pins in
# tools/versions.mk that scripts/release/install-tools.sh installs (ADR 0081, RL-12).
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
workflow=$root/.github/workflows/release.yml
versions=$root/tools/versions.mk
status=0

pin() {
  sed -n "s/^$1 := //p" "$versions"
}

# compare NAME MAKE_VARIABLE WORKFLOW_KEY
compare() {
  want=$(pin "$2")
  got=$(sed -n "s/^[[:space:]]*$3: \(v[0-9][^ ]*\)\$/\1/p" "$workflow" | head -n 1)
  if [ -z "$want" ] || [ -z "$got" ]; then
    echo "release-pins-check: cannot read $1 ($2 in tools/versions.mk or $3 in release.yml)" >&2
    status=1
  elif [ "$want" != "$got" ]; then
    echo "release-pins-check: $1 is $got in release.yml but $want in tools/versions.mk" >&2
    status=1
  fi
}

compare goreleaser GORELEASER_VERSION version
compare cosign COSIGN_VERSION cosign-release

if [ "$status" -eq 0 ]; then
  echo "release-pins-check: ok"
fi
exit "$status"
