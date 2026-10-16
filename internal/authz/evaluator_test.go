// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// testCatalog is the part of the permission catalog the role rules use.
func testCatalog(t *testing.T) *authz.Catalog {
	t.Helper()
	cat, err := authz.NewCatalog(map[authz.Scope][]authz.Permission{
		authz.ScopeOrganization: {"org:read", "org:update", "org:delete", "member:read", "role:read", "project:create", "secret-store:reference"},
		authz.ScopeProject: {
			"project:read", "project:update", "project:manage-members",
			"flow:read", "flow:create", "flow:update", "flow:delete", "flow:move", "flow:publish", "flow:activate", "flow:rollback", "flow:execute",
			"execution:read", "execution:read-data", "execution:retry", "execution:cancel", "execution:resolve", "execution:delete",
			"human-task:resolve", "credential:read", "credential:create", "credential:update", "credential:delete", "credential:use", "credential:share",
			"variable:read", "variable:manage",
		},
	})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return cat
}

// fakeStore is an in-memory authorization store that counts how often it is asked for bindings.
type fakeStore struct {
	epoch     int64
	active    bool
	bindings  []authz.Binding
	custom    map[string][]authz.Permission
	projects  map[string]identity.OrgID
	children  map[string]childRef // by kind + "/" + id
	bindCalls int
}

func (f *fakeStore) Epoch(context.Context, orgscope.Org) (int64, error) { return f.epoch, nil }

func (f *fakeStore) ActiveMember(context.Context, identity.UserID, orgscope.Org) (bool, error) {
	return f.active, nil
}

func (f *fakeStore) Bindings(context.Context, identity.UserID, orgscope.Org) ([]authz.Binding, error) {
	f.bindCalls++
	return f.bindings, nil
}

func (f *fakeStore) CustomRoles(context.Context, orgscope.Org) (map[string][]authz.Permission, error) {
	return f.custom, nil
}

func (f *fakeStore) ProjectOrganization(_ context.Context, project string) (identity.OrgID, error) {
	org, ok := f.projects[project]
	if !ok {
		return identity.OrgID{}, identity.ErrNotFound
	}
	return org, nil
}

var (
	testSeq    = id.NewSequence()
	testOrg    = identity.OrgID(testSeq.New())
	otherOrg   = identity.OrgID(testSeq.New())
	testUser   = identity.UserID(testSeq.New())
	projectOne = testSeq.New().String()
	projectTwo = testSeq.New().String()
	foreign    = testSeq.New().String()
)

// request asks for the requirement on a project of the test organization, as user.
func request(perms ...authz.Permission) authz.Request {
	return authz.Request{
		Principal:   identity.Principal{Kind: identity.PrincipalUser, UserID: testUser, OrgID: testOrg, CredentialType: identity.CredentialSession},
		Requirement: authz.Requirement{Mode: authz.ModeAll, Permissions: perms},
		Resource:    authz.ResourceRef{Kind: "project", ID: projectOne},
	}
}

func baseStore() *fakeStore {
	return &fakeStore{
		epoch:    1,
		active:   true,
		projects: map[string]identity.OrgID{projectOne: testOrg, projectTwo: testOrg, foreign: otherOrg},
		custom:   map[string][]authz.Permission{},
	}
}

