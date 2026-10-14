// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger_test

import (
	"flag"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// storeFlag accepts the flag the criteria pass to every package they name. The
// area cannot import a database adapter (architecture-layers.md AL-2), so the
// tests of this package always run on the in-memory store; the same scenarios
// run against SQLite and PostgreSQL in internal/adapter/storetest, which acts
// on the flag.
var storeFlag = flag.String("store", "auto", "accepted for the criteria; this package always uses the in-memory store, the adapters run the same scenarios in storetest")

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func memory(t *testing.T) triggertest.Backend {
	t.Helper()
	switch *storeFlag {
	case "auto", "sqlite", "postgres", "both":
	default:
		t.Fatalf("unknown -store=%s (want auto, sqlite, postgres or both)", *storeFlag)
	}
	return triggertest.NewMemory().Backend()
}

func TestPublishActivateDeactivateRollback(t *testing.T) {
	t.Parallel()
	triggertest.PublishActivateDeactivateRollback(t, memory)
}

func TestUnpublishedChanges(t *testing.T) {
	t.Parallel()
	triggertest.UnpublishedChanges(t, memory)
}

func TestTriggerValidation(t *testing.T) {
	t.Parallel()
	triggertest.TriggerValidation(t, memory)
}

func TestConfigurationHashKeepsState(t *testing.T) {
	t.Parallel()
	triggertest.ConfigurationHashKeepsState(t, memory)
}

func TestDeletedFlowDeactivatesTriggers(t *testing.T) {
	t.Parallel()
	triggertest.DeletedFlowDeactivatesTriggers(t, memory)
}

func TestRestoredFlowStaysInactive(t *testing.T) {
	t.Parallel()
	triggertest.RestoredFlowStaysInactive(t, memory)
}

func TestPurgedFlowRemovesTriggers(t *testing.T) {
	t.Parallel()
	triggertest.PurgedFlowRemovesTriggers(t, memory)
}

func TestManualRun(t *testing.T) {
	t.Parallel()
	triggertest.ManualRun(t, memory)
}

func TestIsolation(t *testing.T) {
	t.Parallel()
	triggertest.Isolation(t, memory)
}

func TestConcurrentPublish(t *testing.T) {
	t.Parallel()
	triggertest.ConcurrentPublish(t, memory)
}
