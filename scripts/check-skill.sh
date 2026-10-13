#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make skill-check (SPEC-008 E.4, SPEC-009 4.5): every project skill conforms to
# the Agent Skills standard and its generated copies (the projections written by
# the skill's project.sh into the tool directories) are identical.
#
# Every listed skill is required: a missing SKILL.md, validation script or
# frontmatter check fails.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

# Same commit as the UI skill check of SPEC-007.
skills_ref=69ef37e9424c0a7ea9dd2293b559e43ec8176379

# check_skill <name>
check_skill() {
  name=$1
  skill=.agents/skills/$name

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

for name in orcon-go orcon-db orcon-ui orcon-api orcon-connector orcon-contributing orcon-web; do
  check_skill "$name"
done
echo "skill-check: ok"