// TestPredefinedRolesMatchTheDecision (SPEC-004 R-AZ-02, ADR-0041 section 3): the grants of
// the predefined roles, spot-checked on the rules that differ.
func TestPredefinedRolesMatchTheDecision(t *testing.T) {
	cat := testCatalog(t)
	has := func(role string, p authz.Permission) bool {
		return slices.Contains(authz.BuiltinPermissions(cat, role), p)
	}
	cases := []struct {
		role string
		perm authz.Permission
		want bool
	}{
		{authz.RoleOwner, "org:delete", true},
		{authz.RoleAdmin, "org:delete", false},
		{authz.RoleAdmin, "flow:publish", true},
		{authz.RoleMember, "project:create", true},
		{authz.RoleMember, "flow:read", false},
		{authz.RoleProjectAdmin, "project:manage-members", true},
		{authz.RoleProjectAdmin, "org:read", false},
		{authz.RoleEditor, "flow:create", true},
		{authz.RoleEditor, "flow:move", false},
		{authz.RoleEditor, "execution:delete", false},
		{authz.RoleOperator, "execution:retry", true},
		{authz.RoleOperator, "execution:read-data", false},
		{authz.RoleOperator, "flow:update", false},
		{authz.RoleViewer, "flow:read", true},
		{authz.RoleViewer, "flow:update", false},
	}
	for _, c := range cases {
		if got := has(c.role, c.perm); got != c.want {
			t.Errorf("%s has %s = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}

// TestOwnerReachesEveryProject: an organization binding applies to every project.
func TestOwnerReachesEveryProject(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	for _, p := range []string{projectOne, projectTwo} {
		req := request("flow:publish")
		req.Resource.ID = p
		if d, _, err := ev.Authorize(t.Context(), req); err != nil || d != authz.DecisionAllow {
			t.Errorf("Owner in %s: %v, %v; want allow", p, d, err)
		}
	}
}

// TestOperatorOnlyInItsProject (ADR-0041 section 3, Q2): a project binding gives its
// project and nothing else; without data permission the data is forbidden, not hidden.
func TestOperatorOnlyInItsProject(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOperator, Project: projectOne}}
	ev := authz.NewEvaluator(st, testCatalog(t))

	req := request("execution:retry")
	if d, _, _ := ev.Authorize(t.Context(), req); d != authz.DecisionAllow {
		t.Errorf("retry in its project: %v, want allow", d)
	}
	req = request("execution:read-data")
	if d, _, _ := ev.Authorize(t.Context(), req); d != authz.DecisionForbidden {
		t.Errorf("data in its project: %v, want forbidden (it can read, not read data)", d)
	}
	req = request("flow:update")
	if d, _, _ := ev.Authorize(t.Context(), req); d != authz.DecisionForbidden {
		t.Errorf("edit in its project: %v, want forbidden", d)
	}
	req = request("flow:read")
	req.Resource.ID = projectTwo
	if d, _, _ := ev.Authorize(t.Context(), req); d != authz.DecisionHidden {
		t.Errorf("another project: %v, want hidden", d)
	}
}

// TestCustomRoleCarriesDataPermission (ADR-0041 section 3): "Operator with data" is a
// custom role that adds the data permission.
func TestCustomRoleCarriesDataPermission(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: "custom-operator-data", Project: projectOne}}
	st.custom["custom-operator-data"] = []authz.Permission{"project:read", "flow:read", "execution:read", "execution:read-data"}
	ev := authz.NewEvaluator(st, testCatalog(t))
	if d, _, _ := ev.Authorize(t.Context(), request("execution:read-data")); d != authz.DecisionAllow {
		t.Errorf("custom role with data: %v, want allow", d)
	}
}

// TestForeignAndNonMembersAreHidden (SPEC-004 R-AZ-10): a project of another organization,
// and anyone who is not an active member, get the same answer as a missing resource.
func TestForeignAndNonMembersAreHidden(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	req := request("flow:read")
	req.Resource.ID = foreign
	if d, _, _ := ev.Authorize(t.Context(), req); d != authz.DecisionHidden {
		t.Errorf("project of another organization: %v, want hidden", d)
	}
	st.active = false
	if d, _, _ := ev.Authorize(t.Context(), request("flow:read")); d != authz.DecisionHidden {
		t.Errorf("inactive membership: %v, want hidden", d)
	}
}

