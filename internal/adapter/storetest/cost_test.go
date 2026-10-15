// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"path/filepath"
	"testing"

	"github.com/wirvii/orcon/internal/ai/cost/costtest"
)

// The cost battery (internal/ai/cost/costtest) runs the budget, rollup and
// alert rules of SPEC-006 AI2 (criterion E-COST-2) against the stores the -store
// flag selects.

func sqliteCostBackend(t *testing.T) costtest.Backend {
	t.Helper()
	s := openSQLite(t, filepath.Join(t.TempDir(), "orcon.db"))
	return costtest.Backend{Cost: s, Orgs: s}
}

func postgresCostBackend(t *testing.T) costtest.Backend {
	t.Helper()
	s := openPostgres(t, newPostgresDatabase(t))
	return costtest.Backend{Cost: s, Orgs: s}
}

// onCostStores runs one scenario once per store the -store flag selects.
func onCostStores(t *testing.T, scenario func(*testing.T, costtest.Factory)) {
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
	onCostStores(t, costtest.BudgetsStrictestWins)
}

func TestBudgetFailAndSuspend(t *testing.T) {
	onCostStores(t, costtest.BudgetFailAndSuspend)
}

func TestRollupsInSameTransaction(t *testing.T) {
	onCostStores(t, costtest.RollupsInSameTransaction)
}

func TestBudgetAlertOncePerPeriod(t *testing.T) {
	onCostStores(t, costtest.BudgetAlertOncePerPeriod)
}
