// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The tests of SPEC-016 SH4 (criteria SH-PRJ-1 and SH-FLW-1). They run the services and the
// authorization of the project and flow operations over a real store of each dialect; the HTTP
// handlers that call them are tested in internal/adapter/httpapi (a test of the storage package
// cannot import the HTTP adapter).

// bindOrg gives user a role at the organization, with the row the identity area writes.
func bindOrg(t *testing.T, f *fixture, user identity.UserRecord, role string) {
	t.Helper()
	r := rawOf(t, f.s)
	mustExecOrg(t, r, f.orgA, `INSERT INTO role_binding (id, org_id, scope_kind, project_id, application_user_id, role_name, created_at, updated_at)
VALUES (?, ?, 'ORG', NULL, ?, ?, ?, ?)`, r.id([16]byte(next[identity.MembershipID](f))), r.id([16]byte(f.orgA)), r.id([16]byte(user.ID)), role, r.instant(base), r.instant(base))
}

// bindProject gives user a role in one project of the first organization.
func bindProject(t *testing.T, f *fixture, user identity.UserRecord, project identity.ProjectID, role string) {
	t.Helper()
	r := rawOf(t, f.s)
	mustExecOrg(t, r, f.orgA, `INSERT INTO role_binding (id, org_id, scope_kind, project_id, application_user_id, role_name, created_at, updated_at)
VALUES (?, ?, 'PROJECT', ?, ?, ?, ?, ?)`, r.id([16]byte(next[identity.MembershipID](f))), r.id([16]byte(f.orgA)), r.id([16]byte(project)), r.id([16]byte(user.ID)), role, r.instant(base), r.instant(base))
}

// actorOf is the audit actor of a user.
func actorOf(user identity.UserRecord) audit.Actor {
	return audit.Actor{PrincipalID: id.UUID(user.PrincipalID), Kind: audit.ActorUser}
}

// pendingActions lists the actions of the pending audit entries of an organization.
func pendingActions(t *testing.T, f *fixture, org identity.OrgID) []string {
	t.Helper()
	entries, err := f.s.ListPending(f.ctx(), audit.OrganizationChain(id.UUID(org)), 1000)
	if err != nil {
		t.Fatalf("list pending entries: %v", err)
	}
	out := make([]string, 0, len(entries))
	for i := range entries {
		out = append(out, entries[i].Action)
	}
	return out
}

func countOf(list []string, want string) int {
	n := 0
	for _, s := range list {
		if s == want {
			n++
		}
	}
	return n
}

// readableProjects is what listProjects shows user: the projects of the organization that the
// evaluator lets the user read.
func readableProjects(t *testing.T, f *fixture, ev *authz.Evaluator, org identity.OrgID, user identity.UserRecord) ([]identity.ProjectRecord, authz.ProjectAccess) {
	t.Helper()
	access, err := ev.ProjectAccess(f.ctx(), user.ID, scopeOf(org))
	if err != nil {
		t.Fatalf("project access of %s: %v", user.EmailNormalized, err)
	}
	rows, err := authz.ReadableProjects(f.ctx(), f.s, scopeOf(org), access, 2, 1000)
	if err != nil {
		t.Fatalf("readable projects of %s: %v", user.EmailNormalized, err)
	}
	return rows, access
}

func projectIDs(rows []identity.ProjectRecord) []identity.ProjectID {
	out := make([]identity.ProjectID, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].ID)
	}
	return out
}

