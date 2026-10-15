// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// fakeAccess answers the project access of a test: every project, unless a test sets another one.
type fakeAccess struct{ access *authz.ProjectAccess }

func (f *fakeAccess) ProjectAccess(context.Context, identity.UserID, orgscope.Org) (authz.ProjectAccess, error) {
	if f.access != nil {
		return *f.access, nil
	}
	return authz.ProjectAccessOf(authz.Grant{Everywhere: []authz.Permission{"project:read-all"}}), nil
}

// recordingProjects is the project store of a test that remembers the bindings and audit entries
// the identity area asked it to write together with a project.
type recordingProjects struct {
	*triggertest.Memory
	grants  []identity.RoleBindingRecord
	entries [][]audit.Entry
}

func (s *recordingProjects) CreateProjectWithGrant(ctx context.Context, org orgscope.Org, p identity.ProjectRecord, grant identity.RoleBindingRecord, entries []audit.Entry) error {
	s.grants = append(s.grants, grant)
	s.entries = append(s.entries, entries)
	return s.Memory.CreateProjectWithGrant(ctx, org, p, grant, entries)
}

// projectsRig is the project and flow handlers over the in-memory store, for one organization
// and one signed-in user, with the scope the authorization step would have issued.
type projectsRig struct {
	access *fakeAccess
	grants *recordingProjects
	t      *testing.T
	clk    *clock.Fake
	h      *Projects
	ctx    context.Context //nolint:containedctx // the rig carries the request context of every call it makes
	org    identity.OrgID
	scope  orgscope.Org
	user   identity.UserID
}

