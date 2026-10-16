// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest_test

import (
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// The in-memory rename applies the rules of the stores: the revision of the row, a name the
// project already uses and the flow that is not there, and it keeps the audit entry of a
// rename that committed.
func TestMemoryRenameFollowsTheRulesOfTheStores(t *testing.T) {
	m := triggertest.NewMemory()
	ctx := t.Context()
	org := identity.OrgID{1}
	scope := scopeFor(org)
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	if err := m.CreateOrganization(ctx, identity.OrgRecord{ID: org, Name: "A", Slug: "a", Status: identity.OrgActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	project := identity.ProjectID{2}
	if err := m.CreateProject(ctx, scope, identity.ProjectRecord{ID: project, OrgID: org, Name: "P", Slug: "p", Environment: "development", IsProtected: true, CreatedAt: now, UpdatedAt: now},
		audittest.Org(t, id.UUID(org), "project.created")); err != nil {
		t.Fatal(err)
	}
	first := flow.Record{ID: flow.FlowID{3}, OrgID: org, ProjectID: project, Name: "one", Draft: []byte(`{}`), CreatedAt: now, UpdatedAt: now}
	second := flow.Record{ID: flow.FlowID{4}, OrgID: org, ProjectID: project, Name: "two", Draft: []byte(`{}`), CreatedAt: now, UpdatedAt: now}
	for _, rec := range []flow.Record{first, second} {
		if err := m.CreateFlow(ctx, scope, rec); err != nil {
			t.Fatal(err)
		}
	}
	entry := audittest.Org(t, id.UUID(org), "flow.updated")
	if err := m.UpdateFlowNameWithEntry(ctx, scope, first.ID, "renamed", 1, now, entry); !errors.Is(err, flow.ErrRevisionConflict) {
		t.Errorf("rename at a stale revision = %v, want ErrRevisionConflict", err)
	}
	if err := m.UpdateFlowNameWithEntry(ctx, scope, first.ID, "two", 0, now, entry); !errors.Is(err, flow.ErrConflict) {
		t.Errorf("rename to a name in use = %v, want ErrConflict", err)
	}
	if err := m.UpdateFlowNameWithEntry(ctx, scope, flow.FlowID{9}, "x", 0, now, entry); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("rename of a missing flow = %v, want ErrNotFound", err)
	}
	if len(m.Changes()) != 0 {
		t.Fatal("a refused rename kept its audit entry")
	}
	if err := m.UpdateFlowNameWithEntry(ctx, scope, first.ID, "renamed", 0, now, entry); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetFlow(ctx, scope, first.ID)
	if err != nil || got.Name != "renamed" || got.Revision != 1 || len(m.Changes()) != 1 {
		t.Errorf("flow = %+v, %v with %d entries; want the new name at revision 1 and one entry", got, err, len(m.Changes()))
	}
}
