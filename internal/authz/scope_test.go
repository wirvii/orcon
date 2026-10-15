// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// session is a user principal that signed in: it carries no organization of its own, so the
// organization of the request is the one the resource names (SPEC-016 SH4).
func session() identity.Principal {
	return identity.Principal{Kind: identity.PrincipalUser, UserID: testUser, CredentialType: identity.CredentialSession}
}

// orgRequest asks for the requirement on the organization resource org, as principal.
func orgRequest(p identity.Principal, org identity.OrgID, perms ...authz.Permission) authz.Request {
	return authz.Request{
		Principal:   p,
		Requirement: authz.Requirement{Mode: authz.ModeAll, Permissions: perms},
		Resource:    authz.ResourceRef{Kind: "org", ID: org.String()},
	}
}

// TestSessionReachesItsOrganizationThroughTheResource: a session principal with an active
// membership of the organization named by the resource is allowed, and the decision carries the
// scope of that organization, which is the only scope a handler may use.
func TestSessionReachesItsOrganizationThroughTheResource(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleAdmin}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	d, scope, err := ev.Authorize(t.Context(), orgRequest(session(), testOrg, "project:create"))
	if err != nil || d != authz.DecisionAllow {
		t.Fatalf("decision = %v, %v; want allow", d, err)
	}
	if scope.UUID() != id.UUID(testOrg) {
		t.Errorf("scope = %s, want the organization of the resource", scope.UUID())
	}
}

// TestSessionWithoutMembershipIsHidden: a membership that is not active hides the organization
// and issues no scope.
func TestSessionWithoutMembershipIsHidden(t *testing.T) {
	st := baseStore()
	st.active = false
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	d, scope, err := ev.Authorize(t.Context(), orgRequest(session(), testOrg, "org:read"))
	if err != nil || d != authz.DecisionHidden {
		t.Fatalf("decision = %v, %v; want hidden", d, err)
	}
	if !scope.UUID().IsZero() {
		t.Errorf("a denied request carries scope %s", scope.UUID())
	}
}

// TestKeyOfAnotherOrganizationSeesNoOrganizationResource: a key principal is fixed to its own
// organization; the organization of the resource being another one hides it.
func TestKeyOfAnotherOrganizationSeesNoOrganizationResource(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	keyPrincipal := identity.Principal{Kind: identity.PrincipalUser, UserID: testUser, OrgID: testOrg, CredentialType: identity.CredentialPersonalKey, CredentialID: "k"}
	d, _, err := ev.Authorize(t.Context(), orgRequest(keyPrincipal, otherOrg, "org:read"))
	if err != nil || d != authz.DecisionHidden {
		t.Fatalf("decision = %v, %v; want hidden", d, err)
	}
}

// TestSessionProjectTakesTheOrganizationOfTheProject: a project resource addresses the organization
// that owns the project, so a session reaches a project of any organization it is a member of.
func TestSessionProjectTakesTheOrganizationOfTheProject(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	req := authz.Request{
		Principal:   session(),
		Requirement: authz.Requirement{Mode: authz.ModeAll, Permissions: []authz.Permission{"flow:read"}},
		Resource:    authz.ResourceRef{Kind: "project", ID: foreign},
	}
	d, scope, err := ev.Authorize(t.Context(), req)
	if err != nil || d != authz.DecisionAllow {
		t.Fatalf("decision = %v, %v; want allow", d, err)
	}
	if scope.UUID() != id.UUID(otherOrg) {
		t.Errorf("scope = %s, want the organization that owns the project", scope.UUID())
	}
}

// TestUnknownResourceKindIsForbiddenWithoutScope: a kind without a resolver is forbidden, and the
// denial carries no scope.
func TestUnknownResourceKindIsForbiddenWithoutScope(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	req := authz.Request{
		Principal:   session(),
		Requirement: authz.Requirement{Mode: authz.ModeAll, Permissions: []authz.Permission{"flow:read"}},
		Resource:    authz.ResourceRef{Kind: "flow", ID: projectOne},
	}
	d, scope, err := ev.Authorize(t.Context(), req)
	if err != nil || d != authz.DecisionForbidden || !scope.UUID().IsZero() {
		t.Fatalf("decision = %v, scope %s, %v; want forbidden without scope", d, scope.UUID(), err)
	}
}

// TestWithOrgScopeRoundTrips: the scope the authorization middleware stores is the one a handler reads.
func TestWithOrgScopeRoundTrips(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleAdmin}}
	ev := authz.NewEvaluator(st, testCatalog(t))
	_, scope, err := ev.Authorize(t.Context(), orgRequest(session(), testOrg, "project:create"))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := authz.OrgScopeFrom(authz.WithOrgScope(t.Context(), scope))
	if !ok || got.UUID() != scope.UUID() {
		t.Errorf("OrgScopeFrom = %v, %v; want the stored scope", got.UUID(), ok)
	}
	if _, ok := authz.OrgScopeFrom(t.Context()); ok {
		t.Errorf("a context without a scope reports one")
	}
}
