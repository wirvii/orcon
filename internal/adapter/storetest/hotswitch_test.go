// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"flag"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

var (
	rateFlag     = flag.Int("fault.rate", 500, "acceptance reads per second of the hot-switch load")
	durationFlag = flag.Duration("fault.duration", 0, "how long the hot-switch load runs, the publication halfway; 60s when -store names one store, 5s otherwise")
)

// hotSwitchLoad decides the pressure of the hot-switch test. Naming one store
// on the command line (the way criterion E-ACT-2 does) asks for the full
// minute; a general run of the suite only smoke-tests the switch for a few
// seconds, so the whole suite stays fast on both databases.
func hotSwitchLoad() triggertest.Load {
	d := *durationFlag
	if d == 0 {
		d = 5 * time.Second
		if storeFlag() == "sqlite" || storeFlag() == "postgres" {
			d = time.Minute
		}
	}
	return triggertest.Load{Rate: *rateFlag, Duration: d}
}

// TestHotSwitchUnderLoad publishes a new version of an active flow while
// acceptance reads run at 500 per second, on each selected store (E-ACT-2).
// The criterion's HTTP-level load (responses, accepted events, executions)
// belongs to the ingress cut; this proves the part the registry owns.
func TestHotSwitchUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("the hot-switch load runs for seconds; omit -short to run it")
	}
	load := hotSwitchLoad()
	onSelectedStores(t, func(t *testing.T, f triggertest.Factory) {
		t.Helper()
		triggertest.HotSwitchUnderLoad(t, f, load)
	})
}
