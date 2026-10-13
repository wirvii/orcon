#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Validation script of the orcon-ui skill (mneme `skills validate` runs it
# from the skill directory; CI runs it from the repository root).
# Deterministic, no network access. Exit status 0 means the skill is valid.
#
# Checks:
#   1. Frontmatter: Agent Skills rules (scripts/check-frontmatter.sh), license,
#      metadata.version semver, metadata.pinned true, description <= 500
#      characters and containing "Use when".
#   2. Exactly the five required H2 sections, each once, and no other H2.
#   3. Automated Checks table header (3 columns) with at least 9 rows.
#   4. SKILL.md has at most 300 lines and no hex colors.
#   5. Every file of the skill tree exists.
#   6. Every rule document in references/ carries a status line.
#   7. Every relative link in SKILL.md and references/ resolves to a file of
#      the skill and, when it has one, to an existing heading anchor.
#   8. Hex colors appear only in foundations.md, tokens.md, canvas.md and
#      contrast-report.md.
#   9. Inside a repository checkout, .claude/skills/orcon-ui is identical to
#      .agents/skills/orcon-ui.
#  10. skills-ref validate, when skills-ref is installed.

set -u

skill_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
skill_name=orcon-ui
skill_md="$skill_dir/SKILL.md"
errors=0

fail() {
	echo "FAIL $*"
	errors=$((errors + 1))
}

pass() {
	echo "PASS $*"
}

hex_re='#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})([^0-9A-Za-z_-]|$)'

# ---------------------------------------------------------------- 5. tree
check_tree() {
	missing=0
	for f in \
		SKILL.md \
		references/foundations.md \
		references/tokens.md \
		references/components.md \
		references/canvas.md \
		references/content-and-i18n.md \
		references/accessibility.md \
		references/contrast-report.md \
		references/component-checklist.md \
		references/screen-checklist.md \
		references/review-protocol.md \
		scripts/verify.sh \
		scripts/project.sh \
		scripts/check-frontmatter.sh \
		assets/pr-ui-checklist.md \
		assets/review-record-template.md \
		validation/run.sh; do
		if [ ! -f "$skill_dir/$f" ]; then
			fail "tree: missing $f"
			missing=1
		fi
	done
	[ "$missing" -eq 0 ] && pass "tree: all files present"
}

check_tree

if [ ! -f "$skill_md" ]; then
	echo "validation: SKILL.md missing, cannot continue"
	exit 1
fi

# ---------------------------------------------------------------- 1. frontmatter
if sh "$skill_dir/scripts/check-frontmatter.sh" "$skill_dir"; then
	:
else
	fail "frontmatter: Agent Skills rules (see messages above)"
fi

fm=$(awk 'NR == 1 { next } /^---$/ { exit } { print }' "$skill_md")

if printf '%s\n' "$fm" | grep -Eq '^license:[[:space:]]*[^[:space:]]'; then
	pass "frontmatter: license present"
else
	fail "frontmatter: license is required"
fi

meta_value() {
	printf '%s\n' "$fm" | awk -v key="$1" '
		/^metadata:[[:space:]]*$/ { inmeta = 1; next }
		/^[^[:space:]]/ { inmeta = 0 }
		inmeta && $0 ~ "^[[:space:]]+" key ":" {
			sub("^[[:space:]]+" key ":[[:space:]]*", "")
			sub("[[:space:]]+$", "")
			print
			exit
		}'
}

if ! printf '%s\n' "$fm" | grep -q '^metadata:[[:space:]]*$'; then
	fail "frontmatter: metadata map is required"
fi

version=$(meta_value version)
if printf '%s' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
	pass "frontmatter: metadata.version $version"
else
	fail "frontmatter: metadata.version must be unquoted semver (got '$version')"
fi

pinned=$(meta_value pinned)
if [ "$pinned" = "true" ]; then
	pass "frontmatter: metadata.pinned true"
else
	fail "frontmatter: metadata.pinned must be true (got '$pinned')"
fi

description=$(printf '%s\n' "$fm" | sed -n 's/^description:[[:space:]]*//p' | sed -n '1p')
dlen=$(printf '%s' "$description" | wc -m | tr -d ' ')
if [ "$dlen" -le 500 ]; then
	pass "frontmatter: description length $dlen <= 500"
else
	fail "frontmatter: description length $dlen > 500"
fi
case "$description" in
*"Use when"*) pass "frontmatter: description contains 'Use when'" ;;
*) fail "frontmatter: description must contain 'Use when'" ;;
esac

# ---------------------------------------------------------------- 2. H2 sections
body=$(awk 'NR == 1 { next } !done && /^---$/ { done = 1; next } done { print }' "$skill_md")
h2=$(printf '%s\n' "$body" | grep '^## ' || true)
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
if [ "$rows" -ge 9 ]; then
	pass "checks table: $rows rows"
else
	fail "checks table: $rows rows (expected at least 9)"
