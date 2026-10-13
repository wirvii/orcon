#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make skill-check (SPEC-008 E.4): the orcon-go skill conforms to the Agent
# Skills standard and its generated copies (the projections written by the
# skill's project.sh into the tool directories) are identical.
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

# Every other hidden directory with skills/orcon-go is a generated copy and must
# be byte-identical; at least one has to exist.
copies=$(find . -maxdepth 3 -type d -path './.*/skills/orcon-go' ! -path './.agents/*' | sort)
if [ -z "$copies" ]; then
  echo "skill-check: no generated copy of $skill found; run 'make skill-sync'" >&2
  exit 1
fi
for copy in $copies; do
  diff -r "$skill" "$copy"
done
# The other project skills: structural validation and byte-identical copies.
for other in orcon-ui orcon-api orcon-connector orcon-contributing orcon-web; do
  oskill=.agents/skills/$other
  sh "$oskill/validation/run.sh"
  sh "$oskill/scripts/check-frontmatter.sh"
  for copy in $(find . -maxdepth 3 -type d -path "./.*/skills/$other" ! -path './.agents/*' | sort); do
    diff -r "$oskill" "$copy"
  done
done
if ! command -v uvx >/dev/null 2>&1; then
  echo "skill-check: uv is required for the Agent Skills reference validator (https://docs.astral.sh/uv/)" >&2
  exit 1
fi
uvx --from "git+https://github.com/agentskills/agentskills@${skills_ref}#subdirectory=skills-ref" skills-ref validate "$skill"
for other in orcon-ui orcon-api orcon-connector orcon-contributing orcon-web; do
  uvx --from "git+https://github.com/agentskills/agentskills@${skills_ref}#subdirectory=skills-ref" skills-ref validate ".agents/skills/$other"
done
echo "skill-check: ok"