func sameProjects(got []identity.ProjectID, want ...identity.ProjectID) bool {
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

// TestListProjectsOnlyReadable (R-SH-PRJ-01): listProjects shows only the projects the caller
// reads through project:read in the project or project:read-all; a member with neither sees none,
// a project of another organization is hidden, and the answer costs one grant lookup.
func TestListProjectsOnlyReadable(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		beta, gamma := f.project(f.orgA, "beta"), f.project(f.orgA, "gamma")
		all := []identity.ProjectID{f.projA, beta, gamma}

		plain := f.user("plain@example.test")   // Member: org:read, no project:read
		scoped := f.user("scoped@example.test") // Viewer in beta only
		viewer := f.user("viewer@example.test") // Viewer at the organization
		admin := f.user("admin@example.test")   // Admin: project:read-all
		for _, u := range []identity.UserRecord{plain, scoped, viewer, admin} {
			f.member(f.orgA, u, identity.PrincipalID{})
		}
		bindOrg(t, f, plain, authz.RoleMember)
		bindProject(t, f, scoped, beta, "Viewer")
		bindOrg(t, f, viewer, "Viewer")
		bindOrg(t, f, admin, identity.RoleAdmin)

		rows, _ := readableProjects(t, f, ev, f.orgA, plain)
		if len(rows) != 0 {
			t.Errorf("a Member without project:read sees %v, want no project", projectIDs(rows))
		}
		rows, access := readableProjects(t, f, ev, f.orgA, scoped)
		if got := projectIDs(rows); !sameProjects(got, beta) || access.ReadsEveryProject() {
			t.Errorf("a project Viewer sees %v (reads all %v), want only %v", got, access.ReadsEveryProject(), beta)
		}
		rows, access = readableProjects(t, f, ev, f.orgA, viewer)
		if got := projectIDs(rows); !sameProjects(got, all...) || access.ReadsEveryProject() {
			t.Errorf("an organization Viewer sees %v (reads all %v), want the three projects without project:read-all", got, access.ReadsEveryProject())
		}
		rows, access = readableProjects(t, f, ev, f.orgA, admin)
		if got := projectIDs(rows); !sameProjects(got, all...) || !access.ReadsEveryProject() {
			t.Errorf("an Admin sees %v (reads all %v), want the three projects and project:read-all", got, access.ReadsEveryProject())
		}
		for _, p := range rows {
			if p.ID == f.projB {
				t.Errorf("the listing of organization A shows the project of organization B")
			}
		}
		// A project of another organization is hidden from a member of this one (404).
		if d, _, err := ev.Authorize(f.ctx(), requestAs(scoped.ID, f.orgA, f.projB.String(), "project:read")); err != nil || d != authz.DecisionHidden {
			t.Errorf("a project of another organization: %v, %v; want hidden", d, err)
		}
	})
}

// TestListProjectsHandleFilter: the handle filter keeps the one project that holds the handle, an
// unknown handle and the handle of another organization's project keep none; the name prefix and
// environment filters narrow the same list.
func TestListProjectsHandleFilter(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		admin := f.user("admin@example.test")
		f.member(f.orgA, admin, identity.PrincipalID{})
		bindOrg(t, f, admin, identity.RoleAdmin)
		svc := identity.NewProjects(f.s, f.s, clock.NewFake(base), f.ids)
		for _, in := range []identity.NewProject{
			{Name: "Billing", Slug: "billing", Environment: "prod", By: actorOf(admin)},
			{Name: "Billing sandbox", Slug: "billing-sandbox", Environment: "test", By: actorOf(admin)},
			{Name: "Support", Slug: "support", By: actorOf(admin)},
		} {
			if _, err := svc.Create(f.ctx(), scopeOf(f.orgA), in); err != nil {
				t.Fatalf("create %s: %v", in.Slug, err)
			}
		}
		rows, _ := readableProjects(t, f, ev, f.orgA, admin)
		match := func(filter identity.ProjectFilter) []string {
			var out []string
			for i := range rows {
				if filter.Match(rows[i]) {
					out = append(out, rows[i].Handle)
				}
			}
			slices.Sort(out)
			return out
		}
		if got := match(identity.ProjectFilter{Handle: "billing"}); !slices.Equal(got, []string{"billing"}) {
			t.Errorf("handle billing = %v, want exactly it", got)
		}
		if got := match(identity.ProjectFilter{Handle: "nobody"}); len(got) != 0 {
			t.Errorf("an unknown handle = %v, want none", got)
		}
		otherOrg, err := f.s.GetProject(f.ctx(), scopeOf(f.orgB), f.projB)
		if err != nil {
			t.Fatalf("get the project of B: %v", err)
		}
		if got := match(identity.ProjectFilter{Handle: otherOrg.Handle}); len(got) != 0 {
			t.Errorf("the handle of a project of another organization = %v, want none", got)
		}
		if got := match(identity.ProjectFilter{NamePrefix: "bill"}); !slices.Equal(got, []string{"billing", "billing-sandbox"}) {
			t.Errorf("name prefix bill = %v, want both billing projects", got)
		}
		if got := match(identity.ProjectFilter{NamePrefix: "bill", Environment: "test"}); !slices.Equal(got, []string{"billing-sandbox"}) {
			t.Errorf("prefix bill in test = %v, want the sandbox", got)
		}
	})
}

