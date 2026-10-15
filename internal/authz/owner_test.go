// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// fakeOwners maps a resource identifier to the project that owns it.
type fakeOwners map[string]string

func (f fakeOwners) OwnerProject(_ context.Context, _ orgscope.Org, resource string) (identity.ProjectID, error) {
	p, ok := f[resource]
	if !ok {
		return identity.ProjectID{}, identity.ErrNotFound
	}
	u, err := id.Parse(p)
	if err != nil {
		return identity.ProjectID{}, err
	}
	return identity.ProjectID(u), nil
}

// failingOwners fails every lookup with a storage error.
type failingOwners struct{}

func (failingOwners) OwnerProject(context.Context, orgscope.Org, string) (identity.ProjectID, error) {
	return identity.ProjectID{}, errors.New("storage is down")
}

// credentialRequest asks for the requirement on a credential, as user.
func credentialRequest(credential string, perms ...authz.Permission) authz.Request {
	req := request(perms...)
	req.Resource = authz.ResourceRef{Kind: "credential", ID: credential}
	return req
}

// TestCredentialIsDecidedByItsOwningProject (SPEC-004 R-AZ-07): a request naming a credential takes the grant of the
// project that owns it. A user of another project of the organization sees neither the credential nor its sharing.
func TestCredentialIsDecidedByItsOwningProject(t *testing.T) {
	const resource = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a90"
	owners := fakeOwners{resource: projectOne}
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleEditor, Project: projectOne}}
	ev := authz.NewEvaluator(st, testCatalog(t)).WithOwner("credential", owners)

	if d, err := ev.Authorize(t.Context(), credentialRequest(resource, "credential:update")); err != nil || d != authz.DecisionAllow {
		t.Errorf("Editor of the owner updates the credential: %v, %v; want allow", d, err)
	}
	if d, err := ev.Authorize(t.Context(), credentialRequest(resource, "credential:share")); err != nil || d != authz.DecisionAllow {
		t.Errorf("Editor of the owner shares it: %v, %v; want allow (Editor holds credential:share on the credentials of its project)", d, err)
	}

	st.bindings = []authz.Binding{{Role: authz.RoleViewer, Project: projectOne}}
	ev = authz.NewEvaluator(st, testCatalog(t)).WithOwner("credential", owners)
	if d, _ := ev.Authorize(t.Context(), credentialRequest(resource, "credential:update")); d != authz.DecisionForbidden {
		t.Errorf("Viewer of the owner updates it: %v, want forbidden", d)
	}
	if d, _ := ev.Authorize(t.Context(), credentialRequest(resource, "credential:read")); d != authz.DecisionAllow {
		t.Errorf("Viewer of the owner reads it: %v, want allow", d)
	}

	st.bindings = []authz.Binding{{Role: authz.RoleEditor, Project: projectTwo}}
	ev = authz.NewEvaluator(st, testCatalog(t)).WithOwner("credential", owners)
	if d, _ := ev.Authorize(t.Context(), credentialRequest(resource, "credential:read")); d != authz.DecisionHidden {
		t.Errorf("a user of another project reads it: %v, want hidden", d)
	}
}

// TestUnknownCredentialIsHidden: an identifier the organization does not hold answers as hidden, the same as a
// credential of another organization (SPEC-004 R-AZ-10).
func TestUnknownCredentialIsHidden(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t)).WithOwner("credential", fakeOwners{})
	if d, err := ev.Authorize(t.Context(), credentialRequest("0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a99", "credential:read")); err != nil || d != authz.DecisionHidden {
		t.Errorf("unknown credential: %v, %v; want hidden", d, err)
	}
}

// TestKindWithoutResolverIsForbidden: a resource kind with no owner resolver stays denied by default.
func TestKindWithoutResolverIsForbidden(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t)).WithOwner("credential", fakeOwners{})
	req := credentialRequest("0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a99", "flow:read")
	req.Resource.Kind = "flow"
	if d, _ := ev.Authorize(t.Context(), req); d != authz.DecisionForbidden {
		t.Errorf("flow without a resolver: %v, want forbidden", d)
	}
}

// TestOwnerLookupFailureIsNotAnAllow: a failing resolver denies the request and reports the error.
func TestOwnerLookupFailureIsNotAnAllow(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	ev := authz.NewEvaluator(st, testCatalog(t)).WithOwner("credential", failingOwners{})
	d, err := ev.Authorize(t.Context(), credentialRequest("0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a99", "credential:read"))
	if err == nil || d == authz.DecisionAllow {
		t.Errorf("failing resolver: %v, %v; want a denial with an error", d, err)
	}
}

// TestOrganizationOfNeedsAnOrganizationContext: the scope of a request exists only with an organization context.
func TestOrganizationOfNeedsAnOrganizationContext(t *testing.T) {
	if _, err := authz.OrganizationOf(identity.Principal{Kind: identity.PrincipalUser, UserID: testUser}); err == nil {
		t.Error("a principal without an organization has a scope")
	}
	org, err := authz.OrganizationOf(identity.Principal{Kind: identity.PrincipalUser, UserID: testUser, OrgID: testOrg})
	if err != nil || org.UUID() != id.UUID(testOrg) {
		t.Errorf("scope = %v, %v; want the organization of the principal", org, err)
	}
}
