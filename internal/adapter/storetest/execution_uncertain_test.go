// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"slices"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/authz"
	execstore "github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
)

// The cases of SPEC-019 UXB3: an execution says whether it has an operation whose outcome is unknown,
// the organization list shows only the projects the caller reads and filters by identity, status, flow,
// project, time and uncertain outcome, and the project list takes the uncertain filter too.

// listProject lists the executions of one project under a query, with no cursor.
func listProject(t *testing.T, f *fixture, project identity.ProjectID, query execstore.ExecutionQuery) []execstore.Execution {
	t.Helper()
	got, err := f.s.ListProjectExecutions(f.ctx(), scopeOf(f.orgA), project, query, execstore.Cursor{}, 100)
	if err != nil {
		t.Fatalf("list project executions: %v", err)
	}
	return got
}

// executionIDsOf returns the identifiers of the executions, in order.
func executionIDsOf(rows []execstore.Execution) []execstore.ExecutionID {
	out := make([]execstore.ExecutionID, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].ID)
	}
	return out
}

// sameExecutionIDs reports whether got holds exactly the identifiers want holds, in any order.
func sameExecutionIDs(got []execstore.ExecutionID, want ...execstore.ExecutionID) bool {
	if len(got) != len(want) {
		return false
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			return false
		}
	}
	return true
}

// TestListProjectExecutionsKeysetBothDirections: the project listing walks past a cursor in the direction
// asked for, newest first by default and oldest first under OldestFirst, and the two walks of the same
// five executions are each other's reverse (BL-135).
func TestListProjectExecutionsKeysetBothDirections(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "keyset")
		var oldestFirst []execstore.ExecutionID
		for i := range 5 {
			oldestFirst = append(oldestFirst, runIn(t, f, rec, base.Add(time.Duration(i-10)*time.Minute), "SUCCEEDED", "TRIGGER"))
		}
		for _, oldest := range []bool{true, false} {
			want := slices.Clone(oldestFirst)
			if !oldest {
				slices.Reverse(want)
			}
			var got []execstore.ExecutionID
			var cursor execstore.Cursor
			for range 5 {
				rows, err := f.s.ListProjectExecutions(f.ctx(), scopeOf(f.orgA), f.projA, execstore.ExecutionQuery{OldestFirst: oldest}, cursor, 2)
				if err != nil {
					t.Fatalf("list (oldest first %v): %v", oldest, err)
				}
				if len(rows) == 0 {
					break
				}
				got = append(got, executionIDsOf(rows)...)
				last := rows[len(rows)-1]
				cursor = execstore.Cursor{CreatedAt: last.CreatedAt, ID: [16]byte(last.ID)}
			}
			if !slices.Equal(got, want) {
				t.Errorf("walk with oldest first %v = %v, want %v", oldest, got, want)
			}
		}
	})
}

// reader returns the store as the execution reader it implements (the storetest suite names only the
// project listing port; the single read and the filtered listing are the adapter's own methods).
func reader(t *testing.T, f *fixture) execstore.Reader { //nolint:ireturn // the callers choose the adapter's reader, not a type

	t.Helper()
	r, ok := f.s.(execstore.Reader)
	if !ok {
		t.Fatalf("the store does not implement the execution reader")
	}
	return r
}

// TestExecutionHasUncertainOperation: an execution has the flag only when one of its external operations
// is UNCERTAIN; the flag is set on the project listing and on a single read alike, and the flow version
// number comes with the execution.
func TestExecutionHasUncertainOperation(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "review")
		uncertain := uncertainRun(t, f, rec, base.Add(-time.Hour))
		clean := runIn(t, f, rec, base.Add(-2*time.Hour), "SUCCEEDED", "TRIGGER")

		byID := map[execstore.ExecutionID]execstore.Execution{}
		for _, e := range listProject(t, f, f.projA, execstore.ExecutionQuery{}) {
			byID[e.ID] = e
		}
		if !byID[uncertain].HasUncertainOperation || byID[clean].HasUncertainOperation {
			t.Errorf("flags in the listing: uncertain %v, clean %v; want true and false",
				byID[uncertain].HasUncertainOperation, byID[clean].HasUncertainOperation)
		}
		if byID[uncertain].FlowVersionNumber == 0 {
			t.Errorf("the listed execution has no flow version number")
		}
		single, err := reader(t, f).GetExecution(f.ctx(), scopeOf(f.orgA), uncertain)
		if err != nil || !single.HasUncertainOperation {
			t.Errorf("single read of the uncertain execution: flag %v, %v; want true", single.HasUncertainOperation, err)
		}
	})
}

