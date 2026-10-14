# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Targets of the execution engine (SPEC-001, mk/README.md). None of them joins `check`.

.PHONY: test-fault test-engine

# test-fault runs the fault matrix of ADR-0019 section 3 for the durable engine:
# real processes killed with SIGKILL, stopped with SIGSTOP, or run with a limit on
# the files they write. STORE selects the store (sqlite, or postgres for the rows
# that run inside the test process); FI limits the run to some rows, for example
# `make test-fault STORE=sqlite FI=FI-04,FI-12`. See scripts/test-fault.sh for the
# rows of each cut.
STORE ?= sqlite
FI ?=
test-fault:
	STORE=$(STORE) FI=$(FI) sh scripts/test-fault.sh

# test-engine runs the criteria commands of the durable engine on both stores, the
# way E-REC-1, E-REC-2, E-REC-4 and E-FMT-6 of SPEC-001 state them.
test-engine:
	go test ./internal/execution/... -count=1 -run 'TestFencing|TestLeaseExpiry|TestReaper' -args -store=both
	go test ./internal/execution/... -count=1 -run 'TestAtomicCheckpoint|TestForkAtomic|TestJoinExactlyOneContinuation' -args -store=both
	go test ./internal/execution/... -count=1 -run 'TestRecomputeDeterminism' -args -iterations=500
	go test ./internal/execution/... -count=1 -run 'TestUpgradedDocumentCache'
