// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// The cases of SPEC-019 UXB1: the flow list with its latest execution and its executions of the
// last day, the organization list of flows, the flow count of a project and the organization roles
// of the signed-in user. They run over a real store of each dialect.

// activitySince is the start of the last day the cases count executions from.
var activitySince = base.Add(-24 * time.Hour)

// publishVersionAt publishes a version of the flow for one run. Its number is the instant of the
// run in seconds, so every run of a flow in these cases has a version of its own.
func publishVersionAt(t *testing.T, f *fixture, rec flow.Record, at time.Time) flow.VersionRecord {
	t.Helper()
	number := at.Unix()
	v := flow.VersionRecord{
		ID: next[flow.FlowVersionID](f), OrgID: rec.OrgID, FlowID: rec.ID, Number: number,
		Document: []byte(`{"version":1}`), ContentHash: fmt.Sprintf("%064d", number), PublishedAt: base, CreatedAt: base,
	}
	if err := f.s.PublishVersion(f.ctx(), scopeOf(rec.OrgID), v); err != nil {
		t.Fatalf("publish version %d: %v", number, err)
	}
	return v
}

// runAt inserts an execution of the flow's first version, accepted at created and in status. A
// terminal status gets a finish instant five seconds later.
func runAt(t *testing.T, f *fixture, rec flow.Record, created time.Time, status string) store.ExecutionID {
	t.Helper()
	ver := publishVersionAt(t, f, rec, created)
	ex := store.Execution{
		ID: next[store.ExecutionID](f), OrgID: rec.OrgID, ProjectID: rec.ProjectID, FlowID: rec.ID, FlowVersionID: ver.ID,
		Status: status, Mode: "MANUAL", RunnableCount: 0, CreatedAt: created, UpdatedAt: created,
	}
	ex.RootExecutionID = ex.ID
	err := f.s.InTx(f.ctx(), scopeOf(rec.OrgID), func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &ex) })
	if err != nil {
		t.Fatalf("insert execution %s: %v", status, err)
	}
	// The insert does not write the finish instant; the engine sets it on the terminal transition.
	if status == "SUCCEEDED" || status == "FAILED" {
		r := rawOf(t, f.s)
		mustExecOrg(t, r, rec.OrgID, `UPDATE execution SET finished_at = ? WHERE id = ?`, r.instant(created.Add(5*time.Second)), r.id([16]byte(ex.ID)))
	}
	return ex.ID
}

// activityOf lists the flows of a project with their activity, one page of up to hundred flows.
func activityOf(t *testing.T, f *fixture, org identity.OrgID, project identity.ProjectID) []flow.Activity {
	t.Helper()
	out, err := f.s.ListFlowActivity(f.ctx(), scopeOf(org), project, activitySince, flow.FlowID{}, 100)
	if err != nil {
		t.Fatalf("list flow activity: %v", err)
	}
	return out
}

// TestListFlowsIncludesLastExecution: a listed flow carries its most recent execution (the latest
// accepted one, not the last one read), with status and finish instant; a flow that never ran has none.
func TestListFlowsIncludesLastExecution(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ran := f.flow(f.orgA, f.projA, "ran")
		f.flow(f.orgA, f.projA, "idle")
		runAt(t, f, ran, base.Add(-2*time.Hour), "SUCCEEDED")
		latest := runAt(t, f, ran, base.Add(-10*time.Minute), "FAILED")

		byName := map[string]flow.Activity{}
		for _, a := range activityOf(t, f, f.orgA, f.projA) {
			byName[a.Flow.Name] = a
		}
		got := byName["ran"].LastExecution
		if got == nil {
			t.Fatal("the flow that ran has no last execution")
		}
		if got.ID != id.UUID(latest) || got.Status != "FAILED" || got.HasUncertainOperation {
			t.Errorf("last execution = %+v, want the failed one %s", got, latest)
		}
		// The stores keep microseconds, so the expected instants are cut to them.
		wantCreated := base.Add(-10 * time.Minute).Truncate(time.Microsecond)
		wantFinished := base.Add(-10*time.Minute + 5*time.Second).Truncate(time.Microsecond)
		if !got.CreatedAt.Equal(wantCreated) || !got.FinishedAt.Equal(wantFinished) {
			t.Errorf("last execution instants = %v / %v", got.CreatedAt, got.FinishedAt)
		}
		if byName["idle"].LastExecution != nil {
			t.Errorf("a flow that never ran has a last execution: %+v", byName["idle"].LastExecution)
		}
	})
}

// TestListFlowsCountsExecutionsLast24h: the count takes the executions accepted in the 24 hours
// before the read and leaves out the older ones.
func TestListFlowsCountsExecutionsLast24h(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "busy")
		runAt(t, f, rec, base.Add(-25*time.Hour), "SUCCEEDED")
		runAt(t, f, rec, base.Add(-2*time.Hour), "SUCCEEDED")
		runAt(t, f, rec, base.Add(-1*time.Hour), "RUNNING")
		got := activityOf(t, f, f.orgA, f.projA)
		if len(got) != 1 || got[0].ExecutionCountLast24h != 2 {
			t.Errorf("executions in the last 24 hours = %+v, want 2", got)
		}
	})
}

