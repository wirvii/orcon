// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// longRun enables the budget case of the activity summary, which seeds a million executions. The
// case is named in a criterion and runs with -args -long.
var longRun = flag.Bool("long", false, "run the long cases: the activity summary budget seeds a million executions")

// The cases of SPEC-019 UXB2: the activity summary of an organization (getOrgActivitySummary) counts
// only the projects the caller reads, places the executions of the last 24 hours in their hours, counts
// the uncertain ones, computes the time saved with the organization's minutes and leaves out test runs.

// summaryAt reads the summary of the organization over the projects the user reads, at now.
func summaryAt(t *testing.T, f *fixture, ev *authz.Evaluator, org identity.OrgID, user identity.UserRecord, now time.Time) flow.ActivitySummary {
	t.Helper()
	rows, _ := readableProjects(t, f, ev, org, user)
	sum, err := flow.ReadActivitySummary(f.ctx(), f.s, scopeOf(org), projectIDs(rows), now)
	if err != nil {
		t.Fatalf("activity summary of %s: %v", org, err)
	}
	return sum
}

// runIn inserts an execution of the flow in the given mode, accepted at created in the given status.
func runIn(t *testing.T, f *fixture, rec flow.Record, created time.Time, status, mode string) store.ExecutionID {
	t.Helper()
	ver := publishVersionAt(t, f, rec, created)
	ex := store.Execution{
		ID: next[store.ExecutionID](f), OrgID: rec.OrgID, ProjectID: rec.ProjectID, FlowID: rec.ID, FlowVersionID: ver.ID,
		Status: status, Mode: mode, RunnableCount: 0, CreatedAt: created, UpdatedAt: created,
	}
	ex.RootExecutionID = ex.ID
	if err := f.s.InTx(f.ctx(), scopeOf(rec.OrgID), func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &ex) }); err != nil {
		t.Fatalf("insert execution %s/%s: %v", status, mode, err)
	}
	return ex.ID
}

// uncertainRun inserts an execution accepted at created that has one external operation whose outcome
// is unknown: the operation is prepared through the journal, then its state is set to UNCERTAIN.
func uncertainRun(t *testing.T, f *fixture, rec flow.Record, created time.Time) store.ExecutionID {
	t.Helper()
	exID := runIn(t, f, rec, created, "WAITING", "TRIGGER")
	ctx := f.ctx()
	tok := store.Token{
		ID: next[store.TokenID](f), OrgID: rec.OrgID, ExecutionID: exID, Status: "RUNNABLE", NodeID: "effect", ScopePath: "",
		ForkLineage: []byte(`[]`), FairnessKey: "flow/" + rec.ID.String(), EffectivePriority: 0,
		AvailableAt: created, CreatedAt: created, UpdatedAt: created,
	}
	ne := store.NodeExecution{
		ID: next[store.NodeExecutionID](f), OrgID: rec.OrgID, ExecutionID: exID, TokenID: tok.ID, NodeID: "effect",
		Status: "RUNNING", StartedAt: created, CreatedAt: created, UpdatedAt: created,
	}
	op := store.ExternalOperation{
		ID: next[store.ExternalOperationID](f), OrgID: rec.OrgID, ExecutionID: exID, NodeExecutionID: ne.ID,
		ItemIndex: 0, CorrelationKey: fmt.Sprintf("uncertain-%s", exID), ContractReference: "c", CreatedAt: created,
	}
	err := f.s.InTx(ctx, scopeOf(rec.OrgID), func(ctx context.Context, tx store.Tx) error {
		if err := tx.InsertToken(ctx, &tok); err != nil {
			return err
		}
		if err := tx.InsertNodeExecution(ctx, ne); err != nil {
			return err
		}
		return tx.AppendJournal(ctx, []store.ExternalOperation{op})
	})
	if err != nil {
		t.Fatalf("journal an operation: %v", err)
	}
	r := rawOf(t, f.s)
	mustExecOrg(t, r, rec.OrgID, `UPDATE external_operation SET state = ? WHERE id = ?`, "UNCERTAIN", r.id([16]byte(op.ID)))
	return exID
}

