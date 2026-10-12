#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Full local UI gate for Orcon. Runs, in order and stopping at the first
# failure: lint, typecheck, the @orcon/tokens tests, the i18n check, and the
# @orcon/ui unit, story, accessibility, target and visual tests (visual tests
# run in the pinned container).
#
# Usage: sh .agents/skills/orcon-ui/scripts/verify.sh
#
# Exit status: 0 when every step passes; 1 on the first failing or missing step.

set -u

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
	echo "verify.sh: must run from inside an Orcon repository checkout" >&2
	exit 1
}
cd "$repo_root" || exit 1

if ! command -v pnpm >/dev/null 2>&1; then
	echo "verify.sh: pnpm is not installed (pnpm 9 and Node 22 are required)" >&2
	exit 1
fi

# has_script <package-dir> <script-name>: true when package.json defines it.
has_script() {
	[ -f "$1/package.json" ] && grep -Eq "\"$2\"[[:space:]]*:" "$1/package.json"
}

# step <label> <package-dir> <script-name> <command...>
step() {
	label=$1
	pkg_dir=$2
	script=$3
	shift 3
	if ! has_script "$pkg_dir" "$script"; then
		echo "FAIL $label: script '$script' is not defined in $pkg_dir/package.json yet" >&2
		exit 1
	fi
	echo "==> $label: $*"
	if ! "$@"; then
		echo "FAIL $label" >&2
		exit 1
	fi
	echo "PASS $label"
}

ui=packages/js/ui
tokens=packages/js/tokens

step "lint" . lint pnpm lint
step "typecheck" . typecheck pnpm typecheck
step "tokens test" "$tokens" test pnpm --filter @orcon/tokens test
step "i18n check" . i18n:check pnpm i18n:check
step "unit tests" "$ui" test:unit pnpm --filter @orcon/ui test:unit
step "story coverage" "$ui" test:stories pnpm --filter @orcon/ui test:stories
step "accessibility" "$ui" test:a11y pnpm --filter @orcon/ui test:a11y
step "targets and density" "$ui" test:targets pnpm --filter @orcon/ui test:targets
step "visual regression" "$ui" test:visual:docker pnpm --filter @orcon/ui test:visual:docker

echo "verify.sh: all UI checks passed"
