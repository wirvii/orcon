// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
)

// TestUpdateFlowNameWithEntryIsOneTransaction (SPEC-005 Amendment 8 section 8-9): a rename with its
// audit entry moves the revision of the flow row by one and leaves the draft revision alone; a stale
// revision, a name the project already uses, an entry the store refuses and a flow that is not there
// all leave the flow as it was.
func TestUpdateFlowNameWithEntryIsOneTransaction(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		rec := flow.Record{
			ID: next[flow.FlowID](g.fixture), OrgID: g.orgA, ProjectID: g.projA, Name: "Welcome",
			Draft: []byte(`{"n":0}`), ActivationState: flow.ActivationInactive, CreatedAt: base, UpdatedAt: base,
		}
		other := rec
		other.ID, other.Name = next[flow.FlowID](g.fixture), "Taken"
		for _, r := range []flow.Record{rec, other} {
			if err := g.s.CreateFlow(g.ctx(), scope, r); err != nil {
				t.Fatalf("create flow: %v", err)
			}
		}
		good := audittest.Org(t, id.UUID(g.orgA), "flow.updated")
		if err := g.s.UpdateFlowNameWithEntry(g.ctx(), scope, rec.ID, "Renamed", 0, base.Add(time.Second), good); err != nil {
			t.Fatalf("rename = %v", err)
		}
		got, err := g.s.GetFlow(g.ctx(), scope, rec.ID)
		if err != nil || got.Name != "Renamed" || got.Revision != 1 || got.DraftRevision != 0 {
			t.Fatalf("flow = %+v, %v; want the new name at revision 1 and draft revision 0", got, err)
		}
		stale := audittest.Org(t, id.UUID(g.orgA), "flow.updated")
		requireErr(t, "a stale revision", g.s.UpdateFlowNameWithEntry(g.ctx(), scope, rec.ID, "Stale", 0, base.Add(2*time.Second), stale), flow.ErrRevisionConflict)
		dup := audittest.Org(t, id.UUID(g.orgA), "flow.updated")
		requireErr(t, "a name in use", g.s.UpdateFlowNameWithEntry(g.ctx(), scope, rec.ID, "Taken", 1, base.Add(3*time.Second), dup), flow.ErrConflict)
		foreign := audittest.Org(t, id.UUID(g.orgB), "flow.updated")
		if err := g.s.UpdateFlowNameWithEntry(g.ctx(), scope, rec.ID, "Foreign", 1, base.Add(4*time.Second), foreign); err == nil {
			t.Fatal("an entry of another organization was accepted")
		}
		got, err = g.s.GetFlow(g.ctx(), scope, rec.ID)
		if err != nil || got.Name != "Renamed" || got.Revision != 1 {
			t.Errorf("flow = %+v, %v; want the refused renames to leave it at revision 1", got, err)
		}
		missing := audittest.Org(t, id.UUID(g.orgA), "flow.updated")
		requireErr(t, "a missing flow", g.s.UpdateFlowNameWithEntry(g.ctx(), scope, next[flow.FlowID](g.fixture), "Nobody", 0, base, missing), flow.ErrNotFound)
		pending, err := g.s.ListPending(g.ctx(), audit.OrganizationChain(id.UUID(g.orgA)), 10)
		if err != nil {
			t.Fatal(err)
		}
		var renames int
		for i := range pending {
			if pending[i].Action == "flow.updated" {
				renames++
			}
		}
		if renames != 1 {
			t.Errorf("pending flow.updated entries = %d, want 1: only the rename that committed", renames)
		}
	})
}

// TestFlowRevisionRisesWithTheRowNotWithTheDraft (SPEC-005 Amendment 8 section 8-9): a draft save
// leaves the revision of the flow row alone, a publication moves it, and a read shows the number of
// the active version. The release note and the rollback origin of a version come back as written.
func TestFlowRevisionRisesWithTheRowNotWithTheDraft(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		rec := flow.Record{
			ID: next[flow.FlowID](g.fixture), OrgID: g.orgA, ProjectID: g.projA, Name: "Welcome",
			Draft: []byte(`{"n":0}`), ActivationState: flow.ActivationInactive, CreatedAt: base, UpdatedAt: base,
		}
		if err := g.s.CreateFlow(g.ctx(), scope, rec); err != nil {
			t.Fatal(err)
		}
		if _, err := g.s.UpdateFlowDraft(g.ctx(), scope, rec.ID, []byte(`{"n":1}`), 0, nil, base.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		got, err := g.s.GetFlow(g.ctx(), scope, rec.ID)
		if err != nil || got.Revision != 0 || got.DraftRevision != 1 || got.ActiveVersion != 0 {
			t.Fatalf("after a draft save = %+v, %v; want revision 0, draft revision 1, no active version", got, err)
		}
		v1 := flow.VersionRecord{
			ID: next[flow.FlowVersionID](g.fixture), OrgID: g.orgA, FlowID: rec.ID, Number: 1, Document: []byte(`{"v":1}`),
			ContentHash: "hash-1", Note: "first release", PublishedAt: base.Add(2 * time.Second), CreatedAt: base.Add(2 * time.Second),
		}
		v2 := flow.VersionRecord{
			ID: next[flow.FlowVersionID](g.fixture), OrgID: g.orgA, FlowID: rec.ID, Number: 2, Document: []byte(`{"v":2}`),
			ContentHash: "hash-2", RolledBackFromVersion: 1, PublishedAt: base.Add(3 * time.Second), CreatedAt: base.Add(3 * time.Second),
		}
		for _, v := range []flow.VersionRecord{v1, v2} {
			if err := g.s.PublishVersion(g.ctx(), scope, v); err != nil {
				t.Fatalf("publish version %d: %v", v.Number, err)
			}
		}
		got, err = g.s.GetFlow(g.ctx(), scope, rec.ID)
		if err != nil || got.Revision != 2 || got.ActiveVersion != 2 || got.ActiveVersionID != v2.ID {
			t.Fatalf("after two publications = %+v, %v; want revision 2 and active version 2", got, err)
		}
		list, err := g.s.ListVersions(g.ctx(), scope, rec.ID, 0, 10)
		if err != nil || len(list) != 2 || list[0].Note != "first release" || list[0].RolledBackFromVersion != 0 || list[1].Note != "" || list[1].RolledBackFromVersion != 1 {
			t.Fatalf("versions = %+v, %v; want the note of version 1 and the rollback origin of version 2", list, err)
		}
		one, err := g.s.GetVersionByNumber(g.ctx(), scope, rec.ID, 2)
		if err != nil || one.RolledBackFromVersion != 1 {
			t.Fatalf("version 2 = %+v, %v", one, err)
		}
	})
}

// requireErr fails the test unless err is, or wraps, target.
func requireErr(t *testing.T, what string, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("%s = %v, want %v", what, err, target)
	}
}
