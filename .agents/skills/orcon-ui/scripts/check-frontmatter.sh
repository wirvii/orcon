#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Offline check of a skill's SKILL.md frontmatter against the Agent Skills
# standard: allowed top-level keys, required keys, name rules and length
# limits. No network access.
#
# Usage: sh check-frontmatter.sh [skill-dir]
# Default skill-dir: the skill that contains this script.

set -u

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
skill_dir=${1:-"$script_dir/.."}
skill_dir=$(CDPATH='' cd -- "$skill_dir" 2>/dev/null && pwd) || {
	echo "FAIL frontmatter: skill directory not found: ${1:-}" >&2
	exit 1
}
skill_md="$skill_dir/SKILL.md"
dir_name=$(basename -- "$skill_dir")
errors=0

fail() {
	echo "FAIL frontmatter: $*" >&2
	errors=$((errors + 1))
}

if [ ! -f "$skill_md" ]; then
	fail "SKILL.md not found in $skill_dir"
	exit 1
fi

if [ "$(sed -n '1p' "$skill_md")" != "---" ]; then
	fail "SKILL.md must start with a '---' line"
	exit 1
fi

# Frontmatter body: lines between the first and the second '---'.
fm=$(awk 'NR == 1 { next } /^---$/ { exit } { print }' "$skill_md")
closing=$(awk 'NR > 1 && /^---$/ { print NR; exit }' "$skill_md")
if [ -z "$closing" ]; then
	fail "missing closing '---' line"
	exit 1
fi

# Top-level keys: lines that start with a key and a colon, without indentation.
keys=$(printf '%s\n' "$fm" | sed -n 's/^\([A-Za-z0-9_-][A-Za-z0-9_-]*\):.*/\1/p')

allowed=" name description license allowed-tools metadata compatibility "
for k in $keys; do
	case "$allowed" in
	*" $k "*) ;;
	*) fail "unexpected top-level key '$k' (allowed:$allowed)" ;;
	esac
done

dups=$(printf '%s\n' "$keys" | sort | uniq -d)
if [ -n "$dups" ]; then
	fail "duplicate top-level keys: $(printf '%s' "$dups" | tr '\n' ' ')"
fi

# Value of a single-line top-level key (surrounding quotes removed).
value_of() {
	printf '%s\n' "$fm" | sed -n "s/^$1:[[:space:]]*//p" | sed -n '1p' |
		sed -e 's/[[:space:]]*$//' -e 's/^"\(.*\)"$/\1/' -e "s/^'\\(.*\\)'\$/\\1/"
}

# Character count in the current locale (a byte count in the C locale, which
# is stricter for non-ASCII text).
char_count() {
	printf '%s' "$1" | wc -m | tr -d ' '
}

name=$(value_of name)
description=$(value_of description)
compatibility=$(value_of compatibility)

# name rules
if [ -z "$name" ]; then
	fail "name is required"
else
	if [ "$(char_count "$name")" -gt 64 ]; then
		fail "name is longer than 64 characters"
	fi
	if ! printf '%s' "$name" | grep -Eq '^[a-z0-9]+(-[a-z0-9]+)*$'; then
		fail "name '$name' must be lowercase letters, digits and single hyphens, without leading or trailing hyphen"
	fi
	if [ "$name" != "$dir_name" ]; then
		fail "name '$name' does not match directory name '$dir_name'"
	fi
fi

# description rules
case "$description" in
'' | '>'* | '|'*)
	fail "description is required and must be a single-line value"
	;;
*)
	if [ "$(char_count "$description")" -gt 1024 ]; then
		fail "description is longer than 1024 characters"
	fi
	;;
esac

# compatibility rules (optional)
if [ -n "$compatibility" ] && [ "$(char_count "$compatibility")" -gt 500 ]; then
	fail "compatibility is longer than 500 characters"
fi

if [ "$errors" -gt 0 ]; then
	exit 1
fi
echo "PASS frontmatter: $skill_md"
