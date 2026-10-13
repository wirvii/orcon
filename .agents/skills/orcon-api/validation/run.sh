#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Validation script of the orcon-api skill (mneme `skills validate` runs it
# from the skill directory; make skill-check runs it from the repository root).
# POSIX sh, deterministic, no network access. Exit status 0 means valid.
#
# Checks:
#   1. Frontmatter (scripts/check-frontmatter.sh): allowed and required keys,
#      name equal to the directory, unquoted semver metadata.version,
#      metadata.pinned true, single-line description of at most 500 characters
#      with "Use when".
#   2. Each required H2 section exactly once and no other H2.
#   3. Automated Checks table: exact 3-column header, at least 18 rows.
#   4. SKILL.md has at most 300 lines.
#   5. The 14 references/ files exist, each with SPDX lines and a Status: line;
#      scripts/, assets/ and the 5 templates exist.
#   6. SKILL.md carries the SPDX lines right after the frontmatter.
#   7. paths.md lists exactly 31 action verbs; responses.md has RESP-1..RESP-6.
#   8. No file of the skill names a build assistant or its vendor.
#   9. When ../orcon-go exists, the links to its api-codegen.md resolve.

set -u

skill_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
skill_md="$skill_dir/SKILL.md"
errors=0

fail() {
  echo "FAIL $*"
  errors=$((errors + 1))
}

pass() {
  echo "PASS $*"
}

spdx_copyright='SPDX-FileCopyrightText: The Orcon Authors'
spdx_license='SPDX-License-Identifier: Apache-2.0'

# ---------------------------------------------------------------- 5. tree
references="paths fields responses pagination errors idempotency-and-concurrency rate-limits long-running events versioning channels documentation extensions review"

missing=0
for f in \
  SKILL.md \
  scripts/project.sh \
  scripts/check-frontmatter.sh \
  scripts/check-api-templates.sh \
  scripts/api-change-check.sh \
  scripts/verify.sh \
  assets/pr-api-checklist.md \
  assets/templates/operation.yaml \
  assets/templates/list-operation.yaml \
  assets/templates/example-block.yaml \
  assets/templates/event.yaml \
  assets/templates/problem-code.yaml \
  validation/run.sh; do
  if [ ! -f "$skill_dir/$f" ]; then
    fail "tree: missing $f"
    missing=1
  fi
done
for r in $references; do
  f="$skill_dir/references/$r.md"
  if [ ! -f "$f" ]; then
    fail "tree: missing references/$r.md"
    missing=1
    continue
  fi
  if ! grep -qF "$spdx_copyright" "$f" || ! grep -qF "$spdx_license" "$f"; then
    fail "references/$r.md: SPDX lines with holder 'The Orcon Authors' are required"
  fi
  if ! grep -Eq '^Status: [^[:space:]]' "$f"; then
    fail "references/$r.md: a 'Status:' line is required"
  fi
done
[ "$missing" -eq 0 ] && pass "tree: all files present"

if [ ! -f "$skill_md" ]; then
  echo "orcon-api: SKILL.md missing, cannot continue"
  exit 1
fi

# ---------------------------------------------------------------- 1. frontmatter
if sh "$skill_dir/scripts/check-frontmatter.sh" "$skill_dir"; then
  pass "frontmatter"
else
  fail "frontmatter (see messages above)"
fi

closing=$(awk 'NR > 1 && /^---$/ { print NR; exit }' "$skill_md")
closing=${closing:-1}
body=$(awk -v c="$closing" 'NR > c { print }' "$skill_md")

# ---------------------------------------------------------------- 6. SPDX after the frontmatter
first=$(printf '%s\n' "$body" | sed -n '1p')
second=$(printf '%s\n' "$body" | sed -n '2p')
if [ "$first" = "<!-- $spdx_copyright -->" ] && [ "$second" = "<!-- $spdx_license -->" ]; then
  pass "SPDX lines right after the frontmatter"