// TestCreateProjectDerivesHandle (SPEC-004 R-ID-11): the handle is the slug; when another project
// of the instance holds it, the handle is "<orgSlug>-<slug>"; the project is always protected.
func TestCreateProjectDerivesHandle(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		svc := identity.NewProjects(f.s, f.s, clock.NewFake(base), f.ids)
		first, err := svc.Create(f.ctx(), scopeOf(f.orgA), identity.NewProject{Name: "Ops", Slug: "ops"})
		if err != nil {
			t.Fatalf("create in A: %v", err)
		}
		if first.Handle != "ops" || !first.IsProtected {
			t.Errorf("first project = handle %q protected %v, want ops and protected", first.Handle, first.IsProtected)
		}
		second, err := svc.Create(f.ctx(), scopeOf(f.orgB), identity.NewProject{Name: "Ops", Slug: "ops"})
		if err != nil {
			t.Fatalf("create in B with a slug held by A: %v", err)
		}
		if second.Handle != "org-b-ops" || second.Slug != "ops" || !second.IsProtected {
			t.Errorf("second project = handle %q slug %q protected %v, want org-b-ops, ops, protected", second.Handle, second.Slug, second.IsProtected)
		}
		stored, err := f.s.GetProject(f.ctx(), scopeOf(f.orgB), second.ID)
		if err != nil || stored.Handle != "org-b-ops" || !stored.IsProtected {
			t.Errorf("stored project = %+v, %v; want the derived handle, protected", stored, err)
		}
	})
}

// TestCreateProjectHandleUnavailableIsGeneric (SPEC-004 R-ID-11): when neither candidate handle is
// free, the error is the generic one and its text names nothing of the project or the organization
// that hold the handle.
func TestCreateProjectHandleUnavailableIsGeneric(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		svc := identity.NewProjects(f.s, f.s, clock.NewFake(base), f.ids)
		for _, in := range []identity.NewProject{{Name: "Ops", Slug: "ops"}, {Name: "Taken", Slug: "org-b-ops"}} {
			if _, err := svc.Create(f.ctx(), scopeOf(f.orgA), in); err != nil {
				t.Fatalf("create %s in A: %v", in.Slug, err)
			}
		}
		_, err := svc.Create(f.ctx(), scopeOf(f.orgB), identity.NewProject{Name: "Mine", Slug: "ops"})
		if !errors.Is(err, identity.ErrHandleUnavailable) {
			t.Fatalf("create with both handles held = %v, want ErrHandleUnavailable", err)
		}
		for _, leak := range []string{"ops", "org-a", "org-b", "Taken", f.orgA.String(), f.orgB.String()} {
			if strings.Contains(err.Error(), leak) {
				t.Errorf("the error %q names %q", err, leak)
			}
		}
		if rows, _ := f.s.ListProjects(f.ctx(), scopeOf(f.orgB), identity.ProjectID{}, 100); len(rows) != 1 {
			t.Errorf("organization B holds %d projects after the refusal, want only the fixture project", len(rows))
		}
	})
}

// TestCreateProjectGrantsCreatorProjectAdmin (SPEC-016 Q3 = A, R-SH-PRJ-03): the creator becomes
// Project Admin of the new project and of no other, the binding is audited with the project, and
// a creator who is not named gets no binding.
func TestCreateProjectGrantsCreatorProjectAdmin(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		maker := f.user("maker@example.test")
		f.member(f.orgA, maker, identity.PrincipalID{})
		bindOrg(t, f, maker, authz.RoleMember) // Member holds project:create and nothing of project:read
		svc := identity.NewProjects(f.s, f.s, clock.NewFake(base), f.ids)
		scope := scopeOf(f.orgA)

		before := pendingActions(t, f, f.orgA)
		rec, err := svc.Create(f.ctx(), scope, identity.NewProject{Name: "Ops", Slug: "ops", By: actorOf(maker), Creator: maker.ID})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		bindings, err := f.s.Bindings(f.ctx(), maker.ID, scope)
		if err != nil {
			t.Fatalf("bindings: %v", err)
		}
		want := authz.Binding{Role: identity.RoleProjectAdmin, Project: rec.ID.String()}
		if !slices.Contains(bindings, want) {
			t.Fatalf("bindings = %+v, want Project Admin in the new project", bindings)
		}
		access, err := ev.ProjectAccess(f.ctx(), maker.ID, scope)
		if err != nil {
			t.Fatal(err)
		}
		if !access.CanRead(rec.ID.String()) || access.CanRead(f.projA.String()) {
			t.Errorf("the creator reads the new project %v and the other %v; want true and false", access.CanRead(rec.ID.String()), access.CanRead(f.projA.String()))
		}
		after := pendingActions(t, f, f.orgA)
		if countOf(after, "project.created")-countOf(before, "project.created") != 1 ||
			countOf(after, "role_binding.created")-countOf(before, "role_binding.created") != 1 {
			t.Errorf("audit entries before %v and after %v; want one project.created and one role_binding.created more", before, after)
		}

		// Without a creator (the caller already reads every project) nothing is bound or audited as bound.
		if _, err := svc.Create(f.ctx(), scope, identity.NewProject{Name: "Plain", Slug: "plain", By: actorOf(maker)}); err != nil {
			t.Fatalf("create without a creator: %v", err)
		}
		if again, _ := f.s.Bindings(f.ctx(), maker.ID, scope); len(again) != len(bindings) {
			t.Errorf("a creation without a creator changed the bindings: %+v", again)
		}
		if final := pendingActions(t, f, f.orgA); countOf(final, "role_binding.created") != countOf(after, "role_binding.created") {
			t.Errorf("a creation without a creator wrote role_binding.created")
		}
	})
}

