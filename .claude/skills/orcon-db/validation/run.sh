#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Validation script of the orcon-db skill (mneme `skills validate` runs it
# from the skill directory; make skill-check runs it from the repository root).
# POSIX sh, deterministic, no network access. Exit status 0 means valid.
#
# Checks:
#   1. Frontmatter (scripts/check-frontmatter.sh): allowed and required keys,
#      name rules, unquoted semver metadata.version, metadata.pinned true,
#      single-line description of at most 500 characters with "Use when".
#   2. Each required H2 section exactly once and no other H2.
#   3. Automated Checks table: exact 3-column header, at least 15 rows.
#   4. SKILL.md has at most 300 lines.
#   5. The 11 references/ files exist, each with SPDX lines and a Status: line,
#      and every file of the skill tree exists.
#   6. SKILL.md carries the SPDX lines right after the frontmatter.

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
references="naming identifiers types-and-time migrations tenant-isolation integrity deletion-and-retention json queries-and-transactions connections checks"

missing=0
for f in \
  SKILL.md \
  scripts/project.sh \
  scripts/check-frontmatter.sh \
  scripts/db-change-check.sh \
  scripts/verify.sh \
  assets/migration-sqlite.sql \
  assets/migration-postgres.sql \
  assets/queries-template.sql \
  assets/pr-db-checklist.md \
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
  echo "orcon-db: SKILL.md missing, cannot continue"
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
if [ "$rows" -ge 15 ]; then
  pass "checks table: $rows rows"
else
  fail "checks table: $rows rows (expected at least 15)"
fi
bad_cols=$(printf '%s\n' "$body" | awk -F'|' '
  /^## / { insec = ($0 == "## Automated Checks"); next }
  insec && /^\|/ && NF != 5 { print NR }')
if [ -n "$bad_cols" ]; then
  fail "checks table: rows without exactly 3 columns (body lines: $(printf '%s' "$bad_cols" | tr '\n' ' '))"
fi

# ---------------------------------------------------------------- 4. size
lines=$(wc -l <"$skill_md" | tr -d ' ')
if [ "$lines" -le 300 ]; then
  pass "SKILL.md: $lines lines <= 300"
else
  fail "SKILL.md: $lines lines > 300"
fi

if [ "$errors" -gt 0 ]; then
  echo "orcon-db: validation failed ($errors check(s))"
  exit 1
fi
echo "orcon-db: validation passed"
