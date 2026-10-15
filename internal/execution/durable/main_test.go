// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"testing"

	"go.uber.org/goleak"
)

// The engine starts goroutines (the loops of Run, the dedup of the plan loader):
// none may outlive the tests of this package. The tests that run the engine over
// the real stores are in test/engine/durable and check the same there.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
