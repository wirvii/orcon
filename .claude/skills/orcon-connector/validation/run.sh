#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Validation script of the orcon-connector skill (mneme `skills validate` runs
# it from the skill directory; make skill-check runs it from the repository
# root). POSIX sh, deterministic, no network access. Exit status 0 means valid.
#
# Checks:
#   1. Frontmatter (scripts/check-frontmatter.sh): allowed and required keys,
#      name rules, unquoted semver metadata.version, metadata.pinned true,
#      single-line description of at most 500 characters with "Use when".
#   2. Each required H2 section exactly once and no other H2.
#   3. Automated Checks table: exact 3-column header, at least 17 rows.
#   4. SKILL.md has at most 300 lines.
#   5. The 14 references/ files exist, each with SPDX lines and a Status:
#      line; the scripts, the PR checklist and the four template folders exist.
#   6. SKILL.md carries the SPDX lines right after the frontmatter.
#   7. naming.md lists exactly the 17 vocabulary verbs and errors.md exactly
#      the 14 error codes.
#   8. SKILL.md, references/ and assets/ pass the repository provenance check
#      (no build-assistant names, no AI authorship marks). Needs the provenance
#      tool built by `make tools`; skipped with a notice outside a checkout.
#   9. Links to sibling skills (orcon-go, orcon-ui) resolve when the sibling
#      skill folder exists.

set -u

skill_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
skill_md="$skill_dir/SKILL.md"
skills_root=$(dirname -- "$skill_dir")
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
references="kinds naming manifest credentials-and-egress operation-safety pagination-and-rate-limits errors testing versioning documentation icon n8n-mapping catalog-admission checks"

missing=0
for f in \
  SKILL.md \
  scripts/project.sh \
  scripts/check-frontmatter.sh \
  scripts/connector-change-check.sh \
  scripts/verify.sh \
  assets/pr-connector-checklist.md \
  validation/run.sh; do
  if [ ! -f "$skill_dir/$f" ]; then
    fail "tree: missing $f"
    missing=1
  fi
done
for d in declarative native cassette-test connection-guide; do
  if [ ! -d "$skill_dir/assets/templates/$d" ]; then
    fail "tree: missing assets/templates/$d/"
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
  if [ "$(grep -c '^Status: [^[:space:]]' "$f")" -ne 1 ]; then
    fail "references/$r.md: exactly one 'Status:' line is required"
  fi
done
[ "$missing" -eq 0 ] && pass "tree: all files present"

if [ ! -f "$skill_md" ]; then
  echo "orcon-connector: SKILL.md missing, cannot continue"
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
if [ "$rows" -ge 17 ]; then
  pass "checks table: $rows rows"
else
  fail "checks table: $rows rows (expected at least 17)"
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

# ---------------------------------------------------------------- 7. vocabulary and error codes
# table_ids <file>: the backquoted identifier in the first cell of every table row.
table_ids() {
  sed -n 's/^[[:space:]]*| `\([A-Za-z][A-Za-z]*\)` |.*/\1/p' "$1"
}
verbs="create get getMany update upsert delete send search add remove archive unarchive upload download copy move cancel"
codes="AuthInvalid AuthExpired PermissionDenied NotFound Conflict ValidationFailed RateLimited QuotaExceeded PayloadTooLarge ServiceUnavailable Timeout NetworkError Unsupported ProviderError"

# same_set <expected words> <found words>: 0 when both lists hold the same words once each.
same_set() {
  e=$(printf '%s\n' $1 | sort)
  f=$(printf '%s\n' $2 | sort)
  [ "$e" = "$f" ]
}

if [ -f "$skill_dir/references/naming.md" ]; then
  found=$(table_ids "$skill_dir/references/naming.md" | tr '\n' ' ')
  if same_set "$verbs" "$found"; then
    pass "naming.md: the 17 vocabulary verbs"
  else
    fail "naming.md: verb table differs from the 17 vocabulary verbs (found: $found)"
  fi
fi
if [ -f "$skill_dir/references/errors.md" ]; then
  found=$(table_ids "$skill_dir/references/errors.md" | tr '\n' ' ')
  if same_set "$codes" "$found"; then
    pass "errors.md: the 14 error codes"
  else
    fail "errors.md: code table differs from the 14 error codes (found: $found)"
  fi
fi

# ---------------------------------------------------------------- 8. provenance
repo_root=$(git -C "$skill_dir" rev-parse --show-toplevel 2>/dev/null || true)
provenance=""
if [ -n "$repo_root" ] && [ -x "$repo_root/.bin/provenance" ]; then
  provenance="$repo_root/.bin/provenance"
fi
if [ -z "$provenance" ]; then
  echo "SKIP provenance: run 'make tools' in an Orcon checkout to build .bin/provenance"
else
  # The skill folder is outside the repository-wide scan by design, so its
  # readable files are copied to a temporary folder and scanned there.
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  (cd "$skill_dir" && find SKILL.md references assets -type f | while IFS= read -r f; do
    mkdir -p "$tmp/$(dirname -- "$f")"
    cp "$f" "$tmp/$f"
  done)
  if out=$(cd "$tmp" && find . -type f | sed 's|^\./||' | sort | xargs "$provenance" files -allow /dev/null); then
    pass "provenance: SKILL.md, references/ and assets/"
  else
    fail "provenance: $(printf '%s' "$out" | tr '\n' ';')"
  fi
fi

# ---------------------------------------------------------------- 9. links to sibling skills
link_errors=0
check_link() {
  # check_link <file containing the link> <link target without anchor>
  base=$(dirname -- "$1")
  target="$base/$2"
  sibling=$(printf '%s' "$2" | sed -n 's|^\(\.\./\)*\(orcon-[a-z]*\)/.*|\2|p')
  if [ -n "$sibling" ] && [ ! -d "$skills_root/$sibling" ]; then
    return 0
  fi
  if [ ! -f "$target" ]; then
    fail "links: $2 (from $(basename -- "$1")) does not resolve"
    link_errors=1
  fi
}
for f in "$skill_md" "$skill_dir"/references/*.md; do
  [ -f "$f" ] || continue
  for l in $(grep -o '\(\.\./\)\{1,2\}orcon-[a-z]*/[A-Za-z0-9_./-]*\.md' "$f" | sort -u); do
    check_link "$f" "$l"
  done
done
[ "$link_errors" -eq 0 ] && pass "links: sibling skill links resolve (or the sibling is absent)"

if [ "$errors" -gt 0 ]; then
  echo "orcon-connector: validation failed ($errors check(s))"
  exit 1
fi
echo "orcon-connector: validation passed"