// TestActivitySummaryCountsOnlyReadableProjects: the figures add up only the projects the caller reads.
func TestActivitySummaryCountsOnlyReadableProjects(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		beta := f.project(f.orgA, "beta")
		alpha := f.flow(f.orgA, f.projA, "alpha flow")
		betaFlow := f.flow(f.orgA, beta, "beta flow")
		f.flow(f.orgA, beta, "beta idle")
		runIn(t, f, alpha, base.Add(-time.Hour), "SUCCEEDED", "TRIGGER")
		runIn(t, f, betaFlow, base.Add(-2*time.Hour), "FAILED", "TRIGGER")

		viewer := f.user("viewer-summary@example.test")
		f.member(f.orgA, viewer, identity.PrincipalID{})
		bindProject(t, f, viewer, beta, "Viewer")
		got := summaryAt(t, f, ev, f.orgA, viewer, base)
		if got.ProjectCount != 1 || got.FlowCount != 2 || got.Executions.Total != 1 || got.Executions.Failed != 1 {
			t.Errorf("beta Viewer summary = %+v, want one project, two flows and the one failed run of beta", got)
		}

		admin := f.user("admin-summary@example.test")
		f.member(f.orgA, admin, identity.PrincipalID{})
		bindOrg(t, f, admin, identity.RoleAdmin)
		all := summaryAt(t, f, ev, f.orgA, admin, base)
		if all.ProjectCount != 2 || all.FlowCount != 3 || all.Executions.Total != 2 {
			t.Errorf("Admin summary = %+v, want both projects, three flows and both runs", all)
		}
	})
}

// TestActivitySummaryHourlyBuckets: the executions of the last 24 hours fall in their hour, the oldest
// hour first; an execution older than the window is left out of the hours and of the total.
func TestActivitySummaryHourlyBuckets(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		rec := f.flow(f.orgA, f.projA, "hours")
		user := f.user("hours-summary@example.test")
		f.member(f.orgA, user, identity.PrincipalID{})
		bindOrg(t, f, user, identity.RoleAdmin)

		runIn(t, f, rec, base.Add(-23*time.Hour), "SUCCEEDED", "TRIGGER") // 10:30 of the first hour of the window
		runIn(t, f, rec, base.Add(-time.Hour), "SUCCEEDED", "TRIGGER")    // 08:30, the penultimate hour
		runIn(t, f, rec, base.Add(-25*time.Hour), "FAILED", "TRIGGER")    // before the window

		got := summaryAt(t, f, ev, f.orgA, user, base)
		if got.Executions.Total != 2 || got.Executions.Succeeded != 2 {
			t.Errorf("executions of the day = %+v, want two succeeded", got.Executions)
		}
		if got.Executions.ByHour[0] != 1 || got.Executions.ByHour[flow.SummaryHours-2] != 1 {
			t.Errorf("hours = %v, want one at the first hour and one at the penultimate", got.Executions.ByHour)
		}
		var sum int64
		for _, n := range got.Executions.ByHour {
			sum += n
		}
		if sum != got.Executions.Total {
			t.Errorf("the hours add up to %d, the total is %d", sum, got.Executions.Total)
		}
	})
}

// TestActivitySummaryUncertainCount: an execution with an operation whose outcome is unknown is counted
// as uncertain once, however many such operations it has.
func TestActivitySummaryUncertainCount(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		rec := f.flow(f.orgA, f.projA, "uncertain")
		user := f.user("uncertain-summary@example.test")
		f.member(f.orgA, user, identity.PrincipalID{})
		bindOrg(t, f, user, identity.RoleAdmin)

		uncertainRun(t, f, rec, base.Add(-time.Hour))
		runIn(t, f, rec, base.Add(-2*time.Hour), "SUCCEEDED", "TRIGGER")
		got := summaryAt(t, f, ev, f.orgA, user, base)
		if got.Executions.Uncertain != 1 || got.Executions.Total != 2 {
			t.Errorf("uncertain = %d of %d executions, want 1 of 2", got.Executions.Uncertain, got.Executions.Total)
		}
	})
}

