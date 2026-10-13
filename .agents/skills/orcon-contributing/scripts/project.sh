#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Copies the canonical skill (.agents/skills/orcon-contributing) byte for byte to
# .claude/skills/orcon-contributing for clients that read that directory.
#
# Usage: sh .agents/skills/orcon-contributing/scripts/project.sh [--home]
#   --home  also copies the skill to $HOME/.claude/skills/orcon-contributing and
#           $HOME/.agents/skills/orcon-contributing, where `mneme skills lint` and
#           `mneme skills validate` look for skills.
#
# Never edit the copies; edit .agents/skills/orcon-contributing and run this script.

set -eu

skill_name=orcon-contributing
home_copy=0

for arg in "$@"; do
  case "$arg" in
  --home) home_copy=1 ;;
  -h | --help)
    sed -n '5,13p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  *)
    echo "project.sh: unknown argument: $arg" >&2
    exit 2
    ;;
  esac
done

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "project.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}
canonical="$repo_root/.agents/skills/$skill_name"

if [ ! -f "$canonical/SKILL.md" ]; then
  echo "project.sh: canonical skill not found at $canonical" >&2
  exit 1
fi

# copy_to <parent-dir>: replaces <parent-dir>/orcon-contributing with an exact copy.
copy_to() {
  parent=$1
  target="$parent/$skill_name"
  if [ "$target" = "$canonical" ]; then
    echo "project.sh: refusing to overwrite the canonical skill" >&2
    exit 1
  fi
  mkdir -p "$parent"
  rm -rf "$target"
  mkdir "$target"
  # Copy contents (including dot files) preserving modes.
  (cd "$canonical" && tar -cf - .) | (cd "$target" && tar -xpf -)
  echo "projected $canonical -> $target"
}

copy_to "$repo_root/.claude/skills"

if [ "$home_copy" -eq 1 ]; then
  : "${HOME:?HOME is not set}"
  copy_to "$HOME/.claude/skills"
  copy_to "$HOME/.agents/skills"
fi

if ! diff -r "$canonical" "$repo_root/.claude/skills/$skill_name" >/dev/null; then
  echo "project.sh: projection differs from the canonical skill" >&2
  exit 1
fi
