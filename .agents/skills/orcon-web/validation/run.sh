#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Validation script of the orcon-web skill (mneme `skills validate` runs it
# from the skill directory; make skill-check runs it from the repository root).
# POSIX sh, deterministic, no network access. Exit status 0 means valid.
#
# Checks:
#   1. Frontmatter (scripts/check-frontmatter.sh): allowed and required keys,
#      name rules, unquoted semver metadata.version, metadata.pinned true,
#      single-line description of at most 500 characters with "Use when".
#   2. Each required H2 section exactly once and no other H2.
#   3. Automated Checks table: exact 3-column header, at least 18 rows.
#   4. SKILL.md has at most 300 lines.
#   5. The 11 references/ files exist, each with SPDX lines and a Status: line,
#      and every file of the skill tree exists.
#   6. SKILL.md carries the SPDX lines right after the frontmatter.
#   7. No duplication of orcon-ui: no hexadecimal color values and no
#      --orcon- token names in SKILL.md or references/; SKILL.md,
#      architecture.md, forms.md and errors.md link to orcon-ui; when the
#      sibling orcon-ui skill exists, every link to it resolves to a heading.

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
references="architecture data-and-live-updates routing state forms errors typescript testing performance dependencies checks"

missing=0
for f in \
  SKILL.md \
  scripts/project.sh \
  scripts/check-frontmatter.sh \
  scripts/web-change-check.sh \
  scripts/verify.sh \
  assets/pr-web-checklist.md \
  assets/feature-template/route.tsx \
  assets/feature-template/queries.ts \
  assets/feature-template/mutations.ts \
  assets/feature-template/live.ts \
  assets/feature-template/form.tsx \
  assets/feature-template/screen.test.tsx \
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
  echo "orcon-web: SKILL.md missing, cannot continue"
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

# ---------------------------------------------------------------- 7. orcon-ui is linked, not repeated
dup_files="$skill_md"
for r in $references; do
  [ -f "$skill_dir/references/$r.md" ] && dup_files="$dup_files $skill_dir/references/$r.md"
done
# shellcheck disable=SC2086 # word splitting of the file list is intended
hex=$(sed 's/\](\([^)]*\))/]()/g' $dup_files | grep -nE '#[0-9A-Fa-f]{3}([0-9A-Fa-f]{3}([0-9A-Fa-f]{2})?)?([^0-9A-Za-z_-]|$)' || true)
if [ -n "$hex" ]; then
  fail "orcon-ui duplication: hexadecimal color values found: $(printf '%s' "$hex" | head -n 3 | tr '\n' ';')"
else
  pass "orcon-ui duplication: no hexadecimal color values"
fi
# shellcheck disable=SC2086
if grep -qF -- '--orcon-' $dup_files; then
  fail "orcon-ui duplication: '--orcon-' token names found"
else
  pass "orcon-ui duplication: no token names"
fi
if grep -qF '../orcon-ui/' "$skill_md"; then
  pass "orcon-ui link: SKILL.md"
else
  fail "orcon-ui link: SKILL.md must link to ../orcon-ui/"
fi
for r in architecture forms errors; do
  if grep -qF '../../orcon-ui/references/' "$skill_dir/references/$r.md" 2>/dev/null; then
    pass "orcon-ui link: references/$r.md"
  else
    fail "orcon-ui link: references/$r.md must link to ../../orcon-ui/references/"
  fi
done

# anchor_of <heading text>: GitHub-style anchor of a Markdown heading.
anchor_of() {
  printf '%s\n' "$1" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9 _-]//g; s/ /-/g'
}

ui_dir="$skill_dir/../orcon-ui"
if [ -d "$ui_dir" ]; then
  links=$(for r in $references; do
    f="$skill_dir/references/$r.md"
    [ -f "$f" ] && grep -oE '\(\.\./\.\./orcon-ui/references/[A-Za-z0-9_.-]+\.md(#[A-Za-z0-9_-]+)?\)' "$f"
  done | tr -d '()' | sort -u)
  broken=0
  for link in $links; do
    doc=${link#../../orcon-ui/references/}
    anchor=
    case "$doc" in
    *'#'*)
      anchor=${doc#*#}
      doc=${doc%%#*}
      ;;
    esac
    target="$ui_dir/references/$doc"
    if [ ! -f "$target" ]; then
      fail "orcon-ui link: $link (file not found)"
      broken=1
      continue
    fi
    if [ -n "$anchor" ]; then
      found=0
      while IFS= read -r heading; do
        if [ "$(anchor_of "$heading")" = "$anchor" ]; then
          found=1
          break
        fi
      done <<HEADINGS
$(sed -n 's/^#\{1,6\} \{1,\}//p' "$target")
HEADINGS
      if [ "$found" -eq 0 ]; then
        fail "orcon-ui link: $link (no such heading)"
        broken=1
      fi
    fi
  done
  [ "$broken" -eq 0 ] && pass "orcon-ui links resolve"
else
  pass "orcon-ui links: sibling skill not present, resolution skipped"
fi

if [ "$errors" -gt 0 ]; then
  echo "orcon-web: validation failed ($errors check(s))"
  exit 1
fi
echo "orcon-web: validation passed"
