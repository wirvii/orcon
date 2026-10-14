#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# make test-fault STORE=sqlite|postgres [FI=FI-04,FI-12]: the fault matrix of
# ADR-0019 section 3 for the durable engine (SPEC-001). Rows that need a process of
# their own (kill -9, SIGSTOP, a full disk) run on SQLite in this cut; the rows
# that run inside the test process run on either store. The PostgreSQL process rows
# (FI-07, FI-08 and the multi-process races) arrive with cut C4b.
set -eu

store=${STORE:-sqlite}
rows=${FI:-}
stop_seconds=${ORCON_FAULT_STOP_SECONDS:-60}
status=0

case "$store" in
  sqlite | postgres) ;;
  *)
    echo "test-fault: STORE must be sqlite or postgres, got '$store'" >&2
    exit 2
    ;;
esac

# Rows, by id: the package, the tests, and whether the row needs a process (SQLite).
#   fault:   internal/adapter/storetest/fault (real processes)
#   durable: internal/execution/durable (the engine over the store, in the test process)
all_rows="FI-04 FI-05 FI-09 FI-10 FI-11 FI-12 FI-15 FI-16"
if [ -z "$rows" ]; then
  rows=$(echo "$all_rows" | tr ' ' ',')
fi

run() {
  # run <package> <test regexp>
  echo "test-fault: $store: $1 -run '$2'"
  if [ "$store" = postgres ] && [ -z "${ORCON_TEST_POSTGRES_DSN:-}" ]; then
    export ORCON_TEST_POSTGRES=1
  fi
  if ! ORCON_FAULT_STOP_SECONDS="$stop_seconds" go test "$1" -count=1 -timeout 60m -run "$2" -args -store="$store"; then
    status=1
  fi
}

for row in $(echo "$rows" | tr ',' ' '); do
  case "$row" in
    FI-04) [ "$store" = sqlite ] && run ./internal/adapter/storetest/fault/ 'TestKillDuringForkAndJoin' ;;
    FI-05) [ "$store" = sqlite ] && run ./internal/adapter/storetest/fault/ 'TestStoppedWorkerIsFenced' ;;
    FI-09) run ./internal/execution/durable/ 'TestFencing' ;;
    FI-10) run ./internal/execution/durable/ 'TestClockSkew' ;;
    FI-11) run ./internal/execution/durable/ 'TestClaimIsExclusive|TestOutbox' ;;
    FI-12)
      [ "$store" = sqlite ] && run ./internal/adapter/storetest/fault/ 'TestPoisonItem'
      run ./internal/execution/durable/ 'TestReaper|TestGracefulReleaseCountsAbandonment'
      ;;
    FI-15) [ "$store" = sqlite ] && run ./internal/adapter/storetest/fault/ 'TestKillDuringWrites|TestDiskFull' ;;
    FI-16) run ./internal/execution/durable/ 'TestRecomputeDeterminism' ;;
    FI-01 | FI-02 | FI-03 | FI-06)
      echo "test-fault: $row is a row of the effects journal (cut C6); it is not implemented yet" >&2
      status=1
      ;;
    FI-07 | FI-08)
      echo "test-fault: $row needs PostgreSQL processes (cut C4b); it is not implemented yet" >&2
      status=1
      ;;
    FI-13 | FI-14)
      echo "test-fault: $row is the isolation row of cut C12 and the expression cut C5; it is not implemented yet" >&2
      status=1
      ;;
    FI-17 | FI-18)
      echo "test-fault: $row is a row of the attention notifications (cut C17a); it is not implemented yet" >&2
      status=1
      ;;
    *)
      echo "test-fault: unknown fault id '$row'" >&2
      status=2
      ;;
  esac
done

if [ "$status" -eq 0 ]; then
  echo "test-fault: ok ($store)"
fi
exit "$status"
