#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make cover: every business area package (a package under internal/ other than
# internal/adapter and internal/platform) with statements must have at least
# the given statement coverage, counted from the package's own tests (go-style.md
# GS-T7, about 80 % in the domain). Packages without statements (types and
# interfaces only) have nothing to cover.
#
# One exception, named and bounded (GS-T7): a package whose real-store tests had
# to move under test/ (AL-2.4: an area never imports an adapter, not even in its
# tests) may count the coverage that ONE test package under test/ gives it,
# measured with -coverpkg. The figure of the single best test package is used,
# never a union of several, and both figures are printed. Such packages are listed
# below with the amendment of the spec that moved their tests; adding one needs a
# spec amendment.
#
# Usage: check-coverage.sh [minimum-percent]   (default 80)
set -eu

# package (relative to the repository)  <TAB>  amendment that moved its tests
moved='internal/execution/durable	SPEC-001 Enmienda 12 section 12-3'

min=${1:-80}
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM

packages=$(go list ./internal/... | grep -v '/internal/adapter\(/\|$\)' | grep -v '/internal/platform\(/\|$\)' || true)
if [ -z "$packages" ]; then
  echo "cover: no area packages"
  exit 0
fi

# shellcheck disable=SC2086 # the package list is word-split on purpose
if ! go test -count=1 -cover $packages >"$work/out.txt" 2>&1; then
  cat "$work/out.txt" >&2
  echo "cover: tests failed" >&2
  exit 1
fi

module=$(go list -m)

# is_moved prints the amendment of a listed package and fails for any other.
is_moved() {
  printf '%s\n' "$moved" | while IFS='	' read -r rel why; do
    [ "$module/$rel" = "$1" ] && printf '%s\n' "$why"
  done | head -n 1 | grep .
}

# integration runs the packages under test/ once each, with -coverpkg of every
# listed package, and leaves one coverage profile per test package.
integration_done=0
integration() {
  [ "$integration_done" -eq 0 ] || return 0
  integration_done=1
  listed=$(printf '%s\n' "$moved" | while IFS='	' read -r rel _; do printf '%s/%s,' "$module" "$rel"; done | sed 's/,$//')
  n=0
  for tp in $(go list ./test/...); do
    n=$((n + 1))
    if ! go test -count=1 -short -coverpkg="$listed" -coverprofile="$work/prof-$n.out" "$tp" >"$work/it-$n.txt" 2>&1; then
      cat "$work/it-$n.txt" >&2
      echo "cover: the tests of $tp failed" >&2
      exit 1
    fi
    echo "$tp" >"$work/prof-$n.name"
  done
}

# percent_of prints the statement coverage of one package in a profile.
percent_of() {
  awk -v p="$2/" 'NR > 1 {
    split($1, a, ":"); f = a[1]
    if (index(f, p) == 1 && index(substr(f, length(p) + 1), "/") == 0) { t += $2; if ($3 > 0) c += $2 }
  } END { if (t > 0) printf "%.1f", 100 * c / t }' "$1"
}

status=0
while IFS= read -r line; do
  case "$line" in
    *"[no test files]"*) continue ;; # no statements: types and interfaces only
  esac
  pkg=$(printf '%s\n' "$line" | awk '{ print ($1 == "ok" || $1 == "FAIL") ? $2 : $1 }')
  pct=$(printf '%s\n' "$line" | sed -n 's/.*coverage: \([0-9.]*\)% of statements.*/\1/p')
  if [ -z "$pct" ]; then
    continue
  fi
  if ! awk -v got="$pct" -v want="$min" 'BEGIN { exit !(got + 0 < want + 0) }'; then
    echo "cover: $pkg $pct%"
    continue
  fi
  if why=$(is_moved "$pkg"); then
    integration
    best=0
    best_name=""
    for prof in "$work"/prof-*.out; do
      [ -e "$prof" ] || continue
      got=$(percent_of "$prof" "$pkg")
      [ -n "$got" ] || continue
      if awk -v got="$got" -v best="$best" 'BEGIN { exit !(got + 0 > best + 0) }'; then
        best=$got
        best_name=$(cat "${prof%.out}.name")
      fi
    done
    if [ -n "$best_name" ] && awk -v got="$best" -v want="$min" 'BEGIN { exit !(got + 0 >= want + 0) }'; then
      echo "cover: $pkg $pct% own, $best% via ${best_name#"$module"/} ($why)"
      continue
    fi
    echo "cover: $pkg has $pct% of its own and $best% via the best test package, need at least $min% ($why)" >&2
    status=1
    continue
  fi
  echo "cover: $pkg has $pct% coverage, need at least $min%" >&2
  status=1
done <"$work/out.txt"

if [ "$status" -eq 0 ]; then
  echo "cover: ok (minimum $min%)"
fi
exit "$status"
