// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest

import "flag"

// The criteria commands of SPEC-001 run `go test ./internal/execution/... -args -store=both`: the
// same arguments reach the test binary of every package under the tree, and a
// binary that does not know a flag fails whatever tests it holds. The flags are
// therefore defined here, in the package every test binary of the tree links, and
// the packages that use them read them through the functions below.
var (
	storeFlag       = flag.String("store", "", "stores the engine tests run against: memory, session, sqlite, postgres, both or auto (each package has its default)")
	iterationsFlag  = flag.Int("iterations", 0, "interrupted runs of TestRecomputeDeterminism (0: the default of the test)")
	joinRepeatsFlag = flag.Int("join-repeats", 0, "repetitions of the concurrent arrivals of TestJoinExactlyOneContinuation (0: the default of the test)")
)

// StoreFlag returns the value of -store, or def when it was not given.
func StoreFlag(def string) string {
	if *storeFlag == "" {
		return def
	}
	return *storeFlag
}

// Iterations returns the value of -iterations, or def when it was not given.
func Iterations(def int) int {
	if *iterationsFlag <= 0 {
		return def
	}
	return *iterationsFlag
}

// JoinRepeats returns the value of -join-repeats, or def when it was not given.
func JoinRepeats(def int) int {
	if *joinRepeatsFlag <= 0 {
		return def
	}
	return *joinRepeatsFlag
}