// TestListFlowsFiltersByActivationStateAndName: the list filter keeps the flows in the activation
// state asked for and whose name starts with the text, ignoring case.
func TestListFlowsFiltersByActivationStateAndName(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		f.flow(f.orgA, f.projA, "Welcome email")
		f.flow(f.orgA, f.projA, "Welcome SMS")
		f.flow(f.orgA, f.projA, "Weekly report")
		r := rawOf(t, f.s)
		mustExecOrg(t, r, f.orgA, `UPDATE flow SET activation_state = 'ACTIVE' WHERE name = ?`, "Welcome email")

		count := func(filter flow.ListFilter) int {
			n := 0
			for _, a := range activityOf(t, f, f.orgA, f.projA) {
				if filter.Match(a.Flow) {
					n++
				}
			}
			return n
		}
		if n := count(flow.ListFilter{NamePrefix: "WELCOME"}); n != 2 {
			t.Errorf("name prefix WELCOME matches %d flows, want 2", n)
		}
		if n := count(flow.ListFilter{ActivationState: flow.ActivationActive}); n != 1 {
			t.Errorf("ACTIVE matches %d flows, want 1", n)
		}
		if n := count(flow.ListFilter{NamePrefix: "welcome", ActivationState: flow.ActivationInactive}); n != 1 {
			t.Errorf("welcome and INACTIVE matches %d flows, want 1", n)
		}
	})
}

// TestListOrgFlowsOnlyReadableProjects: the flows of an organization that a user reads are the flows
// of the projects the user can read; a project the user cannot read contributes none.
func TestListOrgFlowsOnlyReadableProjects(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		beta := f.project(f.orgA, "beta")
		f.flow(f.orgA, f.projA, "in alpha")
		f.flow(f.orgA, beta, "in beta")
		f.flow(f.orgB, f.projB, "other organization")
		user := f.user("scoped-flows@example.test")
		f.member(f.orgA, user, identity.PrincipalID{})
		bindProject(t, f, user, beta, "Viewer")

		rows, _ := readableProjects(t, f, ev, f.orgA, user)
		var names []string
		for _, p := range rows {
			for _, a := range activityOf(t, f, f.orgA, p.ID) {
				names = append(names, a.Flow.Name)
			}
		}
		if len(names) != 1 || names[0] != "in beta" {
			t.Errorf("flows listed for a project Viewer of beta: %v, want [in beta]", names)
		}
	})
}

// TestListOrgFlowsPaginates: the readers of the organization list go through the flows of each
// readable project one page at a time, with no flow repeated or skipped.
func TestListOrgFlowsPaginates(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		for _, name := range []string{"a", "b", "c", "d", "e"} {
			f.flow(f.orgA, f.projA, name)
		}
		seen := map[flow.FlowID]bool{}
		var after flow.FlowID
		pages := 0
		for {
			page, err := f.s.ListFlowActivity(f.ctx(), scopeOf(f.orgA), f.projA, activitySince, after, 2)
			if err != nil {
				t.Fatalf("page %d: %v", pages, err)
			}
			pages++
			for _, a := range page {
				if seen[a.Flow.ID] {
					t.Fatalf("flow %s repeated on page %d", a.Flow.Name, pages)
				}
				seen[a.Flow.ID] = true
			}
			if len(page) < 2 {
				break
			}
			after = page[len(page)-1].Flow.ID
		}
		if len(seen) != 5 || pages != 3 {
			t.Errorf("paged %d flows in %d pages, want 5 flows in 3 pages", len(seen), pages)
		}
	})
}

// TestListProjectsIncludesFlowCount: a listed project carries the number of its flows; the count
// leaves out the flows of other projects and of other organizations.
func TestListProjectsIncludesFlowCount(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		f.flow(f.orgA, f.projA, "one")
		f.flow(f.orgA, f.projA, "two")
		f.flow(f.orgB, f.projB, "elsewhere")
		n, err := f.s.CountFlows(f.ctx(), scopeOf(f.orgA), f.projA)
		if err != nil || n != 2 {
			t.Errorf("flows of project alpha: %d, %v; want 2", n, err)
		}
		empty := f.project(f.orgA, "empty")
		if n, err := f.s.CountFlows(f.ctx(), scopeOf(f.orgA), empty); err != nil || n != 0 {
			t.Errorf("flows of an empty project: %d, %v; want 0", n, err)
		}
	})
}

// TestCurrentUserIncludesOrgRoles: the organization roles of the signed-in user name only the
// organizations where the user holds an organization-level role; a project-only member has none.
func TestCurrentUserIncludesOrgRoles(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		user := f.user("roles@example.test")
		f.member(f.orgA, user, identity.PrincipalID{})
		f.member(f.orgB, user, identity.PrincipalID{})
		bindOrg(t, f, user, identity.RoleAdmin)

		orgs, err := f.s.ResolveUserOrganizations(f.ctx(), user.ID, 100)
		if err != nil {
			t.Fatalf("resolve organizations: %v", err)
		}
		roles := map[identity.OrgID]string{}
		for i := range orgs {
			role, err := ev.OrganizationRole(f.ctx(), user.ID, orgs[i].Scope)
			if err != nil {
				t.Fatalf("organization role: %v", err)
			}
			if role != "" {
				roles[orgs[i].ID] = role
			}
		}
		if len(roles) != 1 || roles[f.orgA] != identity.RoleAdmin {
			t.Errorf("organization roles = %v, want Admin of organization A only", roles)
		}
	})
}
