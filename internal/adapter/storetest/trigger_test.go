// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"path/filepath"
	"testing"

	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// The trigger battery (internal/trigger/triggertest) runs the lifecycle of
// SPEC-002 cut T1 against the stores the -store flag selects. The same
// scenarios run on the in-memory store in internal/trigger, which proves the
// rules; here they prove that SQLite and PostgreSQL give the same answers.

func sqliteBackend(t *testing.T) triggertest.Backend {
	t.Helper()
	s := openSQLite(t, filepath.Join(t.TempDir(), "orcon.db"))
	return triggertest.Backend{Triggers: s, Flows: s}
}

func postgresBackend(t *testing.T) triggertest.Backend {
	t.Helper()
	s := openPostgres(t, newPostgresDatabase(t))
	return triggertest.Backend{Triggers: s, Flows: s}
}

// onSelectedStores runs one scenario once per store the -store flag selects.
func onSelectedStores(t *testing.T, scenario func(*testing.T, triggertest.Factory)) {
	t.Helper()
	ran := 0
	if wantSQLite() {
		ran++
		t.Run("sqlite", func(t *testing.T) { scenario(t, sqliteBackend) })
	}
	if wantPostgres() {
		ran++
		t.Run("postgres", func(t *testing.T) { scenario(t, postgresBackend) })
	}
	if ran == 0 {
		t.Fatal("no store selected")
	}
}

func TestPublishActivateDeactivateRollback(t *testing.T) {
	onSelectedStores(t, triggertest.PublishActivateDeactivateRollback)
}

func TestUnpublishedChanges(t *testing.T) { onSelectedStores(t, triggertest.UnpublishedChanges) }

func TestTriggerValidation(t *testing.T) { onSelectedStores(t, triggertest.TriggerValidation) }

func TestConfigurationHashKeepsState(t *testing.T) {
	onSelectedStores(t, triggertest.ConfigurationHashKeepsState)
}

func TestDeletedFlowDeactivatesTriggers(t *testing.T) {
	onSelectedStores(t, triggertest.DeletedFlowDeactivatesTriggers)
}

func TestRestoredFlowStaysInactive(t *testing.T) {
	onSelectedStores(t, triggertest.RestoredFlowStaysInactive)
}

func TestPurgedFlowRemovesTriggers(t *testing.T) {
	onSelectedStores(t, triggertest.PurgedFlowRemovesTriggers)
}

func TestManualRun(t *testing.T) { onSelectedStores(t, triggertest.ManualRun) }

func TestTriggerIsolation(t *testing.T) { onSelectedStores(t, triggertest.Isolation) }

func TestConcurrentPublish(t *testing.T) { onSelectedStores(t, triggertest.ConcurrentPublish) }
