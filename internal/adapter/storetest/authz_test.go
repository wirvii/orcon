// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// requestAs asks the evaluator for a permission on a project, as a user of an organization.
func requestAs(user identity.UserID, org identity.OrgID, project string, perms ...authz.Permission) authz.Request {
	return authz.Request{
		Principal:   identity.Principal{Kind: identity.PrincipalUser, UserID: user, OrgID: org, CredentialType: identity.CredentialSession},
		Requirement: authz.Requirement{Mode: authz.ModeAll, Permissions: perms},
		Resource:    authz.ResourceRef{Kind: "project", ID: project},
	}
}

// TestAuthorizationOverTheStore (SPEC-004 R-AZ-02, R-AZ-05, R-AZ-06, R-AZ-10): the evaluator reads
// the bindings, the custom roles and the project organization of the real store; a binding change
// takes effect after the epoch moves.
func TestAuthorizationOverTheStore(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		r := rawOf(t, f.s)
		ctx := f.ctx()
		ev := authz.NewEvaluator(f.s, catalogOf(t))
		ana := f.user("ana@example.test")
		bea := f.user("bea@example.test")
		f.member(f.orgA, ana, identity.PrincipalID{})

		// Ana is an Operator in the project of A only.
		bindingID := id.UUID(next[identity.MembershipID](f))
		mustExecOrg(t, r, f.orgA, `INSERT INTO role_binding (id, org_id, scope_kind, project_id, application_user_id, role_name, created_at, updated_at)
VALUES (?, ?, 'PROJECT', ?, ?, 'Operator', ?, ?)`, r.id([16]byte(bindingID)), r.id([16]byte(f.orgA)), r.id([16]byte(f.projA)), r.id([16]byte(ana.ID)), r.instant(base), r.instant(base))

		projA := f.projA.String()
		projB := f.projB.String()
		if d, err := ev.Authorize(ctx, requestAs(ana.ID, f.orgA, projA, "execution:retry")); err != nil || d != authz.DecisionAllow {
			t.Errorf("Operator retries in its project: %v, %v; want allow", d, err)
		}
		if d, _ := ev.Authorize(ctx, requestAs(ana.ID, f.orgA, projA, "execution:read-data")); d != authz.DecisionForbidden {
			t.Errorf("Operator reads data: %v; want forbidden", d)
		}
		if d, _ := ev.Authorize(ctx, requestAs(ana.ID, f.orgA, projB, "flow:read")); d != authz.DecisionHidden {
			t.Errorf("a project of another organization: %v; want hidden", d)
		}
		if d, _ := ev.Authorize(ctx, requestAs(bea.ID, f.orgA, projA, "flow:read")); d != authz.DecisionHidden {
			t.Errorf("a non-member: %v; want hidden", d)
		}

		// A custom role with the data permission, bound to Ana in the same project.
		roleID := id.UUID(next[identity.MembershipID](f))
		mustExecOrg(t, r, f.orgA, `INSERT INTO custom_role (id, org_id, name, scope_kind, created_at, updated_at) VALUES (?, ?, 'Operator with data', 'PROJECT', ?, ?)`,
			r.id([16]byte(roleID)), r.id([16]byte(f.orgA)), r.instant(base), r.instant(base))
		for _, perm := range []string{"execution:read-data", "project:read", "flow:read"} {
			mustExecOrg(t, r, f.orgA, `INSERT INTO custom_role_permission (id, org_id, custom_role_id, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
				r.id([16]byte(next[identity.MembershipID](f))), r.id([16]byte(f.orgA)), r.id([16]byte(roleID)), perm, r.instant(base), r.instant(base))
		}
		mustExecOrg(t, r, f.orgA, `INSERT INTO role_binding (id, org_id, scope_kind, project_id, application_user_id, role_name, created_at, updated_at)
VALUES (?, ?, 'PROJECT', ?, ?, ?, ?, ?)`, r.id([16]byte(next[identity.MembershipID](f))), r.id([16]byte(f.orgA)), r.id([16]byte(f.projA)), r.id([16]byte(ana.ID)), roleID.String(), r.instant(base), r.instant(base))
		epoch, err := f.s.Epoch(ctx, scopeOf(f.orgA))
		if err != nil {
			t.Fatalf("epoch: %v", err)
		}
		if d, _ := ev.Authorize(ctx, requestAs(ana.ID, f.orgA, projA, "execution:read-data")); d != authz.DecisionForbidden {
			t.Errorf("before the epoch moves the grant is still the cached one: %v", d)
		}

		// Revoking the Operator binding and moving the epoch takes effect at the next request.
		mustExecOrg(t, r, f.orgA, `DELETE FROM role_binding WHERE org_id = ? AND application_user_id = ? AND role_name = 'Operator'`, r.id([16]byte(f.orgA)), r.id([16]byte(ana.ID)))
		mustExecOrg(t, r, f.orgA, `UPDATE org SET authorization_epoch = authorization_epoch + 1 WHERE id = ?`, r.id([16]byte(f.orgA)))
		if next, _ := f.s.Epoch(ctx, scopeOf(f.orgA)); next != epoch+1 {
			t.Errorf("epoch = %d after the change, want %d", next, epoch+1)
		}
		if d, _ := ev.Authorize(ctx, requestAs(ana.ID, f.orgA, projA, "flow:read")); d != authz.DecisionAllow {
			t.Errorf("the custom role still lets Ana read the project: %v", d)
		}
		if d, _ := ev.Authorize(ctx, requestAs(ana.ID, f.orgA, projA, "execution:retry")); d != authz.DecisionForbidden {
			t.Errorf("the revoked Operator permission: %v; want forbidden (Ana can still see the project)", d)
		}
	})
}

// mustExecOrg runs a statement under the organization context of org.
func mustExecOrg(t *testing.T, r raw, org identity.OrgID, query string, args ...any) {
	t.Helper()
	if err := r.exec(t.Context(), org, query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// catalogOf builds the permission catalog from the embedded permissions.yaml. The storetest
// package may not import the YAML codec (AL-8), so the names and scopes are read with the
// same line pattern the contract test uses.
func catalogOf(t *testing.T) *authz.Catalog {
	t.Helper()
	scopeLine := regexp.MustCompile(`^  (organization|project|instance):\s*$`)
	nameLine := regexp.MustCompile(`^\s+- name: (\S+)\s*$`)
	byScope := map[authz.Scope][]authz.Permission{}
	var scope authz.Scope
	for _, line := range strings.Split(string(authz.CatalogSource()), "\n") {
		if m := scopeLine.FindStringSubmatch(line); m != nil {
			scope = authz.Scope(m[1])
			continue
		}
		if m := nameLine.FindStringSubmatch(line); m != nil && scope != "" {
			byScope[scope] = append(byScope[scope], authz.Permission(m[1]))
		}
	}
	cat, err := authz.NewCatalog(byScope)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return cat
}
