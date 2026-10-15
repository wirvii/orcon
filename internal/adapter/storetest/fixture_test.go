// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// base is the instant the tests start from.
var base = time.Date(2026, 10, 13, 9, 30, 15, 123456789, time.UTC)

// fixture seeds what the cases of this package share: two organizations with a
// project each, on a fresh store.
type fixture struct {
	t     *testing.T
	s     storetest.Store
	ids   *id.Sequence
	orgA  identity.OrgID
	orgB  identity.OrgID
	projA identity.ProjectID
	projB identity.ProjectID
}

func newFixture(t *testing.T, newStore storetest.Factory) *fixture {
	t.Helper()
	f := &fixture{t: t, s: newStore(t), ids: id.NewSequence()}
	f.orgA, f.orgB = f.org("org-a"), f.org("org-b")
	f.projA, f.projB = f.project(f.orgA, "alpha"), f.project(f.orgB, "alpha")
	return f
}

func (f *fixture) ctx() context.Context { return f.t.Context() }

func next[T ~[16]byte](f *fixture) T { return T(f.ids.New()) }

// scopeOf builds the scope of an organization the way the authorization context does, through the issuing package.
func scopeOf(org identity.OrgID) orgscope.Org {
	o, err := issue.New(id.UUID(org))
	if err != nil {
		panic(fmt.Sprintf("storetest: scope of %v: %v", org, err))
	}
	return o
}

func (f *fixture) org(name string) identity.OrgID {
	f.t.Helper()
	o := identity.OrgRecord{ID: next[identity.OrgID](f), Name: name, Slug: name, Status: identity.OrgActive, CreatedAt: base, UpdatedAt: base}
	if err := f.s.CreateOrganization(f.ctx(), o); err != nil {
		f.t.Fatalf("create organization %s: %v", name, err)
	}
	return o.ID
}

func (f *fixture) project(org identity.OrgID, slug string) identity.ProjectID {
	f.t.Helper()
	p := identity.ProjectRecord{ID: next[identity.ProjectID](f), OrgID: org, Name: "Project " + slug, Slug: slug, Handle: slug + "-" + org.String(), IsProtected: true, CreatedAt: base, UpdatedAt: base}
	if err := f.s.CreateProject(f.ctx(), scopeOf(org), p, audittest.Org(f.t, id.UUID(org), "project.created")); err != nil {
		f.t.Fatalf("create project %s: %v", slug, err)
	}
	return p.ID
}

func (f *fixture) flow(org identity.OrgID, project identity.ProjectID, name string) flow.Record {
	f.t.Helper()
	rec := flow.Record{ID: next[flow.FlowID](f), OrgID: org, ProjectID: project, Name: name, Draft: []byte(`{"format":"orcon/flow@1"}`), CreatedAt: base, UpdatedAt: base}
	if err := f.s.CreateFlow(f.ctx(), scopeOf(org), rec); err != nil {
		f.t.Fatalf("create flow %s: %v", name, err)
	}
	return rec
}

// user creates a user (and its principal) and returns it.
func (f *fixture) user(email string) identity.UserRecord {
	f.t.Helper()
	u := identity.UserRecord{
		ID: next[identity.UserID](f), PrincipalID: next[identity.PrincipalID](f), EmailNormalized: email,
		DisplayName: "User " + email, Status: identity.UserActive, CreatedAt: base, UpdatedAt: base,
	}
	if err := f.s.CreateUser(f.ctx(), u); err != nil {
		f.t.Fatalf("create user %s: %v", email, err)
	}
	return u
}

// member adds a user to an organization, invited by the principal (the zero value for nobody).
func (f *fixture) member(org identity.OrgID, user identity.UserRecord, invitedBy identity.PrincipalID) identity.MembershipRecord {
	f.t.Helper()
	m := identity.MembershipRecord{
		ID: next[identity.MembershipID](f), OrgID: org, UserID: user.ID, Status: identity.MembershipActive,
		InvitedByPrincipalID: invitedBy, CreatedAt: base, UpdatedAt: base,
	}
	if err := f.s.CreateMembership(f.ctx(), scopeOf(org), m); err != nil {
		f.t.Fatalf("add %s to %s: %v", user.EmailNormalized, org, err)
	}
	return m
}
