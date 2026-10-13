#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make bench-compare (SPEC-008 CFG-7, performance.md PERF-2): runs the Go
# benchmarks of a base revision and of the working tree interleaved, BENCH_COUNT
# times each, compares them with benchstat and applies tools/benchgate.
#
# Environment:
#   BASE_REF         revision to compare against (default origin/main)
#   BENCH_COUNT      interleaved runs per side (default 10)
#   BENCH_TIME       -benchtime of each run (default 2s)
#   BENCH_ACCEPTED   set to 1 when the pull request has the
#                    performance-regression-accepted label
#   PR_BODY_FILE     file with the pull request description (justification)
#
# The base revision is exported with `git archive`; no worktree, branch or
# configuration of the repository is touched.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

base_ref=${BASE_REF:-origin/main}
count=${BENCH_COUNT:-10}
benchtime=${BENCH_TIME:-2s}

# Packages with benchmarks, as directories relative to the root.
pkgdirs=$(grep -rl --include='*_test.go' '^func Benchmark' . 2>/dev/null |
  grep -v '/node_modules/' | xargs -n 50 dirname 2>/dev/null | sed 's|^\./||' | sort -u || true)
if [ -z "$pkgdirs" ]; then
  echo "bench-compare: no benchmarks yet"
  exit 0
fi

if ! git rev-parse --verify --quiet "$base_ref^{commit}" >/dev/null; then
  echo "bench-compare: base revision $base_ref not found (fetch it or set BASE_REF)" >&2
  exit 2
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM
mkdir -p "$work/base"
git archive "$base_ref" | tar -x -C "$work/base"

i=0
runs=""
for rel in $pkgdirs; do
  i=$((i + 1))
  head_bin="$work/head-$i.test"
  base_bin="$work/base-$i.test"
  (cd "$root/$rel" && go test -c -o "$head_bin" .)
  # A package or benchmark new in this change has no base side: the package is
  # missing, has no test files (go test -c then writes no binary), or has no
  # benchmark.
  if [ -d "$work/base/$rel" ] && (cd "$work/base/$rel" && go test -c -o "$base_bin" . 2>/dev/null) &&
    [ -x "$base_bin" ] && "$base_bin" -test.list '^Benchmark' 2>/dev/null | grep -q '^Benchmark'; then
    runs="$runs $i:$rel"
  else
    echo "bench-compare: $rel has no base side; skipped in the comparison"
  fi
done
if [ -z "$runs" ]; then
  echo "bench-compare: every benchmark is new; nothing to compare"
  exit 0
fi

: >"$work/base.txt"
: >"$work/head.txt"
n=0
while [ "$n" -lt "$count" ]; do
  n=$((n + 1))
  for entry in $runs; do
    idx=${entry%%:*}
    rel=${entry#*:}
    echo "bench-compare: run $n/$count $rel"
    # Alternate which side goes first to cancel drift of the runner.
    if [ $((n % 2)) -eq 1 ]; then order="base head"; else order="head base"; fi
    for side in $order; do
      if [ "$side" = base ]; then dir="$work/base/$rel"; else dir="$root/$rel"; fi
      (cd "$dir" && GOMAXPROCS=4 "$work/$side-$idx.test" -test.run '^$' -test.bench . -test.benchmem -test.benchtime "$benchtime" -test.count 1) >>"$work/$side.txt"
    done
  done
done

GOWORK=off GOTOOLCHAIN=auto go tool -modfile="$root/tools/go.mod" benchstat -format csv "$work/base.txt" "$work/head.txt" >"$work/comparison.csv"
GOWORK=off GOTOOLCHAIN=auto go tool -modfile="$root/tools/go.mod" benchstat "$work/base.txt" "$work/head.txt" || true

set -- -csv "$work/comparison.csv"
if [ "${BENCH_ACCEPTED:-0}" = 1 ]; then
  set -- "$@" -label-accepted
fi
if [ -n "${PR_BODY_FILE:-}" ]; then
  set -- "$@" -pr-body-file "$PR_BODY_FILE"
fi
"$root/.bin/benchgate" "$@"
