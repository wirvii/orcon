// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz_test

import (
	"context"
	"testing"

	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
)

type childRef struct {
	org     identity.OrgID
	project string
}

func (f *fakeStore) ResourceProject(_ context.Context, kind, resource string) (identity.OrgID, string, error) {
	c, ok := f.children[kind+"/"+resource]
	if !ok {
		return identity.OrgID{}, "", identity.ErrNotFound
	}
	return c.org, c.project, nil
}

// TestChildResourcesResolveToTheirProject: a flow, execution, external operation or human task is
// authorized in the project that owns it, an organization role reaches all of them, a project role
// only its own, and a resource of another organization or a missing one is hidden.
func TestChildResourcesResolveToTheirProject(t *testing.T) {
	st := baseStore()
	st.children = map[string]childRef{}
	own, other, outside := testSeq.New().String(), testSeq.New().String(), testSeq.New().String()
	for _, kind := range authz.ChildKinds {
		st.children[kind+"/"+own] = childRef{testOrg, projectOne}
		st.children[kind+"/"+other] = childRef{testOrg, projectTwo}
		st.children[kind+"/"+outside] = childRef{otherOrg, foreign}
	}
	ask := func(ev *authz.Evaluator, kind, resource string, perm authz.Permission) authz.Decision {
		req := request(perm)
		req.Resource = authz.ResourceRef{Kind: kind, ID: resource}
		d, _, err := ev.Authorize(t.Context(), req)
		if err != nil {
			t.Fatalf("authorize: %v", err)
		}
		return d
	}
	st.bindings = []authz.Binding{{Role: authz.RoleAdmin}}
	admin := authz.NewEvaluator(st, testCatalog(t))
	for _, kind := range authz.ChildKinds {
		for _, r := range []string{own, other} {
			if d := ask(admin, kind, r, "flow:publish"); d != authz.DecisionAllow {
				t.Errorf("Admin on %s %s: %v, want allow", kind, r, d)
			}
		}
		if d := ask(admin, kind, outside, "flow:read"); d != authz.DecisionHidden {
			t.Errorf("Admin on %s of another organization: %v, want hidden", kind, d)
		}
		if d := ask(admin, kind, testSeq.New().String(), "flow:read"); d != authz.DecisionHidden {
			t.Errorf("Admin on a missing %s: %v, want hidden", kind, d)
		}
	}
	for _, c := range []struct {
		role, resource string
		perm           authz.Permission
		want           authz.Decision
	}{
		{authz.RoleEditor, own, "flow:update", authz.DecisionAllow},
		{authz.RoleEditor, other, "flow:update", authz.DecisionHidden},
		{authz.RoleOperator, own, "flow:activate", authz.DecisionAllow},
		{authz.RoleOperator, own, "flow:update", authz.DecisionForbidden},
		{authz.RoleViewer, own, "flow:read", authz.DecisionAllow},
		{authz.RoleViewer, own, "flow:publish", authz.DecisionForbidden},
	} {
		st.bindings = []authz.Binding{{Role: c.role, Project: projectOne}}
		ev := authz.NewEvaluator(st, testCatalog(t))
		if d := ask(ev, "flow", c.resource, c.perm); d != c.want {
			t.Errorf("%s asking %s: %v, want %v", c.role, c.perm, d, c.want)
		}
	}
	st.bindings = []authz.Binding{{Role: authz.RoleAdmin}}
	st.active = false
	if d := ask(authz.NewEvaluator(st, testCatalog(t)), "flow", own, "flow:read"); d != authz.DecisionHidden {
		t.Errorf("non-member: %v, want hidden", d)
	}
	st.active = true
	if d := ask(authz.NewEvaluator(st, testCatalog(t)), "widget", own, "flow:read"); d != authz.DecisionForbidden {
		t.Errorf("unknown kind: %v, want forbidden", d)
	}
}
