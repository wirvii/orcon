// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package journal

import (
	"testing"

	"go.uber.org/goleak"
)

// The journal starts goroutines (the group commit and the recovery of pending
// attempts): none may outlive the tests of this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
