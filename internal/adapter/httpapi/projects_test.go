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
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// projectsRig is the project and flow handlers over the in-memory store, for one organization
// and one signed-in user, with the scope the authorization step would have issued.
type projectsRig struct {
	t     *testing.T
	clk   *clock.Fake
	h     *Projects
	ctx   context.Context //nolint:containedctx // the rig carries the request context of every call it makes
	org   identity.OrgID
	scope orgscope.Org
	user  identity.UserID
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
	h := NewProjects(
		identity.NewProjects(mem, mem, clk, ids), mem,
		flow.NewCreator(mem, clk, ids), mem, codec,
	)
	return &projectsRig{t: t, clk: clk, h: h, ctx: ctx, org: org, scope: scope, user: user}
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