fi
bad_cols=$(printf '%s\n' "$body" | awk -F'|' '
	/^## / { insec = ($0 == "## Automated Checks"); next }
	insec && /^\|/ && NF != 5 { print NR }')
if [ -n "$bad_cols" ]; then
	fail "checks table: rows without exactly 3 columns (body lines: $(printf '%s' "$bad_cols" | tr '\n' ' '))"
fi

# ---------------------------------------------------------------- 4. size and hex
lines=$(wc -l <"$skill_md" | tr -d ' ')
if [ "$lines" -le 300 ]; then
	pass "SKILL.md: $lines lines <= 300"
else
	fail "SKILL.md: $lines lines > 300"
fi
if grep -Eq "$hex_re" "$skill_md"; then
	fail "SKILL.md: hex color values found (values live in references/)"
else
	pass "SKILL.md: no hex colors"
fi

# ---------------------------------------------------------------- 6. status lines
for doc in foundations tokens components canvas content-and-i18n accessibility \
	component-checklist screen-checklist review-protocol; do
	f="$skill_dir/references/$doc.md"
	[ -f "$f" ] || continue
	if head -n 5 "$f" | grep -Eq '^Status: (Proposed — pending owner approval|Accepted)'; then
		pass "status: references/$doc.md"
	else
		fail "status: references/$doc.md has no 'Status: Proposed — pending owner approval' or 'Status: Accepted' line"
	fi
done

# ---------------------------------------------------------------- 7. links and anchors
# anchors <file>: GitHub-style heading slugs, one per line (code fences skipped).
anchors() {
	awk '/^```/ { fence = !fence; next } !fence && /^#{1,6} / { print }' "$1" |
		sed 's/^#\{1,6\}[[:space:]]*//' |
		tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz' |
		sed -e 's/[^a-z0-9 _-]//g' -e 's/ /-/g'
}

# links <file>: link targets of inline Markdown links (code fences skipped).
links() {
	awk '/^```/ { fence = !fence; next } !fence { print }' "$1" |
		grep -o '\]([^)[:space:]]*)' |
		sed -e 's/^](//' -e 's/)$//'
}

link_errors=0
for src in "$skill_md" "$skill_dir"/references/*.md; do
	src_dir=$(dirname -- "$src")
	src_rel=${src#"$skill_dir"/}
	for target in $(links "$src"); do
		case "$target" in
		http://* | https://* | mailto:*) continue ;;
		esac
		path=${target%%#*}
		anchor=
		case "$target" in
		*'#'*) anchor=${target#*#} ;;
		esac
		if [ -z "$path" ]; then
			file=$src
		else
			file="$src_dir/$path"
		fi
		if [ ! -f "$file" ]; then
			fail "links: $src_rel -> $target (file not found)"
			link_errors=1
			continue
		fi
		abs=$(CDPATH='' cd -- "$(dirname -- "$file")" && pwd)
		case "$abs/" in
		"$skill_dir"/*) ;;
		*)
			fail "links: $src_rel -> $target (points outside the skill)"
			link_errors=1
			continue
			;;
		esac
		if [ -n "$anchor" ] && ! anchors "$file" | grep -qx -- "$anchor"; then
			fail "links: $src_rel -> $target (anchor not found)"
			link_errors=1
		fi
	done
done
[ "$link_errors" -eq 0 ] && pass "links: all relative links and anchors resolve"

# ---------------------------------------------------------------- 8. hex homes
hex_errors=0
for f in "$skill_dir"/references/*.md; do
	case "$(basename -- "$f")" in
	foundations.md | tokens.md | canvas.md | contrast-report.md) continue ;;
	esac
	if grep -Eq "$hex_re" "$f"; then
		fail "hex: references/$(basename -- "$f") contains hex values (allowed only in foundations, tokens, canvas, contrast-report)"
		hex_errors=1
	fi
done
[ "$hex_errors" -eq 0 ] && pass "hex: values only in their home documents"

# ---------------------------------------------------------------- 9. projection
repo_root=$(git -C "$skill_dir" rev-parse --show-toplevel 2>/dev/null || true)
if [ -n "$repo_root" ] && { [ "$skill_dir" = "$repo_root/.agents/skills/$skill_name" ] || [ "$skill_dir" = "$repo_root/.claude/skills/$skill_name" ]; }; then
	canonical="$repo_root/.agents/skills/$skill_name"
	copy="$repo_root/.claude/skills/$skill_name"
	if [ ! -d "$copy" ]; then
		fail "projection: $copy missing (run scripts/project.sh)"
	elif diff -r "$canonical" "$copy" >/dev/null 2>&1; then
		pass "projection: .claude/skills/$skill_name identical to .agents/skills/$skill_name"
	else
		fail "projection: .claude/skills/$skill_name differs from .agents/skills/$skill_name (run scripts/project.sh)"
	fi
else
	echo "SKIP projection (not inside a repository checkout)"
fi

# ---------------------------------------------------------------- 10. skills-ref
if command -v skills-ref >/dev/null 2>&1; then
	if skills-ref validate "$skill_dir"; then
		pass "skills-ref validate"
	else
		fail "skills-ref validate"
	fi
else
	echo "SKIP skills-ref (not installed)"
fi

if [ "$errors" -gt 0 ]; then
	echo "validation: $errors check(s) failed"
	exit 1
fi
echo "validation: all checks passed"