else
  fail "SKILL.md: the two SPDX comment lines must follow the closing '---'"
fi

# ---------------------------------------------------------------- 2. H2 sections
h2=$(printf '%s\n' "$body" | awk '/^```/ { fence = !fence; next } !fence && /^## / { print }')
for s in "When To Use" "Critical Rules" "Automated Checks" "Verification" "Workflow"; do
  n=$(printf '%s\n' "$h2" | grep -cx "## $s" || true)
  if [ "$n" -eq 1 ]; then
    pass "sections: '## $s' once"
  else
    fail "sections: '## $s' appears $n times (expected 1)"
  fi
done
other=$(printf '%s\n' "$h2" | grep -vx -e '## When To Use' -e '## Critical Rules' -e '## Automated Checks' -e '## Verification' -e '## Workflow' | grep -v '^$' || true)
if [ -n "$other" ]; then
  fail "sections: unexpected H2: $(printf '%s' "$other" | tr '\n' ';')"
else
  pass "sections: no other H2"
fi

# ---------------------------------------------------------------- 3. checks table
header_count=$(grep -cxF '| Check | What it verifies | How to fix |' "$skill_md" || true)
if [ "$header_count" -eq 1 ]; then
  pass "checks table: header present once"
else
  fail "checks table: header '| Check | What it verifies | How to fix |' found $header_count times (expected 1)"
fi
rows=$(printf '%s\n' "$body" | awk '
  /^## / { insec = ($0 == "## Automated Checks"); next }
  insec && /^\|/ { n++ }
  END { print n + 0 }')
rows=$((rows - 2))
if [ "$rows" -ge 18 ]; then
  pass "checks table: $rows rows"
else
  fail "checks table: $rows rows (expected at least 18)"
fi

# ---------------------------------------------------------------- 4. size
lines=$(wc -l <"$skill_md" | tr -d ' ')
if [ "$lines" -le 300 ]; then
  pass "SKILL.md: $lines lines <= 300"
else
  fail "SKILL.md: $lines lines > 300"
fi

# ---------------------------------------------------------------- 7. verbs and responses
paths_md="$skill_dir/references/paths.md"
if [ -f "$paths_md" ]; then
  verbs=$(grep -Ec '^[[:space:]]*\| `[a-z]+` \| ' "$paths_md" || true)
  if [ "$verbs" -eq 31 ]; then
    pass "paths.md: 31 action verbs"
  else
    fail "paths.md: $verbs action verb rows (expected 31)"
  fi
fi
responses_md="$skill_dir/references/responses.md"
if [ -f "$responses_md" ]; then
  for n in 1 2 3 4 5 6; do
    if ! grep -qF "**RESP-$n**" "$responses_md"; then
      fail "responses.md: RESP-$n is missing"
    fi
  done
  pass "responses.md: RESP rules checked"
fi

# ---------------------------------------------------------------- 8. provenance terms
# Names of build assistants and their vendors (class B of the provenance check).
terms='claude|codex|anthropic|openai|chatgpt|copilot'
hits=$(cd "$skill_dir" && grep -rliwE "$terms" . | grep -v -e '^\./validation/run\.sh$' -e '^\./scripts/project\.sh$' || true)
if [ -n "$hits" ]; then
  fail "provenance: assistant or vendor names in: $(printf '%s' "$hits" | tr '\n' ' ')"
else
  pass "provenance: no assistant or vendor names"
fi

# ---------------------------------------------------------------- 9. orcon-go links
if [ -d "$skill_dir/../orcon-go" ]; then
  if [ -f "$skill_dir/../orcon-go/references/api-codegen.md" ]; then
    pass "orcon-go: api-codegen.md resolves"
  else
    fail "orcon-go: ../orcon-go/references/api-codegen.md not found"
  fi
fi

if [ "$errors" -gt 0 ]; then
  echo "orcon-api: validation failed ($errors check(s))"
  exit 1
fi
echo "orcon-api: validation passed"