// failingEntryProjects is a project store whose creation fails after every row is written but
// before the commit: the last audit entry is written twice, and the second write violates the
// identity of the pending entry.
type failingEntryProjects struct{ storetest.Store }

func (s failingEntryProjects) CreateProjectWithGrant(ctx context.Context, org orgscope.Org, p identity.ProjectRecord, grant identity.RoleBindingRecord, entries []audit.Entry) error {
	return s.Store.CreateProjectWithGrant(ctx, org, p, grant, append(slices.Clone(entries), entries[len(entries)-1]))
}

// TestCreateProjectAuditedAtomically (R-SH-PRJ-03): a failure before the commit leaves no project,
// no role binding, no audit entry and no epoch change.
func TestCreateProjectAuditedAtomically(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		maker := f.user("maker@example.test")
		f.member(f.orgA, maker, identity.PrincipalID{})
		scope := scopeOf(f.orgA)
		epochBefore, err := f.s.Epoch(f.ctx(), scope)
		if err != nil {
			t.Fatalf("epoch: %v", err)
		}
		pendingBefore := pendingActions(t, f, f.orgA)

		svc := identity.NewProjects(failingEntryProjects{f.s}, f.s, clock.NewFake(base), f.ids)
		_, err = svc.Create(f.ctx(), scope, identity.NewProject{Name: "Doomed", Slug: "doomed", By: actorOf(maker), Creator: maker.ID})
		if err == nil {
			t.Fatal("the creation with a failure before the commit succeeded")
		}
		t.Logf("failure injected before the commit: %v", err)
		rows, err := f.s.ListProjects(f.ctx(), scope, identity.ProjectID{}, 100)
		if err != nil {
			t.Fatal(err)
		}
		for i := range rows {
			if rows[i].Slug == "doomed" {
				t.Errorf("the project survived the failed transaction")
			}
		}
		if bindings, _ := f.s.Bindings(f.ctx(), maker.ID, scope); len(bindings) != 0 {
			t.Errorf("the binding survived the failed transaction: %+v", bindings)
		}
		if pendingAfter := pendingActions(t, f, f.orgA); !slices.Equal(pendingBefore, pendingAfter) {
			t.Errorf("audit entries before %v and after %v; want the same: no event", pendingBefore, pendingAfter)
		}
		if epochAfter, _ := f.s.Epoch(f.ctx(), scope); epochAfter != epochBefore {
			t.Errorf("epoch moved from %d to %d in a failed transaction", epochBefore, epochAfter)
		}
	})
}

// listedFlows returns the flows of a project through the store, as listFlows reads them.
func listedFlows(t *testing.T, f *fixture, org identity.OrgID, project identity.ProjectID) []flow.Record {
	t.Helper()
	rows, err := f.s.ListFlows(f.ctx(), scopeOf(org), project, flow.FlowID{}, 100)
	if err != nil {
		t.Fatalf("list flows: %v", err)
	}
	return rows
}

