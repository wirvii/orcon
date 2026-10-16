// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// The cases of the hourly counters of executions (migration 0025, SPEC-019 R-UX-BE-02): the counters of a
// project equal a recount of its executions after acceptances, terminal transitions and retries, and a
// transaction that does not commit leaves them as they were.

// finishRun moves an execution to a status through the engine's commit, reading the revision in the same
// transaction the way the engine does.
func finishRun(t *testing.T, f *fixture, org identity.OrgID, exec store.ExecutionID, status string) error {
	t.Helper()
	return f.s.InTx(f.ctx(), scopeOf(org), func(ctx context.Context, tx store.Tx) error {
		st, err := tx.LockExecution(ctx, org, exec)
		if err != nil {
			return err
		}
		return tx.SaveEngineState(ctx, store.EngineStateUpdate{
			OrgID: org, Execution: exec, ExpectedRevision: st.Revision, Status: status, FinishedAt: base, Now: base,
		})
	})
}

// countedByStatus sums the counter rows of the project by status over the summary window.
func countedByStatus(t *testing.T, f *fixture, project identity.ProjectID, org identity.OrgID) map[string]int64 {
	t.Helper()
	acts, err := f.s.ProjectActivity(f.ctx(), scopeOf(org), project, flow.SummaryDay(base), flow.SummaryWeek(base))
	if err != nil {
		t.Fatalf("project activity: %v", err)
	}
	out := map[string]int64{}
	for _, c := range acts.Executions {
		out[c.Status] += c.Count
	}
	return out
}

// recountedByStatus counts the executions of the project by status, straight from the execution table.
func recountedByStatus(t *testing.T, f *fixture, project identity.ProjectID, org identity.OrgID) map[string]int64 {
	t.Helper()
	r := rawOf(t, f.s)
	out := map[string]int64{}
	total, err := r.count(f.ctx(), org, `SELECT COUNT(*) FROM execution WHERE project_id = ?`, r.id([16]byte(project)))
	if err != nil {
		t.Fatalf("recount executions: %v", err)
	}
	out["ACCEPTED"] = total
	for _, status := range []string{"SUCCEEDED", "FAILED", "CANCELED"} {
		n, err := r.count(f.ctx(), org, `SELECT COUNT(*) FROM execution WHERE project_id = ? AND status = ?`, r.id([16]byte(project)), status)
		if err != nil {
			t.Fatalf("recount %s executions: %v", status, err)
		}
		out[status] = n
	}
	return out
}

// TestHourCountsMatchRecount: after acceptances, terminal transitions, a repeated terminal commit and a
// test run, the counters of the project agree with a recount of its executions.
func TestHourCountsMatchRecount(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "counted")
		queued := runIn(t, f, rec, base.Add(-time.Hour), "QUEUED", "TRIGGER")
		running := runIn(t, f, rec, base.Add(-90*time.Minute), "RUNNING", "TRIGGER")
		canceled := runIn(t, f, rec, base.Add(-2*time.Hour), "QUEUED", "MANUAL")
		runIn(t, f, rec, base.Add(-150*time.Minute), "SUCCEEDED", "TEST")
		if err := finishRun(t, f, f.orgA, queued, "SUCCEEDED"); err != nil {
			t.Fatalf("finish succeeded: %v", err)
		}
		if err := finishRun(t, f, f.orgA, running, "FAILED"); err != nil {
			t.Fatalf("finish failed: %v", err)
		}
		if err := finishRun(t, f, f.orgA, canceled, "CANCELED"); err != nil {
			t.Fatalf("finish canceled: %v", err)
		}
		// A repeated commit of the same final status does not count the execution twice.
		if err := finishRun(t, f, f.orgA, queued, "SUCCEEDED"); err != nil {
			t.Fatalf("repeat finish: %v", err)
		}

		want := recountedByStatus(t, f, f.projA, f.orgA)
		got := countedByStatus(t, f, f.projA, f.orgA)
		for _, status := range []string{"ACCEPTED", "SUCCEEDED", "FAILED", "CANCELED"} {
			if got[status] != want[status] {
				t.Errorf("counters of %s = %d, recount = %d", status, got[status], want[status])
			}
		}
		if want["ACCEPTED"] != 4 || want["SUCCEEDED"] != 2 {
			t.Errorf("recount = %v, want 4 accepted and 2 succeeded (one of them a test run)", want)
		}
	})
}

