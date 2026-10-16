// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// Authorization reads (ADR-0041 section 6, SPEC-004 R-AZ-10): an organization's bindings, custom roles and
// membership are read in the organization's scope, and a project is resolved to its own organization only.

// caseEpoch: every organization has an epoch.
func caseEpoch(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if _, err := env.Stores.Epoch(ctx, env.A.Scope); err != nil {
		tb.Fatalf("epoch of A: %v", err)
	}
}

// caseActiveMember: the user of A is a member of A, and the user of B is not a member of A.
func caseActiveMember(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if ok, err := env.Stores.ActiveMember(ctx, env.A.User.ID, env.A.Scope); err != nil || !ok {
		tb.Fatalf("member of A = %v, %v; want active", ok, err)
	}
	if ok, err := env.Stores.ActiveMember(ctx, env.B.User.ID, env.A.Scope); err != nil || ok {
		tb.Fatalf("B's user in A = %v, %v; want not a member", ok, err)
	}
}

// caseBindings: the bindings of B's user are not listed in A.
func caseBindings(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.Bindings(ctx, env.B.User.ID, env.A.Scope)
	if err != nil || len(got) != 0 {
		tb.Fatalf("bindings of B's user in A = %+v, %v; want none", got, err)
	}
	noLeak(tb, "the bindings", got, err, &env.B)
}

// caseCustomRoles: the custom roles of A are listed for A only.
func caseCustomRoles(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.CustomRoles(ctx, env.A.Scope)
	if err != nil || len(got) != 0 {
		tb.Fatalf("custom roles of A = %+v, %v; want none", got, err)
	}
}

// caseResourceProject: a flow resolves to its organization and project; an unknown identifier and
// an unknown kind are not found.
func caseResourceProject(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	org, project, err := env.Stores.ResourceProject(ctx, "flow", env.B.Flow.ID.String())
	if err != nil || org != identity.OrgID(env.B.UUID) || project != env.B.Project.ID.String() {
		tb.Fatalf("owner of the flow of B = %v, %s, %v", org, project, err)
	}
	if _, _, err := env.Stores.ResourceProject(ctx, "flow", fresh[flow.FlowID](env).String()); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("an unknown flow: %v, want not found", err)
	}
	if _, _, err := env.Stores.ResourceProject(ctx, "widget", env.B.Flow.ID.String()); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("an unknown kind: %v, want not found", err)
	}
}

// caseProjectOrganization: a project resolves to its organization; an unknown identifier is not found.
func caseProjectOrganization(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	org, err := env.Stores.ProjectOrganization(ctx, env.B.Project.ID.String())
	if err != nil || org != identity.OrgID(env.B.UUID) {
		tb.Fatalf("organization of the project of B = %v, %v", org, err)
	}
	if _, err := env.Stores.ProjectOrganization(ctx, fresh[identity.ProjectID](env).String()); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("an unknown project: %v, want not found", err)
	}
}
