// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// seededAt is the instant every seeded row carries.
var seededAt = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// Seed creates organizations A and B in store and fills each with the same shape of
// data: a project, a flow with a published version and a snapshot, a user and its
// membership. Every text the other organization must never see carries a sentinel
// of its owner (see [Org.Sentinel]). It returns the environment of a case.
func Seed(ctx context.Context, tb testing.TB, store Stores, st Store, rlsOnly bool) *Env {
	tb.Helper()
	env, err := NewEnv(st, rlsOnly)
	if err != nil {
		tb.Fatal(err)
	}
	env.Stores = store
	env.IDs = id.NewSequence()
	for _, o := range []*Org{&env.A, &env.B} {
		seedOrg(ctx, tb, env, o)
	}
	return env
}

func seedOrg(ctx context.Context, tb testing.TB, env *Env, o *Org) {
	tb.Helper()
	s := env.Stores
	must := func(what string, err error) {
		tb.Helper()
		if err != nil {
			tb.Fatalf("seed %s of organization %s: %v", what, o.slug, err)
		}
	}
	orgID := identity.OrgID(o.UUID)
	o.Record = identity.OrgRecord{
		ID: orgID, Name: o.Sentinel("org-name"), Slug: "isolation-" + o.slug, Status: identity.OrgActive,
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("organization", s.CreateOrganization(ctx, o.Record))

	o.Project = identity.ProjectRecord{
		ID: identity.ProjectID(env.IDs.New()), OrgID: orgID, Name: o.Sentinel("project-name"), Slug: o.Sentinel("project-slug"),
		Handle: o.Sentinel("project-handle"), Environment: o.Sentinel("environment"), IsProtected: true,
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("project", s.CreateProject(ctx, o.Scope, o.Project, audittest.Org(tb, o.UUID, "project.created")))

	o.Flow = flow.Record{
		ID: flow.FlowID(env.IDs.New()), OrgID: orgID, ProjectID: o.Project.ID, Name: o.Sentinel("flow-name"),
		Draft: []byte(fmt.Sprintf(`{"note":%q}`, o.Sentinel("draft"))), CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("flow", s.CreateFlow(ctx, o.Scope, o.Flow))

	o.Version = flow.VersionRecord{
		ID: flow.FlowVersionID(env.IDs.New()), OrgID: orgID, FlowID: o.Flow.ID, Number: 1,
		Document: []byte(fmt.Sprintf(`{"note":%q}`, o.Sentinel("version"))), ContentHash: o.Sentinel("version-hash"),
		PublishedAt: seededAt, CreatedAt: seededAt,
	}
	must("version", s.PublishVersion(ctx, o.Scope, o.Version))

	o.Snapshot = flow.SnapshotRecord{
		ID: flow.FlowSnapshotID(env.IDs.New()), OrgID: orgID, FlowID: o.Flow.ID, ContentHash: o.Sentinel("snapshot-hash"),
		Document: []byte(fmt.Sprintf(`{"note":%q}`, o.Sentinel("snapshot"))), CreatedAt: seededAt,
	}
	_, err := s.CreateSnapshot(ctx, o.Scope, o.Snapshot)
	must("snapshot", err)

	o.User = identity.UserRecord{
		ID: identity.UserID(env.IDs.New()), PrincipalID: identity.PrincipalID(env.IDs.New()),
		EmailNormalized: o.Sentinel("user") + "@example.test", DisplayName: o.Sentinel("display-name"), Status: identity.UserActive,
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("user", s.CreateUser(ctx, o.User))

	o.Membership = identity.MembershipRecord{
		ID: identity.MembershipID(env.IDs.New()), OrgID: orgID, UserID: o.User.ID, Status: identity.MembershipActive,
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("membership", s.CreateMembership(ctx, o.Scope, o.Membership))
}

// Secrets returns the values that identify the organization's data and that no
// result, error or stored row of another organization may carry. Row identities
// are not among them: a case hands them to the store itself, so an error that
// repeats one reveals nothing.
func (o *Org) Secrets() []string {
	return []string{
		o.Sentinel("org-name"), o.Record.Slug, o.Sentinel("project-name"), o.Sentinel("project-slug"), o.Sentinel("project-handle"),
		o.Sentinel("environment"), o.Sentinel("flow-name"), o.Sentinel("draft"), o.Sentinel("version"), o.Sentinel("version-hash"),
		o.Sentinel("snapshot"), o.Sentinel("snapshot-hash"), o.Sentinel("user"), o.Sentinel("display-name"),
	}
}