// TestActivitySummaryTimeSavedUsesOrgMinutesDefaultThree: the minutes of the organization default to 3,
// and the time saved multiplies the successful automated executions of the week by them.
func TestActivitySummaryTimeSavedUsesOrgMinutesDefaultThree(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		rec := f.flow(f.orgA, f.projA, "saving")
		user := f.user("saving-summary@example.test")
		f.member(f.orgA, user, identity.PrincipalID{})
		bindOrg(t, f, user, identity.RoleAdmin)
		runIn(t, f, rec, base.Add(-time.Hour), "SUCCEEDED", "TRIGGER")
		runIn(t, f, rec, base.Add(-3*24*time.Hour), "SUCCEEDED", "TRIGGER")
		runIn(t, f, rec, base.Add(-10*24*time.Hour), "SUCCEEDED", "TRIGGER") // outside the week

		got := summaryAt(t, f, ev, f.orgA, user, base).TimeSaved
		if got.MinutesPerExecution != 3 || got.SucceededExecutions != 2 || got.Minutes != 6 || got.WindowDays != 7 {
			t.Errorf("time saved = %+v, want 2 executions at the default 3 minutes", got)
		}
		r := rawOf(t, f.s)
		mustExecOrg(t, r, f.orgA, `UPDATE org SET time_saved_minutes_per_execution = ? WHERE id = ?`, int64(5), r.id([16]byte(f.orgA)))
		got = summaryAt(t, f, ev, f.orgA, user, base).TimeSaved
		if got.MinutesPerExecution != 5 || got.Minutes != 10 {
			t.Errorf("time saved after the organization sets 5 minutes = %+v, want 10 minutes", got)
		}
	})
}

// TestActivitySummaryIgnoresTestExecutions: a test run that succeeded is not time saved.
func TestActivitySummaryIgnoresTestExecutions(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		rec := f.flow(f.orgA, f.projA, "tests")
		user := f.user("tests-summary@example.test")
		f.member(f.orgA, user, identity.PrincipalID{})
		bindOrg(t, f, user, identity.RoleAdmin)
		runIn(t, f, rec, base.Add(-time.Hour), "SUCCEEDED", "TEST")
		runIn(t, f, rec, base.Add(-2*time.Hour), "SUCCEEDED", "TRIGGER")

		got := summaryAt(t, f, ev, f.orgA, user, base).TimeSaved
		if got.SucceededExecutions != 1 || got.Minutes != 3 {
			t.Errorf("time saved = %+v, want the one automated execution only", got)
		}
	})
}

// TestActivitySummaryOtherOrganizationNotFound: a user of one organization gets no summary of another;
// the authorization hides the organization (404).
func TestActivitySummaryOtherOrganizationNotFound(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		user := f.user("outsider-summary@example.test")
		f.member(f.orgA, user, identity.PrincipalID{})
		bindOrg(t, f, user, identity.RoleAdmin)
		d, _, err := ev.Authorize(f.ctx(), requestAs(user.ID, f.orgB, "", "execution:read"))
		if err != nil || d != authz.DecisionHidden {
			t.Errorf("summary of organization B for a member of A: %v, %v; want hidden", d, err)
		}
	})
}

// TestActivitySummaryBudget (SPEC-019 R-UX-BE-02): with a million executions accepted in the last 24
// hours, the summary answers with a 95th percentile of at most 150 milliseconds on both stores. The
// case runs only with -args -long, because it seeds the million rows.
func TestActivitySummaryBudget(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		if !*longRun {
			t.Skip("the budget seeds a million executions; run it with -args -long")
		}
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "busy")
		seedExecutions(t, f, rec, 1_000_000)

		const runs = 20
		const budget = 150 * time.Millisecond
		durations := make([]time.Duration, 0, runs)
		for range runs {
			start := time.Now()
			if _, err := flow.ReadActivitySummary(f.ctx(), f.s, scopeOf(f.orgA), []identity.ProjectID{f.projA}, base); err != nil {
				t.Fatalf("activity summary: %v", err)
			}
			durations = append(durations, time.Since(start))
		}
		slices.Sort(durations)
		p95 := durations[(runs*95+99)/100-1]
		t.Logf("activity summary over 1 000 000 executions: p95 %s, max %s", p95, durations[runs-1])
		if p95 > budget {
			t.Errorf("activity summary p95 = %s, want at most %s", p95, budget)
		}
	})
}

