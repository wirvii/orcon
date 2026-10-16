// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
)

// TestFlowScopedAuthorizationOverTheStore: a request that names a flow is authorized in the
// project of the flow, through the real store. An organization Admin reaches every flow of the
// organization, a project role reaches the flows of its project only, a user who is not a member
// and a flow of another organization are hidden, and a missing flow is hidden like them.
func TestFlowScopedAuthorizationOverTheStore(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		beta := f.project(f.orgA, "beta")
		inA, inBeta := f.flow(f.orgA, f.projA, "in alpha"), f.flow(f.orgA, beta, "in beta")
		foreign := f.flow(f.orgB, f.projB, "secret plans")

		admin, editor, viewer, outsider := f.user("admin@example.test"), f.user("editor@example.test"), f.user("viewer@example.test"), f.user("outsider@example.test")
		for _, u := range []identity.UserRecord{admin, editor, viewer} {
			f.member(f.orgA, u, identity.PrincipalID{})
		}
		bindOrg(t, f, admin, identity.RoleAdmin)
		bindProject(t, f, editor, f.projA, "Editor")
		bindProject(t, f, viewer, f.projA, "Viewer")

		ask := func(user identity.UserRecord, flowID string, perm authz.Permission) authz.Decision {
			req := requestAs(user.ID, f.orgA, flowID, perm)
			req.Resource.Kind = "flow"
			d, scope, err := ev.Authorize(f.ctx(), req)
			if err != nil {
				t.Fatalf("authorize: %v", err)
			}
			if d == authz.DecisionAllow && scope.UUID().IsZero() {
				t.Errorf("an allowed request carries no organization scope")
			}
			return d
		}
		const missing = "01a140e5-2cc2-7589-800d-e5e0f090504f"
		cases := []struct {
			name string
			user identity.UserRecord
			flow string
			perm authz.Permission
			want authz.Decision
		}{
			{"admin reads a draft", admin, inA.ID.String(), "flow:read", authz.DecisionAllow},
			{"admin saves a draft", admin, inBeta.ID.String(), "flow:update", authz.DecisionAllow},
			{"admin publishes", admin, inA.ID.String(), "flow:publish", authz.DecisionAllow},
			{"admin activates", admin, inBeta.ID.String(), "flow:activate", authz.DecisionAllow},
			{"editor saves in its project", editor, inA.ID.String(), "flow:update", authz.DecisionAllow},
			{"editor publishes in its project", editor, inA.ID.String(), "flow:publish", authz.DecisionAllow},
			{"editor in another project", editor, inBeta.ID.String(), "flow:read", authz.DecisionHidden},
			{"viewer reads", viewer, inA.ID.String(), "flow:read", authz.DecisionAllow},
			{"viewer cannot save", viewer, inA.ID.String(), "flow:update", authz.DecisionForbidden},
			{"viewer in another project", viewer, inBeta.ID.String(), "flow:read", authz.DecisionHidden},
			{"not a member", outsider, inA.ID.String(), "flow:read", authz.DecisionHidden},
			{"flow of another organization", admin, foreign.ID.String(), "flow:read", authz.DecisionHidden},
			{"missing flow", admin, missing, "flow:read", authz.DecisionHidden},
			{"not an identifier", admin, "nope", "flow:read", authz.DecisionHidden},
		}
		for _, c := range cases {
			if got := ask(c.user, c.flow, c.perm); got != c.want {
				t.Errorf("%s: %v, want %v", c.name, got, c.want)
			}
		}
		for _, kind := range authz.ChildKinds {
			req := requestAs(admin.ID, f.orgA, missing, "execution:read")
			req.Resource.Kind = kind
			if d, _, err := ev.Authorize(f.ctx(), req); err != nil || d != authz.DecisionHidden {
				t.Errorf("a missing %s: %v, %v; want hidden", kind, d, err)
			}
		}
	})
}
