// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest_test

import (
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// TestMemoryPassesTheBattery keeps the in-memory store honest: it must give
// the answers the SQLite and PostgreSQL adapters give.
func TestMemoryPassesTheBattery(t *testing.T) {
	t.Parallel()
	triggertest.Battery(t, func(*testing.T) triggertest.Backend { return triggertest.NewMemory().Backend() })
}

func TestMemoryHotSwitch(t *testing.T) {
	t.Parallel()
	load := triggertest.Load{Rate: 400, Duration: 400 * time.Millisecond}
	triggertest.HotSwitchUnderLoad(t, func(*testing.T) triggertest.Backend { return triggertest.NewMemory().Backend() }, load)
}
