// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"path/filepath"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
)

// The cost battery (cost_battery_test.go) runs the budget, rollup and
// alert rules of SPEC-006 AI2 (criterion E-COST-2) against the stores the -store
// flag selects.

func sqliteCostBackend(t *testing.T) costBackend {
	t.Helper()
	s := storetest.OpenSQLite(t, filepath.Join(t.TempDir(), "orcon.db"))
	return costBackend{Cost: s, Orgs: s, Scope: storetest.ScopeOf}
}

func postgresCostBackend(t *testing.T) costBackend {
	t.Helper()
	s := storetest.OpenPostgres(t, newPostgresDatabase(t))
	return costBackend{Cost: s, Orgs: s, Scope: storetest.ScopeOf}
}

// onCostStores runs one scenario once per store the -store flag selects.
func onCostStores(t *testing.T, scenario func(*testing.T, costFactory)) {
	t.Helper()
	ran := 0
	if wantSQLite() {
		ran++
		t.Run("sqlite", func(t *testing.T) { scenario(t, sqliteCostBackend) })
	}
	if wantPostgres() {
		ran++
		t.Run("postgres", func(t *testing.T) { scenario(t, postgresCostBackend) })
	}
	if ran == 0 {
		t.Fatal("no store selected")
	}
}

func TestBudgetsStrictestWins(t *testing.T) {
	onCostStores(t, costBudgetsStrictestWins)
}

func TestBudgetFailAndSuspend(t *testing.T) {
	onCostStores(t, costBudgetFailAndSuspend)
}

func TestRollupsInSameTransaction(t *testing.T) {
	onCostStores(t, costRollupsInSameTransaction)
}

func TestBudgetAlertOncePerPeriod(t *testing.T) {
	onCostStores(t, costBudgetAlertOncePerPeriod)
}
