#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Refreshes the vendored Luxon bundle (ADR-0018 section 1: Luxon, MIT, used
# unmodified). The global build of the pinned npm release is copied byte for
# byte as luxon.min.js together with its licence text.
#
#   sh internal/expr/prelude/vendor-luxon.sh
set -eu

LUXON_URL="https://registry.npmjs.org/luxon/-/luxon-3.7.2.tgz"
LUXON_SHA256="498078da47254917ea264631160880fec8936994de3e6271e9c75c542bd35832"

here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM

curl -fsSL -o "$work/luxon.tgz" "$LUXON_URL"
echo "$LUXON_SHA256  $work/luxon.tgz" | shasum -a 256 -c - >/dev/null
mkdir "$work/pkg"
tar -xzf "$work/luxon.tgz" -C "$work/pkg"
cp "$work/pkg/package/build/global/luxon.min.js" "$here/luxon.min.js"
cp "$work/pkg/package/LICENSE.md" "$here/LICENSE.luxon"
shasum -a 256 "$here/luxon.min.js"
