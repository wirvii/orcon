#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make skill-check (SPEC-008 E.4, SPEC-009 4.5): every project skill conforms to
# the Agent Skills standard and its generated copies (the projections written by
# the skill's project.sh into the tool directories) are identical.
#
# The orcon-go skill is completed by SPEC-008 slice G6. The orcon-db skill is
# completed by SPEC-009 cut D5; until its SKILL.md exists the check says so on
# standard output and skips it, never silently. A skill whose SKILL.md exists is
# checked in full and a missing piece fails.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

# Same commit as the UI skill check of SPEC-007.
skills_ref=69ef37e9424c0a7ea9dd2293b559e43ec8176379

# check_skill <name> <complete-required>: 1 = a missing SKILL.md fails, 0 = declared skip.
check_skill() {
  name=$1
  required=$2
  skill=.agents/skills/$name

  if [ "$required" = 0 ] && [ ! -f "$skill/SKILL.md" ]; then
    echo "skill-check: $skill/SKILL.md does not exist yet (SPEC-009 cut D5 writes it); $name skipped"
    return 0
  fi

  for needed in "$skill/SKILL.md" "$skill/validation/run.sh" "$skill/scripts/check-frontmatter.sh"; do
    if [ ! -f "$needed" ]; then
      echo "skill-check: $needed does not exist; the $name skill is incomplete" >&2
      exit 1
    fi
  done

  sh "$skill/validation/run.sh"
  sh "$skill/scripts/check-frontmatter.sh"

  # Every other hidden directory with skills/<name> is a generated copy and must
  # be byte-identical; at least one has to exist.
  copies=$(find . -maxdepth 3 -type d -path "./.*/skills/$name" ! -path './.agents/*' | sort)
  if [ -z "$copies" ]; then
    echo "skill-check: no generated copy of $skill found; run 'make skill-sync'" >&2
    exit 1
  fi
  for copy in $copies; do
    diff -r "$skill" "$copy"
  done
  if ! command -v uvx >/dev/null 2>&1; then
    echo "skill-check: uv is required for the Agent Skills reference validator (https://docs.astral.sh/uv/)" >&2
    exit 1
  fi
  uvx --from "git+https://github.com/agentskills/agentskills@${skills_ref}#subdirectory=skills-ref" skills-ref validate "$skill"
}

check_skill orcon-go 1
check_skill orcon-db 0
echo "skill-check: ok"