// TestListOrgExecutionsOnlyReadableProjects: the executions a user lists across the organization are
// those of the projects the user reads; the executions of another project are left out.
func TestListOrgExecutionsOnlyReadableProjects(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		beta := f.project(f.orgA, "beta")
		alphaRun := runIn(t, f, f.flow(f.orgA, f.projA, "alpha run"), base.Add(-time.Hour), "SUCCEEDED", "TRIGGER")
		betaRun := runIn(t, f, f.flow(f.orgA, beta, "beta run"), base.Add(-time.Hour), "SUCCEEDED", "TRIGGER")
		user := f.user("exec-readable@example.test")
		f.member(f.orgA, user, identity.PrincipalID{})
		bindProject(t, f, user, beta, authz.RoleProjectAdmin)

		rows, _ := readableProjects(t, f, ev, f.orgA, user)
		var listed []execstore.ExecutionID
		for _, p := range rows {
			listed = append(listed, executionIDsOf(listProject(t, f, p.ID, execstore.ExecutionQuery{}))...)
		}
		if !sameExecutionIDs(listed, betaRun) || slices.Contains(listed, alphaRun) {
			t.Errorf("executions listed for a beta Viewer = %v, want only the beta run", listed)
		}
	})
}

// TestListOrgExecutionsFiltersUncertain: the uncertain query keeps the executions with an unknown outcome.
func TestListOrgExecutionsFiltersUncertain(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "filtered")
		uncertain := uncertainRun(t, f, rec, base.Add(-time.Hour))
		runIn(t, f, rec, base.Add(-2*time.Hour), "SUCCEEDED", "TRIGGER")
		got := listProject(t, f, f.projA, execstore.ExecutionQuery{UncertainOnly: true})
		if !sameExecutionIDs(executionIDsOf(got), uncertain) {
			t.Errorf("uncertain executions = %v, want only %s", executionIDsOf(got), uncertain)
		}
	})
}

// TestListOrgExecutionsFiltersByIdStatusFlowProjectAndDates: each filter of the list keeps the executions
// that match it and no others; the time filters take an instant at or after and before.
func TestListOrgExecutionsFiltersByIdStatusFlowProjectAndDates(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		beta := f.project(f.orgA, "beta")
		x := f.flow(f.orgA, f.projA, "x")
		y := f.flow(f.orgA, f.projA, "y")
		recent := runIn(t, f, y, base.Add(-30*time.Minute), "SUCCEEDED", "TRIGGER")
		earlier := runIn(t, f, x, base.Add(-time.Hour), "SUCCEEDED", "TRIGGER")
		failed := runIn(t, f, x, base.Add(-3*time.Hour), "FAILED", "TRIGGER")
		other := runIn(t, f, f.flow(f.orgA, beta, "elsewhere"), base.Add(-time.Hour), "SUCCEEDED", "TRIGGER")

		check := func(name string, query execstore.ExecutionQuery, want ...execstore.ExecutionID) {
			t.Helper()
			project := f.projA
			if slices.Contains(want, other) {
				project = beta
			}
			if got := executionIDsOf(listProject(t, f, project, query)); !sameExecutionIDs(got, want...) {
				t.Errorf("%s: %v, want %v", name, got, want)
			}
		}
		check("the identity", execstore.ExecutionQuery{ID: failed}, failed)
		check("the status", execstore.ExecutionQuery{Status: "FAILED"}, failed)
		check("the flow", execstore.ExecutionQuery{FlowID: y.ID}, recent)
		check("the project", execstore.ExecutionQuery{}, recent, earlier, failed)
		check("accepted at or after", execstore.ExecutionQuery{CreatedAfter: base.Add(-2 * time.Hour)}, recent, earlier)
		check("accepted before", execstore.ExecutionQuery{CreatedBefore: base.Add(-2 * time.Hour)}, failed)
		check("the other project", execstore.ExecutionQuery{}, other)
	})
}

// TestListExecutionsFiltersUncertain: the project listing takes the uncertain filter, the one field that
// may narrow a project, and returns the same executions as the uncertain query of the project.
func TestListExecutionsFiltersUncertain(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "listed")
		uncertain := uncertainRun(t, f, rec, base.Add(-time.Hour))
		runIn(t, f, rec, base.Add(-2*time.Hour), "SUCCEEDED", "TRIGGER")
		got, err := reader(t, f).ListExecutions(f.ctx(), scopeOf(f.orgA), execstore.ExecutionFilter{ProjectID: f.projA, Uncertain: true}, execstore.Cursor{}, 10)
		if err != nil {
			t.Fatalf("list executions with the uncertain filter: %v", err)
		}
		if !sameExecutionIDs(executionIDsOf(got), uncertain) {
			t.Errorf("project executions with the uncertain filter = %v, want only %s", executionIDsOf(got), uncertain)
		}
	})
}
