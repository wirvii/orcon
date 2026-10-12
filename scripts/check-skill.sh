#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make skill-check (SPEC-008 E.4): the orcon-go skill conforms to the Agent
# Skills standard and its generated Claude copy is identical.
#
# The skill is completed by SPEC-008 slice G6 (SKILL.md, scripts/, validation/).
# Until then this target fails with a clear message instead of passing.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

skill=.agents/skills/orcon-go
# Same commit as the UI skill check of SPEC-007.
skills_ref=69ef37e9424c0a7ea9dd2293b559e43ec8176379

for required in "$skill/SKILL.md" "$skill/validation/run.sh" "$skill/scripts/check-frontmatter.sh"; do
  if [ ! -f "$required" ]; then
    echo "skill-check: $required does not exist; the orcon-go skill is completed by SPEC-008 slice G6" >&2
    exit 1
  fi
done

sh "$skill/validation/run.sh"
sh "$skill/scripts/check-frontmatter.sh"
diff -r "$skill" .claude/skills/orcon-go
if ! command -v uvx >/dev/null 2>&1; then
  echo "skill-check: uv is required for the Agent Skills reference validator (https://docs.astral.sh/uv/)" >&2
  exit 1
fi
uvx --from "git+https://github.com/agentskills/agentskills@${skills_ref}#subdirectory=skills-ref" skills-ref validate "$skill"
echo "skill-check: ok"