// seedExecutions inserts n executions of the flow spread over the 24 hours before base, through bulk
// statements of seedBatch rows per dialect (the rows are written by SQL, not by the store: the budget
// measures reads, and a single statement of a million rows outlasts the statement timeout of the
// PostgreSQL role). One execution in twenty failed. The parameters of each statement are the
// organization, the project, the flow, the version and base, in that order.
func seedExecutions(t *testing.T, f *fixture, rec flow.Record, n int) {
	t.Helper()
	const seedBatch = 100_000
	ver := publishVersionAt(t, f, rec, base)
	r := rawOf(t, f.s)
	for from := 1; from <= n; from += seedBatch {
		to := min(from+seedBatch-1, n)
		var query string
		switch r.dialect() {
		case "sqlite":
			query = fmt.Sprintf(`WITH RECURSIVE seq(n) AS (SELECT %d UNION ALL SELECT n + 1 FROM seq WHERE n < %d)
INSERT INTO execution (id, org_id, project_id, flow_id, flow_version_id, flow_snapshot_id, status, mode, priority,
  root_execution_id, call_depth, run_generation, runnable_count, leased_count, waiting_count, suspended_count, active_milliseconds, created_at, updated_at)
SELECT CAST(printf('%%016d', n) AS BLOB), ?, ?, ?, ?, NULL,
  CASE WHEN n %% 20 = 0 THEN 'FAILED' ELSE 'SUCCEEDED' END, 'TRIGGER', 0,
  CAST(printf('%%016d', n) AS BLOB), 0, 0, 0, 0, 0, 0, 0, ? - n * 86400, ? - n * 86400
FROM seq`, from, to)
		case "postgres":
			query = fmt.Sprintf(`INSERT INTO execution (id, org_id, project_id, flow_id, flow_version_id, flow_snapshot_id, status, mode, priority,
  root_execution_id, call_depth, run_generation, runnable_count, leased_count, waiting_count, suspended_count, active_milliseconds, created_at, updated_at)
SELECT md5(n::text)::uuid, ?, ?, ?, ?, NULL,
  CASE WHEN n %% 20 = 0 THEN 'FAILED' ELSE 'SUCCEEDED' END, 'TRIGGER', 0,
  md5(n::text)::uuid, 0, 0, 0, 0, 0, 0, 0, CAST(? AS timestamptz) - n * interval '86.4 milliseconds', CAST(? AS timestamptz) - n * interval '86.4 milliseconds'
FROM generate_series(%d, %d) AS n`, from, to)
		default:
			t.Fatalf("no bulk insert for the %s dialect", r.dialect())
		}
		mustExecOrg(t, r, rec.OrgID, query,
			r.id([16]byte(rec.OrgID)), r.id([16]byte(rec.ProjectID)), r.id([16]byte(rec.ID)), r.id([16]byte(ver.ID)),
			r.instant(base), r.instant(base))
	}
	// The seed writes the executions directly, so the hourly counters are filled by the same backfill
	// the migration runs (migration 0025); the write path itself is covered by the counter tests.
	for _, stmt := range backfillCounters(r.dialect()) {
		mustExecOrg(t, r, rec.OrgID, stmt)
	}
}

// backfillCounters returns the statements that fill the hourly counters from the executions, as migration
// 0025 does for an existing database.
func backfillCounters(dialect string) []string {
	if dialect == "postgres" {
		return []string{
			`INSERT INTO execution_hour_count (id, org_id, project_id, hour_started_at, mode, status, execution_count, created_at, updated_at)
SELECT gen_random_uuid(), org_id, project_id, date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC', mode, 'ACCEPTED', COUNT(*), min(created_at), max(updated_at)
FROM execution GROUP BY org_id, project_id, date_trunc('hour', created_at AT TIME ZONE 'UTC'), mode`,
			`INSERT INTO execution_hour_count (id, org_id, project_id, hour_started_at, mode, status, execution_count, created_at, updated_at)
SELECT gen_random_uuid(), org_id, project_id, date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC', mode, status, COUNT(*), min(created_at), max(updated_at)
FROM execution WHERE status IN ('SUCCEEDED', 'FAILED', 'CANCELED')
GROUP BY org_id, project_id, date_trunc('hour', created_at AT TIME ZONE 'UTC'), mode, status`,
		}
	}
	return []string{
		`INSERT INTO execution_hour_count (id, org_id, project_id, hour_started_at, mode, status, execution_count, created_at, updated_at)
SELECT randomblob(16), org_id, project_id, (created_at / 3600000000) * 3600000000, mode, 'ACCEPTED', COUNT(*), min(created_at), max(updated_at)
FROM execution GROUP BY org_id, project_id, created_at / 3600000000, mode`,
		`INSERT INTO execution_hour_count (id, org_id, project_id, hour_started_at, mode, status, execution_count, created_at, updated_at)
SELECT randomblob(16), org_id, project_id, (created_at / 3600000000) * 3600000000, mode, status, COUNT(*), min(created_at), max(updated_at)
FROM execution WHERE status IN ('SUCCEEDED', 'FAILED', 'CANCELED')
GROUP BY org_id, project_id, created_at / 3600000000, mode, status`,
	}
}
