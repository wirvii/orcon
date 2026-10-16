// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"testing"

	"go.uber.org/goleak"
)

// The agent runs its tool calls in parallel goroutines: none may outlive the tests of this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
