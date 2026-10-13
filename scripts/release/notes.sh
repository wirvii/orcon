#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Prints the release notes of a tag: the section of CHANGELOG.md for that
# version (without its heading) followed by build/release/notes-footer.md with
# {{VERSION}} replaced (orcon-contributing RL-6, ADR 0081).
#
# Usage: sh scripts/release/notes.sh vX.Y.Z
# CHANGELOG and NOTES_FOOTER override the input files (used by notes-selftest.sh).
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: notes.sh vX.Y.Z" >&2
  exit 2
fi

root=$(cd "$(dirname "$0")/../.." && pwd)
changelog=${CHANGELOG:-$root/CHANGELOG.md}
footer=${NOTES_FOOTER:-$root/build/release/notes-footer.md}
version=${1#v}

# The section starts at "## [X.Y.Z]" (release-please) or "## X.Y.Z" and ends at the next "## ".
section=$(awk -v v="$version" '
  /^## / {
    if (found) { exit }
    head = $0
    if (index(head, "## [" v "]") == 1 || head == "## " v || index(head, "## " v " ") == 1) { found = 1; next }
  }
  found { print }
' "$changelog")

if [ -z "$(printf '%s' "$section" | tr -d '[:space:]')" ]; then
  echo "notes.sh: no section for version $version in $changelog" >&2
  exit 1
fi

printf '%s\n' "$section" | sed -e '/./,$!d' | sed -e :a -e '/^[[:space:]]*$/{$d;N;ba' -e '}'
grep -v '^<!-- SPDX-' "$footer" | sed "s/{{VERSION}}/$version/g"