// TestListFlowsFieldsAndDefaultSort (SPEC-016 SH-FLW-1): a listed flow carries its identity, name,
// activation state and update time at the precision the default sort needs, and ordering by
// update time, newest first, puts the last changed flow first; the activation filter keeps the
// matching flows.
func TestListFlowsFieldsAndDefaultSort(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		scope := scopeOf(f.orgA)
		for i, name := range []string{"oldest", "middle", "newest"} {
			at := base.Add(time.Duration(i) * time.Second).Truncate(time.Microsecond)
			rec := flow.Record{
				ID: next[flow.FlowID](f), OrgID: f.orgA, ProjectID: f.projA, Name: name, Draft: []byte(`{"format":"orcon/flow@1"}`),
				ActivationState: flow.ActivationInactive, CreatedAt: at, UpdatedAt: at,
			}
			if err := f.s.CreateFlowWithEntry(f.ctx(), scope, rec, audittest.Org(t, id.UUID(f.orgA), "flow.created")); err != nil {
				t.Fatalf("create %s: %v", name, err)
			}
		}
		rows := listedFlows(t, f, f.orgA, f.projA)
		if len(rows) != 3 {
			t.Fatalf("listed %d flows, want 3", len(rows))
		}
		for i := range rows {
			if rows[i].ID.IsZero() || rows[i].Name == "" || rows[i].ActivationState != flow.ActivationInactive || rows[i].UpdatedAt.IsZero() {
				t.Errorf("flow %d = %+v, want id, name, INACTIVE and an update time", i, rows[i])
			}
		}
		slices.SortFunc(rows, func(a, b flow.Record) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
		var order []string
		for i := range rows {
			order = append(order, rows[i].Name)
		}
		if !slices.Equal(order, []string{"newest", "middle", "oldest"}) {
			t.Errorf("order by -updatedAt = %v, want newest first", order)
		}
		filter := flow.ListFilter{ActivationState: flow.ActivationActive}
		for i := range rows {
			if filter.Match(rows[i]) {
				t.Errorf("flow %q passes the ACTIVE filter; new flows are inactive", rows[i].Name)
			}
		}
		if got := (flow.ListFilter{NamePrefix: "MID", ActivationState: flow.ActivationInactive}); !got.Match(rows[1]) {
			t.Errorf("the filter %+v does not keep %q", got, rows[1].Name)
		}
	})
}

// TestListFlowsOtherOrganizationNotFound (ADR-0041 section 6): the flows of a project of another
// organization are neither listed nor readable, and the evaluator hides that project, which the
// API answers as 404.
func TestListFlowsOtherOrganizationNotFound(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		viewer := f.user("viewer@example.test")
		f.member(f.orgA, viewer, identity.PrincipalID{})
		bindOrg(t, f, viewer, "Viewer")
		foreign := f.flow(f.orgB, f.projB, "secret plans")

		if d, _, err := ev.Authorize(f.ctx(), requestAs(viewer.ID, f.orgA, f.projB.String(), "flow:read")); err != nil || d != authz.DecisionHidden {
			t.Errorf("flows of a project of another organization: %v, %v; want hidden", d, err)
		}
		if rows := listedFlows(t, f, f.orgA, f.projB); len(rows) != 0 {
			t.Errorf("organization A lists %d flows of a project of B, want none", len(rows))
		}
		if _, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), foreign.ID); !errors.Is(err, identity.ErrNotFound) && !errors.Is(err, flow.ErrNotFound) {
			t.Errorf("get a flow of B under the scope of A = %v, want not found", err)
		}
	})
}

// TestCreateFlowStartsInactiveWithEmptyDraft (ADR-0080 section 3): a new flow is inactive, has no
// published version, and its draft is the empty document of the format at revision zero; the
// creation writes flow.created.
func TestCreateFlowStartsInactiveWithEmptyDraft(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		maker := f.user("maker@example.test")
		scope := scopeOf(f.orgA)
		before := pendingActions(t, f, f.orgA)
		creator := flow.NewCreator(f.s, clock.NewFake(base), f.ids)
		rec, err := creator.Create(f.ctx(), scope, flow.CreateInput{ProjectID: f.projA, Name: "Welcome", By: actorOf(maker)})
		if err != nil {
			t.Fatalf("create flow: %v", err)
		}
		got, err := f.s.GetFlow(f.ctx(), scope, rec.ID)
		if err != nil {
			t.Fatalf("get flow: %v", err)
		}
		if got.ActivationState != flow.ActivationInactive || got.DraftRevision != 0 || !got.ActiveVersionID.IsZero() {
			t.Errorf("flow = %+v, want inactive, draft revision 0, no published version", got)
		}
		for _, part := range []string{`"format":"orcon/flow@1"`, `"nodes":[]`, `"edges":[]`, `"variables":[]`, `"name":"Welcome"`} {
			if !bytes.Contains(got.Draft, []byte(part)) {
				t.Errorf("draft %s lacks %s", got.Draft, part)
			}
		}
		after := pendingActions(t, f, f.orgA)
		if countOf(after, "flow.created")-countOf(before, "flow.created") != 1 {
			t.Errorf("audit entries before %v and after %v; want one flow.created more", before, after)
		}
	})
}

