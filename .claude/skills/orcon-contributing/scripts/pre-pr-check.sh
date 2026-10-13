#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Run before opening a pull request: lints every commit of the branch against
# origin/main (commits.md, CM-1..CM-12) and prints the advisory pull request
# checks without a body: the measured size against the 400-line limit and the
# skills whose checklists apply (pull-requests.md, PR-6, PR-7).
#
# Usage: sh .agents/skills/orcon-contributing/scripts/pre-pr-check.sh [base]
# Default base: origin/main.
#
# Exit status: non-zero when a commit message fails the lint or a check
# cannot run; size and skill findings are advisory and never fail.

set -eu

base=${1:-origin/main}

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "pre-pr-check.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}

echo "== commit messages ($base..HEAD)"
make -s -C "$repo_root" commit-lint FROM="$base" TO=HEAD || {
  echo "pre-pr-check.sh: reword the failing commits with git rebase -i $base" >&2
  exit 1
}

echo "== size and applicable skills"
make -s -C "$repo_root" pr-check FROM="$base" TO=HEAD BODY= BRANCH="$(git -C "$repo_root" branch --show-current)"
