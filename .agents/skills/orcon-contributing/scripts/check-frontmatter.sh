#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Offline check of the skill's SKILL.md frontmatter against the Agent Skills
# standard and the mneme template: allowed and required top-level keys, name
# rules, an unquoted semver metadata.version, metadata.pinned true and a
# single-line description of at most 500 characters containing "Use when".
# No network access; mneme is not needed.
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

closing=$(awk 'NR > 1 && /^---$/ { print NR; exit }' "$skill_md")
if [ -z "$closing" ]; then
  fail "missing closing '---' line"
  exit 1
fi

# Frontmatter body: lines between the first and the second '---'.
fm=$(awk 'NR == 1 { next } /^---$/ { exit } { print }' "$skill_md")

# Top-level keys: lines that start with a key and a colon, without indentation.
keys=$(printf '%s\n' "$fm" | sed -n 's/^\([A-Za-z0-9_-][A-Za-z0-9_-]*\):.*/\1/p')

allowed=" name description license compatibility metadata allowed-tools "
for k in $keys; do
  case "$allowed" in
  *" $k "*) ;;
  *) fail "unexpected top-level key '$k' (allowed:$allowed)" ;;
  esac
done

for k in name description license metadata; do
  if ! printf '%s\n' "$keys" | grep -qx "$k"; then
    fail "top-level key '$k' is required"
  fi
done

dups=$(printf '%s\n' "$keys" | sort | uniq -d)
if [ -n "$dups" ]; then
  fail "duplicate top-level keys: $(printf '%s' "$dups" | tr '\n' ' ')"
fi

# value_of <key>: raw value of a single-line top-level key.
value_of() {
  printf '%s\n' "$fm" | sed -n "s/^$1:[[:space:]]*//p" | sed -n '1p' | sed 's/[[:space:]]*$//'
}

# meta_value <key>: raw value of a key nested under metadata:.
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

# Character count in the current locale (a byte count in the C locale, which
# is stricter for non-ASCII text).
char_count() {
  printf '%s' "$1" | wc -m | tr -d ' '
}

name=$(value_of name)
description=$(value_of description)
license=$(value_of license)
compatibility=$(value_of compatibility)

# name rules
if [ -n "$name" ]; then
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

# description rules: one line, at most 500 characters, with "Use when".
case "$description" in
'' | '>'* | '|'* | '"'* | "'"*)
  fail "description must be an unquoted single-line value"
  ;;
*)
  if [ "$(char_count "$description")" -gt 500 ]; then
    fail "description is longer than 500 characters"
  fi
  case "$description" in
  *"Use when"*) ;;
  *) fail "description must contain 'Use when'" ;;
  esac
  ;;
esac

if printf '%s\n' "$keys" | grep -qx license && [ -z "$license" ]; then
  fail "license must have a value"
fi

# compatibility rules (optional)
if [ -n "$compatibility" ] && [ "$(char_count "$compatibility")" -gt 500 ]; then
  fail "compatibility is longer than 500 characters"
fi

# metadata rules
if printf '%s\n' "$keys" | grep -qx metadata; then
  if ! printf '%s\n' "$fm" | grep -q '^metadata:[[:space:]]*$'; then
    fail "metadata must be a block map"
  fi
  version=$(meta_value version)
  if ! printf '%s' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
    fail "metadata.version must be unquoted semver (got '$version')"
  fi
  pinned=$(meta_value pinned)
  if [ "$pinned" != "true" ]; then
    fail "metadata.pinned must be true (got '$pinned')"
  fi
fi

if [ "$errors" -gt 0 ]; then
  exit 1
fi
echo "PASS frontmatter: $skill_md"