// failingEntryFlows is a flow store whose creation reuses the identity of an entry that is
// already pending, so the entry is refused after the flow row was written and before the commit.
type failingEntryFlows struct {
	storetest.Store
	reuse audit.Entry
}

func (s failingEntryFlows) CreateFlowWithEntry(ctx context.Context, org orgscope.Org, f flow.Record, _ audit.Entry) error {
	entry := s.reuse
	return s.Store.CreateFlowWithEntry(ctx, org, f, entry)
}

// TestCreateFlowAuditedAtomically (SPEC-016 SH-FLW-1): a failure before the commit leaves no flow
// and no audit entry.
func TestCreateFlowAuditedAtomically(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		maker := f.user("maker@example.test")
		scope := scopeOf(f.orgA)
		taken := audittest.Org(t, id.UUID(f.orgA), "flow.created")
		first := flow.Record{
			ID: next[flow.FlowID](f), OrgID: f.orgA, ProjectID: f.projA, Name: "First", Draft: []byte(`{"format":"orcon/flow@1"}`),
			ActivationState: flow.ActivationInactive, CreatedAt: base, UpdatedAt: base,
		}
		if err := f.s.CreateFlowWithEntry(f.ctx(), scope, first, taken); err != nil {
			t.Fatalf("seed flow: %v", err)
		}
		pendingBefore := pendingActions(t, f, f.orgA)
		flowsBefore := len(listedFlows(t, f, f.orgA, f.projA))

		creator := flow.NewCreator(failingEntryFlows{Store: f.s, reuse: taken}, clock.NewFake(base), f.ids)
		if _, err := creator.Create(f.ctx(), scope, flow.CreateInput{ProjectID: f.projA, Name: "Doomed", By: actorOf(maker)}); err == nil {
			t.Fatal("the creation with a failure before the commit succeeded")
		} else {
			t.Logf("failure injected before the commit: %v", err)
		}
		for _, rec := range listedFlows(t, f, f.orgA, f.projA) {
			if rec.Name == "Doomed" {
				t.Errorf("the flow survived the failed transaction")
			}
		}
		if got := len(listedFlows(t, f, f.orgA, f.projA)); got != flowsBefore {
			t.Errorf("flows = %d after the failed creation, want %d", got, flowsBefore)
		}
		if pendingAfter := pendingActions(t, f, f.orgA); !slices.Equal(pendingBefore, pendingAfter) {
			t.Errorf("audit entries before %v and after %v; want the same: no event", pendingBefore, pendingAfter)
		}
	})
}

// TestOrganizationsCreateAuditsBindingAndMembership (BL-129, SPEC-004 Annex B): creating an
// organization from the operator's command writes org.created, member.joined for the
// owner and role_binding.created for the Owner binding, in the same write as the rows;
// it does not write member.role_changed, which means replacing a role.
func TestOrganizationsCreateAuditsBindingAndMembership(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		owner := f.user("boss@example.test")
		kr, _ := keyringOf(t, f)
		orgs := identity.NewOrganizations(f.s, keyringIssuer{kr: kr}, clock.NewFake(base), f.ids)
		org, err := orgs.Create(f.ctx(), identity.NewOrganization{Name: "Fresh", Slug: "fresh", OwnerEmail: "boss@example.test"})
		if err != nil {
			t.Fatalf("create organization: %v", err)
		}
		got := pendingActions(t, f, org.ID)
		slices.Sort(got)
		want := []string{"member.joined", "org.created", "role_binding.created"}
		if !slices.Equal(got, want) {
			t.Errorf("audit actions = %v, want %v", got, want)
		}
		bindings, err := f.s.Bindings(f.ctx(), owner.ID, scopeOf(org.ID))
		if err != nil || len(bindings) != 1 || bindings[0].Role != identity.RoleOwner || bindings[0].Project != "" {
			t.Errorf("bindings = %+v, %v; want the organization Owner binding (the creator of an organization is its Owner)", bindings, err)
		}
	})
}
