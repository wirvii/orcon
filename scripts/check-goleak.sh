#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make goleak-check: a package that starts goroutines (a `go` statement or an
# errgroup/lifecycle `.Go(` call) in non-test code must have a test file with
# goleak.VerifyTestMain (go-style.md GS-K6).
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

status=0
checked=0
# Go files under the repository, without generated code, test data and the
# vendored or built trees.
dirs=$(find . \( -name node_modules -o -name testdata -o -name .bin -o -name dist -o -name .git \) -prune -o -name '*.go' ! -name '*_test.go' ! -name '*.gen.go' -print |
  xargs -n 50 dirname 2>/dev/null | sort -u)

for dir in $dirs; do
  # shellcheck disable=SC2046 # one path per non-test file of the directory
  starts=$(find "$dir" -maxdepth 1 -name '*.go' ! -name '*_test.go' ! -name '*.gen.go' -exec grep -lE '^[[:space:]]*go[[:space:]]+[A-Za-z_(]|\.Go\(' {} + 2>/dev/null || true)
  [ -n "$starts" ] || continue
  checked=$((checked + 1))
  if ! find "$dir" -maxdepth 1 -name '*_test.go' -exec grep -l 'goleak\.VerifyTestMain' {} + >/dev/null 2>&1; then
    echo "goleak-check: $dir starts goroutines but no test file calls goleak.VerifyTestMain:" >&2
    echo "$starts" | sed 's/^/  /' >&2
    echo "  add a TestMain with goleak.VerifyTestMain(m)" >&2
    status=1
  fi
done

if [ "$status" -eq 0 ]; then
  echo "goleak-check: ok ($checked package(s) start goroutines)"
fi
exit "$status"
