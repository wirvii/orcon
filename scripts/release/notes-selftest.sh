#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Self-test of notes.sh: the fixture changelog yields exactly the expected notes
# for a published version, and a version without a section fails.
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
data=$root/scripts/testdata/release-notes

actual=$(CHANGELOG=$data/CHANGELOG.md NOTES_FOOTER=$data/footer.md sh "$root/scripts/release/notes.sh" v0.2.0)
expected=$(cat "$data/expected.md")
if [ "$actual" != "$expected" ]; then
  echo "notes-selftest: FAIL, notes for v0.2.0 differ from $data/expected.md" >&2
  printf '%s\n' "$actual" >&2
  exit 1
fi

if CHANGELOG=$data/CHANGELOG.md NOTES_FOOTER=$data/footer.md sh "$root/scripts/release/notes.sh" v9.9.9 >/dev/null 2>&1; then
  echo "notes-selftest: FAIL, a version without a section must fail" >&2
  exit 1
fi

# The real footer carries the placeholder that notes.sh replaces.
if ! grep -q '{{VERSION}}' "$root/build/release/notes-footer.md"; then
  echo "notes-selftest: FAIL, build/release/notes-footer.md has no {{VERSION}} placeholder" >&2
  exit 1
fi
echo "notes-selftest: ok"
