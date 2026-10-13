#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Validation script of the orcon-contributing skill (mneme `skills validate`
# runs it from the skill directory; make skill-check runs it from the
# repository root). POSIX sh, deterministic, no network access. Exit status 0
# means valid.
#
# Checks:
#   1. Frontmatter (scripts/check-frontmatter.sh).
#   2. Each required H2 section exactly once and no other H2.
#   3. Automated Checks table: exact 3-column header, at least 12 rows.
#   4. SKILL.md has at most 300 lines.
#   5. Every file of the skill tree exists; the 9 references/ files carry the
#      SPDX lines and a Status: line.
#   6. SKILL.md carries the SPDX lines right after the frontmatter.
#   7. Relative links in SKILL.md and references/ stay inside the skill and
#      point to existing files and headings.
#   8. The 16 scopes of Critical Rule 4 equal the CM-3 table of commits.md.
#   9. Inside a repository checkout, the generated copy under
#      .claude/skills/orcon-contributing is identical.
#  10. The Agent Skills reference validator, when skills-ref is on PATH.

set -u

skill_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
skill_md="$skill_dir/SKILL.md"
skill_name=orcon-contributing
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
references="branches commits pull-requests review releases community-docs provenance sdd-and-prs github-settings"

missing=0
for f in \
  SKILL.md \
  scripts/project.sh \
  scripts/check-frontmatter.sh \
  scripts/verify.sh \
  scripts/pre-pr-check.sh \
  scripts/compose-pr-body.sh \
  scripts/check-github-settings.sh \
  assets/pr-contributing-checklist.md \
  assets/github-settings.expected.json \
  assets/templates/commit-message.txt \
  assets/templates/size-justification.md \
  assets/templates/release-pr-review.md \
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
  echo "$skill_name: SKILL.md missing, cannot continue"
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
if [ "$rows" -ge 12 ]; then
  pass "checks table: $rows rows"
else
  fail "checks table: $rows rows (expected at least 12)"
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

# ---------------------------------------------------------------- 7. links
# slugs <file>: GitHub-style anchors of the file's headings, one per line.
slugs() {
  awk '/^```/ { fence = !fence; next } !fence && /^#+ / { sub(/^#+ /, ""); print }' "$1" |
    tr '[:upper:]' '[:lower:]' |
    sed -e 's/[^a-z0-9 _-]//g' -e 's/ /-/g'
}

link_errors=0
for f in "$skill_md" "$skill_dir"/references/*.md; do
  [ -f "$f" ] || continue
  dir=$(dirname -- "$f")
  rel=${f#"$skill_dir"/}
  links=$(grep -o '\](\([^)]*\))' "$f" | sed -e 's/^](//' -e 's/)$//' || true)
  for link in $links; do
    case "$link" in
    http://* | https://* | mailto:*) continue ;;
    esac
    path=${link%%#*}
    anchor=
    case "$link" in
    *'#'*) anchor=${link#*#} ;;
    esac
    if [ -z "$path" ]; then
      target=$f
    else
      target="$dir/$path"
    fi
    if [ ! -e "$target" ]; then
      fail "links: $rel -> $link: target does not exist"
      link_errors=1
      continue
    fi
    abs=$(CDPATH='' cd -- "$(dirname -- "$target")" && pwd)/$(basename -- "$target")
    case "$abs" in
    "$skill_dir"/*) ;;
    *)
      fail "links: $rel -> $link: leaves the skill directory"
      link_errors=1
      continue
      ;;
    esac
    if [ -n "$anchor" ] && ! slugs "$target" | grep -qxF "$anchor"; then
      fail "links: $rel -> $link: no heading with that anchor"
      link_errors=1
    fi
  done
done
[ "$link_errors" -eq 0 ] && pass "links: every relative link resolves inside the skill"

# ---------------------------------------------------------------- 8. scopes
rule_scopes=$(awk '/^4\. Scopes are only / { sub(/^4\. Scopes are only /, ""); sub(/;.*/, ""); print; exit }' "$skill_md" |
  tr ',' '\n' | sed 's/[[:space:]]//g' | grep -v '^$' | sort)
table_scopes=$(awk '/CM-3/ { incm = 1 } /CM-4/ { incm = 0 } incm' "$skill_dir/references/commits.md" |
  sed -n 's/^[[:space:]]*| `\([a-z]*\)` |.*/\1/p' | sort)
if [ -n "$rule_scopes" ] && [ "$rule_scopes" = "$table_scopes" ] && [ "$(printf '%s\n' "$rule_scopes" | wc -l | tr -d ' ')" -eq 16 ]; then
  pass "scopes: Critical Rule 4 equals the CM-3 table (16 scopes)"
else
  fail "scopes: Critical Rule 4 [$(printf '%s' "$rule_scopes" | tr '\n' ' ')] differs from the CM-3 table [$(printf '%s' "$table_scopes" | tr '\n' ' ')]"
fi

# ---------------------------------------------------------------- 9. generated copy
repo_root=$(git -C "$skill_dir" rev-parse --show-toplevel 2>/dev/null || true)
if [ -n "$repo_root" ] && [ "$skill_dir" = "$repo_root/.agents/skills/$skill_name" ]; then
  copy="$repo_root/.claude/skills/$skill_name"
  if [ ! -d "$copy" ]; then
    fail "copy: $copy missing; run scripts/project.sh"
  elif diff -r "$skill_dir" "$copy" >/dev/null; then
    pass "copy: generated copy is identical"
  else
    fail "copy: $copy differs from the canonical skill; run scripts/project.sh"
  fi
else
  pass "copy: not the canonical checkout, skipped"
fi

# ---------------------------------------------------------------- 10. reference validator
if command -v skills-ref >/dev/null 2>&1; then
  if skills-ref validate "$skill_dir"; then
    pass "skills-ref"
  else
    fail "skills-ref validate"
  fi
fi

if [ "$errors" -gt 0 ]; then
  echo "$skill_name: validation failed ($errors check(s))"
  exit 1
fi
echo "$skill_name: validation passed"
