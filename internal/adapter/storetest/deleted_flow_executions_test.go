// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	execstore "github.com/wirvii/orcon/internal/execution/store"
)

// TestExecutionsOfDeletedFlowStayInHistory: the executions of a soft-deleted flow stay in the project
// history, in the activity counters and in the flow-scoped list until retention purges the flow (ADR-0007
// section on deleting a flow, ADR-0017 section 8, ADR-0077 section 4), while the flow list no longer
// shows the flow. Both dialects answer the same.
func TestExecutionsOfDeletedFlowStayInHistory(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		gone := f.flow(f.orgA, f.projA, "deleted later")
		kept := f.flow(f.orgA, f.projA, "kept")
		ok := runIn(t, f, gone, base.Add(-3*time.Hour), "SUCCEEDED", "TRIGGER")
		bad := runIn(t, f, gone, base.Add(-3*time.Hour+time.Minute), "FAILED", "TRIGGER")

		r := rawOf(t, f.s)
		if err := r.exec(f.ctx(), f.orgA, "UPDATE flow SET deleted_at = ? WHERE id = ?", r.instant(base.Add(-2*time.Hour)), r.id(gone.ID)); err != nil {
			t.Fatalf("soft delete: %v", err)
		}

		// The project history lists both, with and without the flow filter.
		all, err := reader(t, f).ListExecutions(f.ctx(), scopeOf(f.orgA), execstore.ExecutionFilter{ProjectID: f.projA}, execstore.Cursor{}, 10)
		if err != nil {
			t.Fatalf("list project executions: %v", err)
		}
		if !sameExecutionIDs(executionIDsOf(all), ok, bad) {
			t.Errorf("project executions = %v, want both of the deleted flow", executionIDsOf(all))
		}
		byFlow, err := reader(t, f).ListExecutions(f.ctx(), scopeOf(f.orgA), execstore.ExecutionFilter{ProjectID: f.projA, FlowID: gone.ID}, execstore.Cursor{}, 10)
		if err != nil {
			t.Fatalf("list executions of the deleted flow: %v", err)
		}
		if !sameExecutionIDs(executionIDsOf(byFlow), ok, bad) {
			t.Errorf("executions of the deleted flow = %v, want both", executionIDsOf(byFlow))
		}

		// The home counters keep counting them.
		act, err := f.s.ProjectActivity(f.ctx(), scopeOf(f.orgA), f.projA, base.Add(-24*time.Hour), base.Add(-7*24*time.Hour))
		if err != nil {
			t.Fatalf("project activity: %v", err)
		}
		var accepted, failed int64
		for _, c := range act.Executions {
			switch c.Status {
			case "ACCEPTED":
				accepted += c.Count
			case "FAILED":
				failed += c.Count
			}
		}
		if accepted != 2 || failed != 1 || act.Flows != 1 {
			t.Errorf("activity = accepted %d, failed %d, flows %d; want 2, 1, 1", accepted, failed, act.Flows)
		}

		// The flow list shows only the live flow, and the deleted flow's executions count for no row.
		rows := activityOf(t, f, f.orgA, f.projA)
		if len(rows) != 1 || rows[0].Flow.ID != kept.ID || rows[0].ExecutionCountLast24h != 0 || rows[0].LastExecution != nil {
			t.Errorf("flow rows = %+v, want only the kept flow with no executions", rows)
		}
	})
}