// TestSelectOrganizationNeedsAnActiveMembership (ADR-0038 section 6, SPEC-004 SE6-http): a user
// is a member of the organization named in the header only with an ACTIVE membership there, and
// a zero user or organization is never a member.
func TestSelectOrganizationNeedsAnActiveMembership(t *testing.T) {
	st := baseStore()
	ev := authz.NewEvaluator(st, testCatalog(t))
	if ok, err := ev.SelectOrganization(t.Context(), testUser, testOrg); err != nil || !ok {
		t.Errorf("active member: %v, %v; want true, nil", ok, err)
	}
	st.active = false
	if ok, err := ev.SelectOrganization(t.Context(), testUser, testOrg); err != nil || ok {
		t.Errorf("inactive membership: %v, %v; want false, nil", ok, err)
	}
	st.active = true
	if ok, _ := ev.SelectOrganization(t.Context(), identity.UserID{}, testOrg); ok {
		t.Error("a zero user is a member")
	}
	if ok, _ := ev.SelectOrganization(t.Context(), testUser, identity.OrgID{}); ok {
		t.Error("a zero organization is a member")
	}
	if ok, err := (authz.DenyAll{}).SelectOrganization(t.Context(), testUser, testOrg); err != nil || ok {
		t.Errorf("DenyAll: %v, %v; want false, nil", ok, err)
	}
}

// TestUnknownProjectIsHidden: an identifier that names no project is hidden, not an error.
func TestUnknownProjectIsHidden(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	req := request("flow:read")
	req.Resource.ID = testSeq.New().String()
	if d, _, err := ev.Authorize(t.Context(), req); err != nil || d != authz.DecisionHidden {
		t.Errorf("unknown project: %v, %v; want hidden", d, err)
	}
}

// TestGrantCacheFollowsTheEpoch (SPEC-004 R-AZ-06): a grant is reused while the epoch holds
// and recomputed when the epoch moves, so a revoked role stops working at the next request.
func TestGrantCacheFollowsTheEpoch(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	if d, _, _ := ev.Authorize(t.Context(), request("flow:publish")); d != authz.DecisionAllow {
		t.Fatalf("first request: %v", d)
	}
	if d, _, _ := ev.Authorize(t.Context(), request("flow:publish")); d != authz.DecisionAllow || st.bindCalls != 1 {
		t.Fatalf("second request reused the cache: %v, bindings read %d times; want one read", d, st.bindCalls)
	}
	st.bindings = nil
	st.epoch++
	if d, _, _ := ev.Authorize(t.Context(), request("flow:publish")); d != authz.DecisionHidden {
		t.Errorf("after the revocation and the epoch move: %v, want hidden (no project access)", d)
	}
}

// TestAnonymousAndAuthenticatedNeedNoDecision: the pipeline decides those.
func TestAnonymousAndAuthenticatedNeedNoDecision(t *testing.T) {
	ev := authz.NewEvaluator(baseStore(), testCatalog(t))
	req := authz.Request{Requirement: authz.Requirement{Mode: authz.ModeAuthenticated}}
	if d, _, err := ev.Authorize(t.Context(), req); err != nil || d != authz.DecisionAllow {
		t.Errorf("authenticated: %v, %v", d, err)
	}
}

// TestRoleGrantsCannotEscalate (SPEC-004 R-AZ-03): an actor gives a custom role only the
// permissions it holds itself.
func TestRoleGrantsCannotEscalate(t *testing.T) {
	cat := testCatalog(t)
	editor := authz.BuiltinPermissions(cat, authz.RoleEditor)
	if !authz.MayGrant(editor, []authz.Permission{"flow:read", "flow:create"}) {
		t.Error("an editor may give what it holds")
	}
	if authz.MayGrant(editor, []authz.Permission{"flow:move"}) {
		t.Error("an editor gave flow:move, which it does not hold")
	}
}

// errStore fails every read, to check that the evaluator does not decide on a failure.
type errStore struct{ fakeStore }

func (*errStore) Epoch(context.Context, orgscope.Org) (int64, error) {
	return 0, errors.New("database down")
}

func TestStoreFailureIsNotAnAllow(t *testing.T) {
	ev := authz.NewEvaluator(&errStore{fakeStore{active: true}}, testCatalog(t))
	req := request("org:read")
	req.Resource = authz.ResourceRef{}
	d, _, err := ev.Authorize(t.Context(), req)
	if err == nil || d == authz.DecisionAllow {
		t.Errorf("a failed read: %v, %v; want an error and no allow", d, err)
	}
}
