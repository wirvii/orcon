#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Prints a pull request body: the repository template with the checklist line
# of every applicable skill ticked and, below the list, the content of each
# applicable skill's assets/pr-*-checklist.md (pull-requests.md, PR-6). The
# applicable skills come from tools/prcheck (the same SkillMap the advisory CI
# check uses), so the body and the check never disagree.
#
# Usage: sh .agents/skills/orcon-contributing/scripts/compose-pr-body.sh [base] > pr-body.md
# Default base: origin/main. Complete every section before opening the pull
# request.

set -eu

base=${1:-origin/main}

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "compose-pr-body.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}
template="$repo_root/.github/pull_request_template.md"
if [ ! -f "$template" ]; then
  echo "compose-pr-body.sh: $template not found" >&2
  exit 1
fi
if [ ! -d "$repo_root/tools/prcheck" ]; then
  echo "compose-pr-body.sh: tools/prcheck not found; it computes the applicable skills" >&2
  exit 1
fi

skills=$(GOWORK=off go run -C "$repo_root/tools" ./prcheck skills --list \
  --repo "$repo_root" --base "$base" --head HEAD) || {
  echo "compose-pr-body.sh: tools/prcheck failed" >&2
  exit 1
}

# Tick "- [ ] <skill> checklist" for every applicable skill.
body=$(cat "$template")
for skill in $skills; do
  body=$(printf '%s\n' "$body" | sed "s/^- \[ \] $skill checklist/- [x] $skill checklist/")
done

# Append each applicable checklist right after the skill list, before
# "### Every pull request".
printf '%s\n' "$body" | while IFS= read -r line; do
  if [ "$line" = "### Every pull request" ]; then
    for skill in $skills; do
      found=0
      for checklist in "$repo_root/.agents/skills/$skill/assets"/pr-*-checklist.md; do
        [ -f "$checklist" ] || continue
        found=1
        grep -v '^<!-- SPDX-' "$checklist"
        printf '\n'
      done
      if [ "$found" -eq 0 ]; then
        echo "compose-pr-body.sh: warning: $skill has no assets/pr-*-checklist.md" >&2
      fi
    done
  fi
  printf '%s\n' "$line"
done