func newProjectsRig(t *testing.T) *projectsRig {
	t.Helper()
	ids := id.NewSequence()
	clk := clock.NewFake(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC))
	mem := triggertest.NewMemory()
	org := identity.OrgID(ids.New())
	if err := mem.CreateOrganization(t.Context(), identity.OrgRecord{ID: org, Name: "Acme", Slug: "acme", Status: identity.OrgActive, CreatedAt: clk.Now(), UpdatedAt: clk.Now()}); err != nil {
		t.Fatal(err)
	}
	scope, err := issue.New(id.UUID(org))
	if err != nil {
		t.Fatal(err)
	}
	user := identity.UserID(ids.New())
	codec, err := page.NewCodec(make([]byte, page.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{ID: identity.PrincipalID(ids.New()), Kind: identity.PrincipalUser, UserID: user, CredentialType: identity.CredentialSession}
	ctx := middleware.WithPrincipal(t.Context(), principal)
	ctx = authz.WithOrgScope(ctx, scope)
	access := &fakeAccess{}
	grants := &recordingProjects{Memory: mem}
	h := NewProjects(
		access, identity.NewProjects(grants, mem, clk, ids), mem,
		flow.NewCreator(mem, clk, ids), mem, codec,
	)
	return &projectsRig{access: access, grants: grants, t: t, clk: clk, h: h, ctx: ctx, org: org, scope: scope, user: user}
}

// projectsOf returns the body of a 200 listing of projects.
func projectsOf(t *testing.T, resp apigen.ListProjectsResponseObject) apigen.ProjectList {
	t.Helper()
	out, ok := resp.(apigen.ListProjects200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	return out.Body
}

// flowsOf returns the body of a 200 listing of flows.
func flowsOf(t *testing.T, resp apigen.ListFlowsResponseObject) apigen.FlowList {
	t.Helper()
	out, ok := resp.(apigen.ListFlows200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	return out.Body
}

func (r *projectsRig) createProject(name, slug string) (apigen.CreateProject201JSONResponse, error) {
	r.t.Helper()
	body := apigen.CreateProjectRequest{Name: name}
	if slug != "" {
		body.Slug = &slug
	}
	resp, err := r.h.CreateProject(r.ctx, apigen.CreateProjectRequestObject{OrgId: types.UUID(r.org), Body: &body})
	if err != nil {
		return apigen.CreateProject201JSONResponse{}, err
	}
	out, ok := resp.(apigen.CreateProject201JSONResponse)
	if !ok {
		r.t.Fatalf("response = %T, want 201", resp)
	}
	return out, nil
}

// TestSlugOfDerivesTheSlugOfAName: the slug of a name without a slug is lowercase letters and digits
// separated by single hyphens; a name with none of them has an empty slug.
func TestSlugOfDerivesTheSlugOfAName(t *testing.T) {
	cases := map[string]string{
		"Customer onboarding": "customer-onboarding",
		"  Prod -- 2026  ":    "prod-2026",
		"Ñandú":               "and",
		"!!!":                 "",
	}
	for name, want := range cases {
		if got := slugOf(name); got != want {
			t.Errorf("slugOf(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestCreateProjectDerivesTheHandleAndAnswers201: the handle of a new project is its slug, the answer
// is 201 with the location of the project, and the project is protected unless the caller says otherwise.
func TestCreateProjectDerivesTheHandleAndAnswers201(t *testing.T) {
	r := newProjectsRig(t)
	resp, err := r.createProject("Customer onboarding", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if resp.Body.Handle != "customer-onboarding" || resp.Body.Slug != "customer-onboarding" || !resp.Body.Protected {
		t.Errorf("project = %+v, want handle and slug from the name, protected", resp.Body)
	}
	if resp.Headers.Location == nil || *resp.Headers.Location != "/api/v1/projects/"+resp.Body.Id.String() {
		t.Errorf("location = %v, want the project's address", resp.Headers.Location)
	}
}

// TestCreateProjectRefusesABadSlug: a slug outside the lowercase form is a validation error, not a
// stored project.
func TestCreateProjectRefusesABadSlug(t *testing.T) {
	r := newProjectsRig(t)
	if _, err := r.createProject("Bad", "Not A Slug"); !errors.Is(err, identity.ErrInvalidInput) {
		t.Errorf("a bad slug = %v, want ErrInvalidInput", err)
	}
}

// TestCreateProjectSameSlugIsAGenericConflict: a second project with a slug the organization already
// holds is a conflict whose message names nothing of anyone else.
func TestCreateProjectSameSlugIsAGenericConflict(t *testing.T) {
	r := newProjectsRig(t)
	if _, err := r.createProject("First", "shared"); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := r.createProject("Second", "shared")
	if !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("second = %v, want a conflict", err)
	}
}

// TestCreateProjectWithoutAuthorizedScopeIsRefused: a handler reached without the authorization step
// never runs: the request has no organization scope to write into.
func TestCreateProjectWithoutAuthorizedScopeIsRefused(t *testing.T) {
	r := newProjectsRig(t)
	bare := middleware.WithPrincipal(t.Context(), identity.Principal{UserID: r.user})
	slug := "bare"
	_, err := r.h.CreateProject(bare, apigen.CreateProjectRequestObject{OrgId: types.UUID(r.org), Body: &apigen.CreateProjectRequest{Name: "Bare", Slug: &slug}})
	if !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("without a scope = %v, want ErrUnauthenticated", err)
	}
}

// TestListProjectsPagesByNameAndFiltersByPrefix: the listing is ordered by name, pages with a token
// bound to its query, and filters by name prefix.
func TestListProjectsPagesByNameAndFiltersByPrefix(t *testing.T) {
	r := newProjectsRig(t)
	for _, n := range []string{"Gamma", "Alpha", "Beta", "Alphabet"} {
		if _, err := r.createProject(n, ""); err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
	}
	limit := 2
	first, err := r.h.ListProjects(r.ctx, apigen.ListProjectsRequestObject{OrgId: types.UUID(r.org), Params: apigen.ListProjectsParams{Limit: &limit}})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	page1 := projectsOf(t, first)
	if len(page1.Items) != 2 || page1.Items[0].Name != "Alpha" || page1.Items[1].Name != "Alphabet" || page1.NextPageToken == nil {
		t.Fatalf("first page = %+v, want Alpha and Alphabet with a next token", page1)
	}
	second, err := r.h.ListProjects(r.ctx, apigen.ListProjectsRequestObject{OrgId: types.UUID(r.org), Params: apigen.ListProjectsParams{Limit: &limit, PageToken: page1.NextPageToken}})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	page2 := projectsOf(t, second)
	if len(page2.Items) != 2 || page2.Items[0].Name != "Beta" || page2.Items[1].Name != "Gamma" || page2.NextPageToken != nil {
		t.Fatalf("second page = %+v, want Beta and Gamma with no next token", page2)
	}

	q := "al"
	filtered, err := r.h.ListProjects(r.ctx, apigen.ListProjectsRequestObject{OrgId: types.UUID(r.org), Params: apigen.ListProjectsParams{Q: &q}})
	if err != nil {
		t.Fatalf("filtered: %v", err)
	}
	if got := projectsOf(t, filtered).Items; len(got) != 2 {
		t.Errorf("prefix al = %d projects, want Alpha and Alphabet", len(got))
	}
}

// TestPageTokenOfAnotherQueryIsRefused: a token issued for one sort or filter is invalid for another.
func TestPageTokenOfAnotherQueryIsRefused(t *testing.T) {
	r := newProjectsRig(t)
	for _, n := range []string{"One", "Two", "Three"} {
		if _, err := r.createProject(n, ""); err != nil {
			t.Fatal(err)
		}
	}
	limit := 1
	first, err := r.h.ListProjects(r.ctx, apigen.ListProjectsRequestObject{OrgId: types.UUID(r.org), Params: apigen.ListProjectsParams{Limit: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	token := projectsOf(t, first).NextPageToken
	if token == nil {
		t.Fatal("no next token")
	}
	desc := apigen.ListProjectsParamsSort("-name")
	_, err = r.h.ListProjects(r.ctx, apigen.ListProjectsRequestObject{OrgId: types.UUID(r.org), Params: apigen.ListProjectsParams{Limit: &limit, Sort: &desc, PageToken: token}})
	if !errors.Is(err, page.ErrInvalidToken) {
		t.Errorf("a token of another sort = %v, want page.ErrInvalidToken", err)
	}
}

// TestCreateFlowStartsInactiveWithAnEmptyDraft (ADR-0080 section 3): a new flow is inactive, its
// draft is revision zero with the empty document, and its entity tag is that revision.
func TestCreateFlowStartsInactiveWithAnEmptyDraft(t *testing.T) {
	r := newProjectsRig(t)
	project, err := r.createProject("Ops", "")
	if err != nil {
		t.Fatal(err)
	}
	pid := *project.Body.Id
	resp, err := r.h.CreateFlow(r.ctx, apigen.CreateFlowRequestObject{ProjectId: pid, Body: &apigen.CreateFlowRequest{Name: "Welcome"}})
	if err != nil {
		t.Fatalf("create flow: %v", err)
	}
	out, ok := resp.(apigen.CreateFlow201JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 201", resp)
	}
	if out.Body.ActivationState != apigen.FlowActivationState("INACTIVE") || out.Body.DraftRevision != 0 || out.Body.HasUnpublishedChanges {
		t.Errorf("flow = %+v, want an inactive flow at draft revision 0", out.Body)
	}
	if out.Headers.ETag == nil || *out.Headers.ETag != `"0"` {
		t.Errorf("etag = %v, want \"0\"", out.Headers.ETag)
	}
}

// TestListFlowsFiltersByActivationAndSortsNewestFirst: the default order is the update time, newest
// first, and the activation filter keeps the inactive flows.
func TestListFlowsFiltersByActivationAndSortsNewestFirst(t *testing.T) {
	r := newProjectsRig(t)
	project, err := r.createProject("Ops", "")
	if err != nil {
		t.Fatal(err)
	}
	pid := *project.Body.Id
	for _, n := range []string{"First", "Second"} {
		r.clk.Advance(time.Second)
		if _, err := r.h.CreateFlow(r.ctx, apigen.CreateFlowRequestObject{ProjectId: pid, Body: &apigen.CreateFlowRequest{Name: n}}); err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
	}
	state := apigen.ListFlowsParamsActivationState("ACTIVE")
	active, err := r.h.ListFlows(r.ctx, apigen.ListFlowsRequestObject{ProjectId: pid, Params: apigen.ListFlowsParams{ActivationState: &state}})
	if err != nil {
		t.Fatal(err)
	}
	if got := flowsOf(t, active).Items; len(got) != 0 {
		t.Errorf("active flows = %d, want none: new flows are inactive", len(got))
	}
	all, err := r.h.ListFlows(r.ctx, apigen.ListFlowsRequestObject{ProjectId: pid})
	if err != nil {
		t.Fatal(err)
	}
	items := flowsOf(t, all).Items
	if len(items) != 2 || items[0].Name != "Second" || items[1].Name != "First" {
		t.Errorf("default order = %+v, want the newest flow first", items)
	}
}

// TestCreateProjectIsAlwaysProtected (SH-PRJ-1, ADR-0059 section 5): a body that asks for an
// unprotected project still gets a protected one.
func TestCreateProjectIsAlwaysProtected(t *testing.T) {
	r := newProjectsRig(t)
	no := false
	slug := "open"
	resp, err := r.h.CreateProject(r.ctx, apigen.CreateProjectRequestObject{OrgId: types.UUID(r.org), Body: &apigen.CreateProjectRequest{Name: "Open", Slug: &slug, Protected: &no}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	out, ok := resp.(apigen.CreateProject201JSONResponse)
	if !ok || !out.Body.Protected {
		t.Fatalf("response = %+v, want 201 with protected true", resp)
	}
	stored, err := r.h.store.GetProject(r.ctx, r.scope, identity.ProjectID(*out.Body.Id))
	if err != nil || !stored.IsProtected {
		t.Errorf("stored project = %+v, %v; want protected", stored, err)
	}
}

// TestCreateProjectBindsOnlyWhoCannotReadEveryProject (R-SH-PRJ-03, SPEC-016 Q3 = A): a creator
// without project:read-all becomes Project Admin of the project, with the binding audited next to
// the creation; an Owner, an Admin or any holder of project:read-all gets no binding.
func TestCreateProjectBindsOnlyWhoCannotReadEveryProject(t *testing.T) {
	r := newProjectsRig(t)
	none := authz.ProjectAccessOf(authz.Grant{Everywhere: []authz.Permission{"project:create"}})
	r.access.access = &none
	created, err := r.createProject("Mine", "mine")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.grants.grants) != 1 {
		t.Fatalf("grants = %d, want one binding for a creator who cannot read every project", len(r.grants.grants))
	}
	g := r.grants.grants[0]
	if g.UserID != r.user || g.RoleName != identity.RoleProjectAdmin || g.ProjectID != identity.ProjectID(*created.Body.Id) || g.ScopeKind != identity.RoleScopeProject {
		t.Errorf("binding = %+v, want Project Admin of the new project for the creator", g)
	}
	var actions []string
	for _, e := range r.grants.entries[0] {
		actions = append(actions, e.Action)
	}
	if len(actions) != 2 || actions[0] != "project.created" || actions[1] != "role_binding.created" {
		t.Errorf("audit actions = %v, want project.created and role_binding.created in the same write", actions)
	}

	all := authz.ProjectAccessOf(authz.Grant{Everywhere: []authz.Permission{"project:read-all"}})
	r.access.access = &all
	if _, err := r.createProject("Theirs", "theirs"); err != nil {
		t.Fatal(err)
	}
	if len(r.grants.grants) != 1 {
		t.Errorf("grants = %d after a creator with project:read-all, want still 1", len(r.grants.grants))
	}
}

// TestListProjectsShowsOnlyReadable (R-SH-PRJ-01): the listing keeps the projects the caller
// reads, and nothing when the caller reads none.
func TestListProjectsShowsOnlyReadable(t *testing.T) {
	r := newProjectsRig(t)
	var mine apigen.CreateProject201JSONResponse
	for _, n := range []string{"alpha", "beta", "gamma"} {
		out, err := r.createProject(n, n)
		if err != nil {
			t.Fatal(err)
		}
		if n == "beta" {
			mine = out
		}
	}
	list := func() []apigen.Project {
		resp, err := r.h.ListProjects(r.ctx, apigen.ListProjectsRequestObject{OrgId: types.UUID(r.org)})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		return projectsOf(t, resp).Items
	}
	if got := list(); len(got) != 3 {
		t.Fatalf("a caller with project:read-all sees %d projects, want 3", len(got))
	}
	one := authz.ProjectAccessOf(authz.Grant{PerProject: map[string][]authz.Permission{mine.Body.Id.String(): {"project:read"}}})
	r.access.access = &one
	if got := list(); len(got) != 1 || got[0].Name != "beta" {
		t.Errorf("a caller with project:read in one project sees %+v, want only beta", got)
	}
	nothing := authz.ProjectAccessOf(authz.Grant{Everywhere: []authz.Permission{"org:read"}})
	r.access.access = &nothing
	if got := list(); len(got) != 0 {
		t.Errorf("a caller with org:read only sees %d projects, want none", len(got))
	}
}
