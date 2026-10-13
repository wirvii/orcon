#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make commitlint-selftest (orcon-contributing CM-12, ADR 0081): the commit
# message rules accept every message in scripts/testdata/commit-messages/valid,
# reject every message in .../invalid, and the scope list of
# commitlint.config.mjs equals the CM-3 table of references/commits.md.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

if [ ! -x node_modules/.bin/commitlint ]; then
  echo "commitlint-selftest: commitlint is not installed; run pnpm install" >&2
  exit 1
fi

fixtures=scripts/testdata/commit-messages
status=0

for f in "$fixtures"/valid/*.txt; do
  if ! pnpm exec commitlint --config commitlint.config.mjs <"$f" >/dev/null 2>&1; then
    echo "commitlint-selftest: FAIL, valid message rejected: $f" >&2
    status=1
  fi
done

for f in "$fixtures"/invalid/*.txt; do
  if pnpm exec commitlint --config commitlint.config.mjs <"$f" >/dev/null 2>&1; then
    echo "commitlint-selftest: FAIL, invalid message accepted: $f" >&2
    status=1
  fi
done

# The closed scope list must equal the CM-3 table (first column of the rows
# between the "CM-3" rule and the "CM-4" rule).
configured=$(node --input-type=module -e "
import config from './commitlint.config.mjs';
console.log([...config.rules['scope-enum'][2]].sort().join('\n'));
")
documented=$(sed -n '/^- \*\*CM-3\*\*/,/^- \*\*CM-4\*\*/p' .agents/skills/orcon-contributing/references/commits.md |
  sed -n 's/^| `\([a-z]*\)` |.*/\1/p' | sort)
if [ "$configured" != "$documented" ]; then
  echo "commitlint-selftest: FAIL, scope list differs from the CM-3 table" >&2
  echo "configured: $(echo "$configured" | tr '\n' ' ')" >&2
  echo "documented: $(echo "$documented" | tr '\n' ' ')" >&2
  status=1
fi

if [ "$status" -eq 0 ]; then
  echo "commitlint-selftest: ok"
fi
exit "$status"