// TestHourCountRollsBackWithTheTransaction: a commit that the transaction does not complete changes no
// counter; the retry counts the execution once, and a further commit of the final status counts nothing.
func TestHourCountRollsBackWithTheTransaction(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "retried")
		exec := runIn(t, f, rec, base.Add(-time.Hour), "QUEUED", "TRIGGER")

		crash := errors.New("the process stopped after the commit")
		err := f.s.InTx(f.ctx(), scopeOf(f.orgA), func(ctx context.Context, tx store.Tx) error {
			st, err := tx.LockExecution(ctx, f.orgA, exec)
			if err != nil {
				return err
			}
			if err := tx.SaveEngineState(ctx, store.EngineStateUpdate{OrgID: f.orgA, Execution: exec, ExpectedRevision: st.Revision, Status: "SUCCEEDED", FinishedAt: base, Now: base}); err != nil {
				return err
			}
			return crash
		})
		if !errors.Is(err, crash) {
			t.Fatalf("the interrupted transaction returned %v", err)
		}
		if got := countedByStatus(t, f, f.projA, f.orgA)["SUCCEEDED"]; got != 0 {
			t.Errorf("an interrupted commit counted %d succeeded executions", got)
		}

		if err := finishRun(t, f, f.orgA, exec, "SUCCEEDED"); err != nil {
			t.Fatalf("retry: %v", err)
		}
		if err := finishRun(t, f, f.orgA, exec, "SUCCEEDED"); err != nil {
			t.Fatalf("second commit of the final status: %v", err)
		}
		want := recountedByStatus(t, f, f.projA, f.orgA)
		got := countedByStatus(t, f, f.projA, f.orgA)
		if got["SUCCEEDED"] != 1 || want["SUCCEEDED"] != 1 {
			t.Errorf("succeeded after the retry: counted %d, recount %d; want 1 and 1", got["SUCCEEDED"], want["SUCCEEDED"])
		}
	})
}

// TestHourCountFollowsAReopenedExecution: an execution that leaves a final status (a failed one that runs
// again in place) lowers the counter of that status in the same commit, so after RUNNING, FAILED, RUNNING
// and SUCCEEDED the counters equal the recount at every step (BL-134).
func TestHourCountFollowsAReopenedExecution(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "reopened")
		exec := runIn(t, f, rec, base.Add(-time.Hour), "RUNNING", "TRIGGER")
		other := runIn(t, f, rec, base.Add(-90*time.Minute), "QUEUED", "TRIGGER")
		if err := finishRun(t, f, f.orgA, other, "FAILED"); err != nil {
			t.Fatalf("finish the other execution: %v", err)
		}
		steps := []struct {
			status string
			want   map[string]int64
		}{
			{"FAILED", map[string]int64{"ACCEPTED": 2, "SUCCEEDED": 0, "FAILED": 2, "CANCELED": 0}},
			{"RUNNING", map[string]int64{"ACCEPTED": 2, "SUCCEEDED": 0, "FAILED": 1, "CANCELED": 0}},
			{"SUCCEEDED", map[string]int64{"ACCEPTED": 2, "SUCCEEDED": 1, "FAILED": 1, "CANCELED": 0}},
		}
		for _, step := range steps {
			if err := finishRun(t, f, f.orgA, exec, step.status); err != nil {
				t.Fatalf("move to %s: %v", step.status, err)
			}
			got := countedByStatus(t, f, f.projA, f.orgA)
			recount := recountedByStatus(t, f, f.projA, f.orgA)
			for status, want := range step.want {
				if got[status] != want || recount[status] != want {
					t.Errorf("after %s: %s counted %d, recount %d, want %d", step.status, status, got[status], recount[status], want)
				}
			}
		}
	})
}
