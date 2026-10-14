// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package storetest is the conformance suite of the storage adapters
// (ADR-0015 section 1, data-access.md DA-4): one battery of tests that runs
// against the SQLite adapter and, through testcontainers, against the
// PostgreSQL adapter, so both stores behave the same through the ports of the
// areas. The database rules (naming, types, isolation, plans, upgrades) are
// checked by the subpackage dbrules against the real migrations.
package storetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// Store is every port the storage adapters implement, and Close.
type Store interface { //nolint:interfacebloat // the suite exercises every port of the adapters through one value
	identity.OrgStore
	identity.PrincipalStore
	identity.UserStore
	identity.SessionStore
	identity.LoginThrottleStore
	identity.MembershipStore
	identity.ProjectStore
	flow.Store
	flow.VersionStore
	flow.SnapshotStore
	flow.ReferenceStore
	connector.PackageStore
	connector.GrantStore
	execution.WorkerStore
	store.Reader
	store.Inbox
	store.Transactor
	store.WorkQueue
	store.Deliveries
	store.WaitResolver
	io.Closer
}

// Factory returns a store on a fresh, migrated database and registers its
// cleanup with t.
type Factory func(t *testing.T) Store

// base is the instant tests start from; it carries nanoseconds so the
// truncation to microseconds is visible.
var base = time.Date(2026, 10, 13, 9, 30, 15, 123456789, time.UTC)

// fixture seeds what the cases share: two organizations with a project each.
type fixture struct {
	t     *testing.T
	s     Store
	ids   *id.Sequence
	orgA  identity.OrgID
	orgB  identity.OrgID
	projA identity.ProjectID
	projB identity.ProjectID
}

func newFixture(t *testing.T, newStore Factory) *fixture {
	t.Helper()
	f := &fixture{t: t, s: newStore(t), ids: id.NewSequence()}
	f.orgA, f.orgB = f.org("org-a"), f.org("org-b")
	f.projA, f.projB = f.project(f.orgA, "alpha"), f.project(f.orgB, "alpha")
	return f
}

func (f *fixture) ctx() context.Context { return f.t.Context() }

// createFlow writes a flow under the scope of the organization it names.
func (f *fixture) createFlow(rec flow.Record) error {
	return f.s.CreateFlow(f.ctx(), scopeOf(rec.OrgID), rec)
}

func (f *fixture) org(name string) identity.OrgID {
	f.t.Helper()
	o := identity.OrgRecord{ID: next[identity.OrgID](f.ids), Name: name, Slug: name, Status: identity.OrgActive, CreatedAt: base, UpdatedAt: base}
	if err := f.s.CreateOrganization(f.ctx(), o); err != nil {
		f.t.Fatalf("create organization %s: %v", name, err)
	}
	return o.ID
}

func (f *fixture) project(org identity.OrgID, slug string) identity.ProjectID {
	f.t.Helper()
	p := identity.ProjectRecord{ID: next[identity.ProjectID](f.ids), OrgID: org, Name: "Project " + slug, Slug: slug, Handle: slug + "-" + org.String(), Environment: "development", IsProtected: true, CreatedAt: base, UpdatedAt: base}
	if err := f.s.CreateProject(f.ctx(), scopeOf(p.OrgID), p); err != nil {
		f.t.Fatalf("create project %s: %v", slug, err)
	}
	return p.ID
}

func (f *fixture) flow(org identity.OrgID, project identity.ProjectID, name string) flow.Record {
	f.t.Helper()
	rec := flow.Record{ID: next[flow.FlowID](f.ids), OrgID: org, ProjectID: project, Name: name, Draft: []byte(`{"format":"orcon/flow@1"}`), CreatedAt: base, UpdatedAt: base}
	if err := f.s.CreateFlow(f.ctx(), scopeOf(rec.OrgID), rec); err != nil {
		f.t.Fatalf("create flow %s: %v", name, err)
	}
	return rec
}

func (f *fixture) firstVersion(rec flow.Record) flow.VersionRecord {
	f.t.Helper()
	const number = 1
	v := flow.VersionRecord{
		ID: next[flow.FlowVersionID](f.ids), OrgID: rec.OrgID, FlowID: rec.ID, Number: number, Document: []byte(fmt.Sprintf(`{"version":%d}`, number)),
		ContentHash: fmt.Sprintf("%064d", number), PublishedAt: base, CreatedAt: base,
	}
	if err := f.s.PublishVersion(f.ctx(), scopeOf(v.OrgID), v); err != nil {
		f.t.Fatalf("publish version %d: %v", number, err)
	}
	return v
}

// scopeOf returns the scope of an organization. The suite builds scopes the way
// the authorization context does, through the issuing package.
func scopeOf(org identity.OrgID) orgscope.Org {
	o, err := issue.New(id.UUID(org))
	if err != nil {
		panic(fmt.Sprintf("storetest: scope of %v: %v", org, err))
	}
	return o
}

// next returns the next identity of the generator as the given identity type.
func next[T ~[16]byte](ids id.Generator) T { return T(ids.New()) }

func wantErr(t *testing.T, what string, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("%s: error %v, want %v", what, got, want)
	}
}

// Conformance runs the whole battery against stores from newStore.
func Conformance(t *testing.T, newStore Factory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(*testing.T, Factory)
	}{
		{"Instance", instanceCase},
		{"Projects", projectsCase},
		{"Flows", flowsCase},
		{"DraftRevisions", draftRevisionsCase},
		{"Versions", versionsCase},
		{"PublishIsAtomic", publishAtomicCase},
		{"Snapshots", snapshotsCase},
		{"ConnectorPackages", connectorPackagesCase},
		{"ConnectorGrants", connectorGrantsCase},
		{"ConnectorReferences", connectorReferencesCase},
		{"OrganizationIsolation", isolationCase},
		{"InstantsAndBytesRoundTrip", roundTripCase},
		{"ConcurrentWriters", concurrencyCase},
		{"CloseWhileOperationsRun", closeCase},
	}
	cases = append(cases, executionCases()...)
	for i := range cases {
		c := &cases[i]
		t.Run(c.name, func(t *testing.T) { c.run(t, newStore) })
	}
}

// Smoke is the minimal subset a binary of an older release runs against a
// database migrated by the current binary (ADR-0015 section 4, floor
// verification): the paths every release must keep working.
func Smoke(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "smoke")
	f.firstVersion(rec)
	got, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), rec.ID)
	if err != nil || got.ActiveVersionID.IsZero() {
		t.Fatalf("a published flow must have an active version: %+v, %v", got, err)
	}
	if _, err := f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), rec.ID, []byte(`{}`), got.DraftRevision, nil, base); err != nil {
		t.Fatalf("update draft: %v", err)
	}
}

func instanceCase(t *testing.T, newStore Factory) {
	t.Helper()
	s := newStore(t)
	ctx := t.Context()
	ids := id.NewSequence()
	if _, err := s.GetSystemPrincipal(ctx); !errors.Is(err, flow.ErrNotFound) {
		t.Fatalf("system principal before bootstrap: %v", err)
	}
	system := identity.PrincipalRecord{ID: next[identity.PrincipalID](ids), Kind: identity.PrincipalSystem, CreatedAt: base, UpdatedAt: base}
	if err := s.CreatePrincipal(ctx, system); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSystemPrincipal(ctx)
	if err != nil || got.ID != system.ID || got.Kind != identity.PrincipalSystem {
		t.Fatalf("system principal = %+v, %v", got, err)
	}
	wantErr(t, "a second SYSTEM principal", s.CreatePrincipal(ctx, identity.PrincipalRecord{ID: next[identity.PrincipalID](ids), Kind: identity.PrincipalSystem, CreatedAt: base, UpdatedAt: base}), flow.ErrConflict)
	if err := s.CreatePrincipal(ctx, identity.PrincipalRecord{ID: next[identity.PrincipalID](ids), Kind: identity.PrincipalUser, CreatedAt: base, UpdatedAt: base}); err != nil {
		t.Fatalf("a USER principal: %v", err)
	}
	wantErr(t, "an unknown principal kind", s.CreatePrincipal(ctx, identity.PrincipalRecord{ID: next[identity.PrincipalID](ids), Kind: "ROBOT", CreatedAt: base, UpdatedAt: base}), flow.ErrInvalidRecord)

	org := identity.OrgRecord{ID: next[identity.OrgID](ids), Name: "Acme", Slug: "acme", Status: identity.OrgActive, CreatedAt: base, UpdatedAt: base}
	if err := s.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	gotOrg, err := s.GetOrganization(ctx, scopeOf(org.ID))
	want := org
	want.CreatedAt, want.UpdatedAt = base.Truncate(time.Microsecond), base.Truncate(time.Microsecond)
	if err != nil || gotOrg != want {
		t.Fatalf("organization = %+v, %v; want %+v", gotOrg, err, want)
	}
	wantErr(t, "a duplicate organization", s.CreateOrganization(ctx, org), flow.ErrConflict)
	sameSlug := org
	sameSlug.ID = next[identity.OrgID](ids)
	wantErr(t, "a repeated organization slug", s.CreateOrganization(ctx, sameSlug), flow.ErrConflict)
	scheduled := org
	scheduled.ID, scheduled.Slug, scheduled.Status, scheduled.DeletionScheduledAt = next[identity.OrgID](ids), "going", identity.OrgDeletionScheduled, base.Add(time.Hour)
	if err := s.CreateOrganization(ctx, scheduled); err != nil {
		t.Fatalf("an organization scheduled for deletion: %v", err)
	}
	gotScheduled, err := s.GetOrganization(ctx, scopeOf(scheduled.ID))
	if err != nil || !gotScheduled.DeletionScheduledAt.Equal(base.Add(time.Hour).Truncate(time.Microsecond)) {
		t.Fatalf("scheduled organization = %+v, %v", gotScheduled, err)
	}
	scheduled.ID, scheduled.Slug, scheduled.DeletionScheduledAt = next[identity.OrgID](ids), "going-2", time.Time{}
	wantErr(t, "DELETION_SCHEDULED without its instant", s.CreateOrganization(ctx, scheduled), flow.ErrInvalidRecord)
	active := org
	active.ID, active.Slug, active.DeletionScheduledAt = next[identity.OrgID](ids), "active-2", base
	wantErr(t, "an instant on an active organization", s.CreateOrganization(ctx, active), flow.ErrInvalidRecord)
	_, err = s.GetOrganization(ctx, scopeOf(next[identity.OrgID](ids)))
	wantErr(t, "an unknown organization", err, flow.ErrNotFound)
	bad := org
	bad.ID, bad.Slug, bad.Status = next[identity.OrgID](ids), "gone", "GONE"
	wantErr(t, "an unknown status", s.CreateOrganization(ctx, bad), flow.ErrInvalidRecord)
	bad.Status, bad.Name = identity.OrgActive, ""
	wantErr(t, "an empty name", s.CreateOrganization(ctx, bad), flow.ErrInvalidRecord)

	w := execution.WorkerRecord{ID: next[execution.WorkerID](ids), Role: "WORKER", Host: "host-1", Version: "1.0.0", StartedAt: base, HeartbeatAt: base}
	if err := s.RegisterWorker(ctx, w); err != nil {
		t.Fatal(err)
	}
	w.HeartbeatAt, w.Version = base.Add(time.Minute), "1.0.1"
	if err := s.RegisterWorker(ctx, w); err != nil {
		t.Fatalf("a heartbeat refreshes the worker: %v", err)
	}
	w.ID, w.Role = next[execution.WorkerID](ids), "MINER"
	wantErr(t, "an unknown worker role", s.RegisterWorker(ctx, w), flow.ErrInvalidRecord)
}

func projectsCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	var created []identity.ProjectID
	for _, slug := range []string{"one", "two", "three"} {
		created = append(created, f.project(f.orgA, slug))
	}
	got, err := f.s.GetProject(f.ctx(), scopeOf(f.orgA), created[0])
	if err != nil || got.Slug != "one" || got.Environment != "development" || !got.IsProtected {
		t.Fatalf("project = %+v, %v", got, err)
	}
	p := identity.ProjectRecord{ID: next[identity.ProjectID](f.ids), OrgID: f.orgA, Name: "Again", Slug: "one", Environment: "production", IsProtected: true, CreatedAt: base, UpdatedAt: base}
	wantErr(t, "a duplicate slug", f.s.CreateProject(f.ctx(), scopeOf(p.OrgID), p), flow.ErrConflict)
	p.Slug, p.OrgID = "fresh", next[identity.OrgID](f.ids)
	wantErr(t, "an unknown organization", f.s.CreateProject(f.ctx(), scopeOf(p.OrgID), p), flow.ErrReference)
	_, err = f.s.GetProject(f.ctx(), scopeOf(f.orgA), next[identity.ProjectID](f.ids))
	wantErr(t, "an unknown project", err, flow.ErrNotFound)
	p.OrgID, p.Slug, p.Environment = f.orgA, "no-label", ""
	if err := f.s.CreateProject(f.ctx(), scopeOf(p.OrgID), p); err != nil {
		t.Fatalf("a project without an environment label: %v", err)
	}
	if got, err := f.s.GetProject(f.ctx(), scopeOf(f.orgA), p.ID); err != nil || got.Environment != "" {
		t.Fatalf("environment of a project without a label = %q, %v", got.Environment, err)
	}
	p.ID, p.Slug, p.Environment = next[identity.ProjectID](f.ids), "too-long", strings.Repeat("x", 101)
	wantErr(t, "an environment label over 100 characters", f.s.CreateProject(f.ctx(), scopeOf(p.OrgID), p), flow.ErrInvalidRecord)

	// Keyset pages in identity order: alpha (fixture), one, two, three and the unlabeled one.
	var all []identity.ProjectID
	var after identity.ProjectID
	for {
		page, err := f.s.ListProjects(f.ctx(), scopeOf(f.orgA), after, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 2 {
			t.Fatalf("page of %d, limit 2", len(page))
		}
		for i := range page {
			all = append(all, page[i].ID)
		}
		after = page[len(page)-1].ID
	}
	if len(all) != 5 {
		t.Fatalf("listed %d projects, want 5", len(all))
	}
	for i := 1; i < len(all); i++ {
		if bytes.Compare(all[i-1][:], all[i][:]) >= 0 {
			t.Fatalf("projects are not in identity order: %v", all)
		}
	}
}

func flowsCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "invoice")
	got, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "invoice" || got.ProjectID != f.projA || got.DraftRevision != 0 || !bytes.Equal(got.Draft, rec.Draft) ||
		!got.ActiveVersionID.IsZero() || got.IsDeleted() || !got.DeletedByPrincipalID.IsZero() {
		t.Fatalf("flow = %+v", got)
	}
	byName, err := f.s.GetFlowByName(f.ctx(), scopeOf(f.orgA), f.projA, "invoice")
	if err != nil || byName.ID != rec.ID {
		t.Fatalf("by name = %+v, %v", byName, err)
	}
	_, err = f.s.GetFlowByName(f.ctx(), scopeOf(f.orgA), f.projA, "missing")
	wantErr(t, "an unknown name", err, flow.ErrNotFound)

	dup := rec
	dup.ID = next[flow.FlowID](f.ids)
	wantErr(t, "a duplicate name in the project", f.s.CreateFlow(f.ctx(), scopeOf(dup.OrgID), dup), flow.ErrConflict)
	other := f.project(f.orgA, "beta")
	dup.ProjectID = other
	if err := f.s.CreateFlow(f.ctx(), scopeOf(dup.OrgID), dup); err != nil {
		t.Fatalf("the same name in another project: %v", err)
	}
	dup.ID, dup.Name, dup.ProjectID = next[flow.FlowID](f.ids), "orphan", next[identity.ProjectID](f.ids)
	wantErr(t, "an unknown project", f.s.CreateFlow(f.ctx(), scopeOf(dup.OrgID), dup), flow.ErrReference)
	dup.ProjectID, dup.Name, dup.Draft = f.projA, "", nil
	wantErr(t, "an empty name", f.s.CreateFlow(f.ctx(), scopeOf(dup.OrgID), dup), flow.ErrInvalidRecord)

	for _, n := range []string{"b", "c"} {
		f.flow(f.orgA, f.projA, n)
	}
	page, err := f.s.ListFlows(f.ctx(), scopeOf(f.orgA), f.projA, flow.FlowID{}, 2)
	if err != nil || len(page) != 2 {
		t.Fatalf("first page = %d flows, %v", len(page), err)
	}
	rest, err := f.s.ListFlows(f.ctx(), scopeOf(f.orgA), f.projA, page[1].ID, 10)
	if err != nil || len(rest) != 1 {
		t.Fatalf("second page = %d flows, %v", len(rest), err)
	}
}

func draftRevisionsCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "draft")
	rev, err := f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), rec.ID, []byte(`{"n":1}`), 0, nil, base.Add(time.Second))
	if err != nil || rev != 1 {
		t.Fatalf("first update = %d, %v", rev, err)
	}
	_, err = f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), rec.ID, []byte(`{"n":2}`), 0, nil, base.Add(2*time.Second))
	wantErr(t, "a stale revision", err, flow.ErrDraftConflict)
	got, _ := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), rec.ID)
	if got.DraftRevision != 1 || string(got.Draft) != `{"n":1}` || !got.UpdatedAt.Equal(base.Add(time.Second).Truncate(time.Microsecond)) {
		t.Fatalf("a rejected update changed the flow: %+v", got)
	}
	_, err = f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), next[flow.FlowID](f.ids), []byte(`{}`), 0, nil, base)
	wantErr(t, "an unknown flow", err, flow.ErrNotFound)
	if rev, err = f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), rec.ID, []byte(`{"n":3}`), 1, nil, base.Add(3*time.Second)); err != nil || rev != 2 {
		t.Fatalf("second update = %d, %v", rev, err)
	}
}

func versionsCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "versions")
	actor := identity.PrincipalRecord{ID: next[identity.PrincipalID](f.ids), Kind: identity.PrincipalUser, CreatedAt: base, UpdatedAt: base}
	if err := f.s.CreatePrincipal(f.ctx(), actor); err != nil {
		t.Fatal(err)
	}
	v1 := f.firstVersion(rec)
	v2 := flow.VersionRecord{ID: next[flow.FlowVersionID](f.ids), OrgID: f.orgA, FlowID: rec.ID, Number: 2, Document: []byte(`{"v":2}`), ContentHash: "hash-2",
		Warnings: []byte(`[{"code":"W1"}]`), PublishedByPrincipalID: actor.ID, PublishedAt: base.Add(time.Hour), CreatedAt: base.Add(time.Hour)}
	if err := f.s.PublishVersion(f.ctx(), scopeOf(v2.OrgID), v2); err != nil {
		t.Fatal(err)
	}
	got, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), rec.ID)
	if err != nil || got.ActiveVersionID != v2.ID {
		t.Fatalf("active version = %v, %v; want %v", got.ActiveVersionID, err, v2.ID)
	}
	gotV2, err := f.s.GetVersion(f.ctx(), scopeOf(f.orgA), v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotV2.Number != 2 || gotV2.ContentHash != "hash-2" || string(gotV2.Warnings) != `[{"code":"W1"}]` ||
		gotV2.PublishedByPrincipalID != actor.ID || !gotV2.PublishedAt.Equal(v2.PublishedAt.Truncate(time.Microsecond)) {
		t.Fatalf("version 2 = %+v", gotV2)
	}
	gotV1, err := f.s.GetVersionByNumber(f.ctx(), scopeOf(f.orgA), rec.ID, 1)
	if err != nil || gotV1.ID != v1.ID || gotV1.Warnings != nil || !gotV1.PublishedByPrincipalID.IsZero() {
		t.Fatalf("version 1 = %+v, %v", gotV1, err)
	}
	_, err = f.s.GetVersionByNumber(f.ctx(), scopeOf(f.orgA), rec.ID, 9)
	wantErr(t, "an unknown number", err, flow.ErrVersionNotFound)
	_, err = f.s.GetVersion(f.ctx(), scopeOf(f.orgA), next[flow.FlowVersionID](f.ids))
	wantErr(t, "an unknown version", err, flow.ErrVersionNotFound)
	list, err := f.s.ListVersions(f.ctx(), scopeOf(f.orgA), rec.ID, 0, 10)
	if err != nil || len(list) != 2 || list[0].Number != 1 || list[1].Number != 2 {
		t.Fatalf("versions = %+v, %v", list, err)
	}
	if list, err = f.s.ListVersions(f.ctx(), scopeOf(f.orgA), rec.ID, 1, 10); err != nil || len(list) != 1 || list[0].Number != 2 {
		t.Fatalf("versions after 1 = %+v, %v", list, err)
	}
	// An unknown publisher is a reference error, not a silent NULL.
	bad := v2
	bad.ID, bad.Number, bad.PublishedByPrincipalID = next[flow.FlowVersionID](f.ids), 3, next[identity.PrincipalID](f.ids)
	wantErr(t, "an unknown publisher", f.s.PublishVersion(f.ctx(), scopeOf(bad.OrgID), bad), flow.ErrReference)
}

func publishAtomicCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "atomic")
	v1 := f.firstVersion(rec)
	dup := v1
	dup.ID = next[flow.FlowVersionID](f.ids)
	wantErr(t, "a repeated version number", f.s.PublishVersion(f.ctx(), scopeOf(dup.OrgID), dup), flow.ErrConflict)
	got, _ := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), rec.ID)
	if got.ActiveVersionID != v1.ID {
		t.Fatalf("a failed publication moved the active version to %v", got.ActiveVersionID)
	}
	// A version of a flow that does not exist leaves no version row behind.
	ghost := flow.VersionRecord{ID: next[flow.FlowVersionID](f.ids), OrgID: f.orgA, FlowID: next[flow.FlowID](f.ids), Number: 1, Document: []byte(`{}`), ContentHash: "h", PublishedAt: base, CreatedAt: base}
	err := f.s.PublishVersion(f.ctx(), scopeOf(ghost.OrgID), ghost)
	if !errors.Is(err, flow.ErrNotFound) && !errors.Is(err, flow.ErrReference) {
		t.Fatalf("publishing for an unknown flow: %v", err)
	}
	_, err = f.s.GetVersion(f.ctx(), scopeOf(f.orgA), ghost.ID)
	wantErr(t, "the rejected version", err, flow.ErrVersionNotFound)
}

func snapshotsCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "snap")
	s1 := flow.SnapshotRecord{ID: next[flow.FlowSnapshotID](f.ids), OrgID: f.orgA, FlowID: rec.ID, ContentHash: "abc", Document: []byte(`{"s":1}`), CreatedAt: base}
	got, err := f.s.CreateSnapshot(f.ctx(), scopeOf(s1.OrgID), s1)
	if err != nil || got.ID != s1.ID {
		t.Fatalf("first snapshot = %+v, %v", got, err)
	}
	again := s1
	again.ID = next[flow.FlowSnapshotID](f.ids)
	got, err = f.s.CreateSnapshot(f.ctx(), scopeOf(again.OrgID), again)
	if err != nil || got.ID != s1.ID {
		t.Fatalf("a snapshot with the same content hash must return the stored one: %+v, %v", got, err)
	}
	stored, err := f.s.GetSnapshot(f.ctx(), scopeOf(f.orgA), s1.ID)
	if err != nil || string(stored.Document) != `{"s":1}` || stored.FlowID != rec.ID || stored.ContentHash != "abc" {
		t.Fatalf("snapshot = %+v, %v", stored, err)
	}
	_, err = f.s.GetSnapshot(f.ctx(), scopeOf(f.orgA), next[flow.FlowSnapshotID](f.ids))
	wantErr(t, "an unknown snapshot", err, flow.ErrSnapshotNotFound)
	// The same hash in another organization is its own snapshot.
	recB := f.flow(f.orgB, f.projB, "snap")
	sb := flow.SnapshotRecord{ID: next[flow.FlowSnapshotID](f.ids), OrgID: f.orgB, FlowID: recB.ID, ContentHash: "abc", Document: []byte(`{"s":2}`), CreatedAt: base}
	if got, err = f.s.CreateSnapshot(f.ctx(), scopeOf(sb.OrgID), sb); err != nil || got.ID != sb.ID {
		t.Fatalf("snapshot of another organization = %+v, %v", got, err)
	}
}

func isolationCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "secret")
	v := f.firstVersion(rec)
	snap := flow.SnapshotRecord{ID: next[flow.FlowSnapshotID](f.ids), OrgID: f.orgA, FlowID: rec.ID, ContentHash: "h", Document: []byte(`{}`), CreatedAt: base}
	if _, err := f.s.CreateSnapshot(f.ctx(), scopeOf(snap.OrgID), snap); err != nil {
		t.Fatal(err)
	}
	ctx := f.ctx()
	// Organization B asks for organization A's rows by identity: not found, never the data.
	if _, err := f.s.GetProject(ctx, scopeOf(f.orgB), f.projA); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("project of another organization: %v", err)
	}
	if _, err := f.s.GetFlow(ctx, scopeOf(f.orgB), rec.ID); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("flow of another organization: %v", err)
	}
	if _, err := f.s.GetFlowByName(ctx, scopeOf(f.orgB), f.projA, "secret"); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("flow by name of another organization: %v", err)
	}
	if _, err := f.s.GetVersion(ctx, scopeOf(f.orgB), v.ID); !errors.Is(err, flow.ErrVersionNotFound) {
		t.Errorf("version of another organization: %v", err)
	}
	if _, err := f.s.GetVersionByNumber(ctx, scopeOf(f.orgB), rec.ID, 1); !errors.Is(err, flow.ErrVersionNotFound) {
		t.Errorf("version by number of another organization: %v", err)
	}
	if _, err := f.s.GetSnapshot(ctx, scopeOf(f.orgB), snap.ID); !errors.Is(err, flow.ErrSnapshotNotFound) {
		t.Errorf("snapshot of another organization: %v", err)
	}
	if got, err := f.s.ListFlows(ctx, scopeOf(f.orgB), f.projA, flow.FlowID{}, 10); err != nil || len(got) != 0 {
		t.Errorf("flows of another organization's project: %d, %v", len(got), err)
	}
	if got, err := f.s.ListVersions(ctx, scopeOf(f.orgB), rec.ID, 0, 10); err != nil || len(got) != 0 {
		t.Errorf("versions of another organization: %d, %v", len(got), err)
	}
	if got, err := f.s.ListProjects(ctx, scopeOf(f.orgB), identity.ProjectID{}, 10); err != nil || len(got) != 1 || got[0].ID != f.projB {
		t.Errorf("projects of organization B: %+v, %v", got, err)
	}
	// Writes cannot cross either: a flow of B pointing at A's project is rejected.
	cross := flow.Record{ID: next[flow.FlowID](f.ids), OrgID: f.orgB, ProjectID: f.projA, Name: "cross", Draft: []byte(`{}`), CreatedAt: base, UpdatedAt: base}
	wantErr(t, "a flow in another organization's project", f.s.CreateFlow(ctx, scopeOf(cross.OrgID), cross), flow.ErrReference)
	if _, err := f.s.UpdateFlowDraft(ctx, scopeOf(f.orgB), rec.ID, []byte(`{}`), 0, nil, base); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("draft update of another organization's flow: %v", err)
	}
	vb := flow.VersionRecord{ID: next[flow.FlowVersionID](f.ids), OrgID: f.orgB, FlowID: rec.ID, Number: 7, Document: []byte(`{}`), ContentHash: "h", PublishedAt: base, CreatedAt: base}
	if err := f.s.PublishVersion(ctx, scopeOf(vb.OrgID), vb); err == nil {
		t.Error("a version of another organization's flow was accepted")
	}
	got, err := f.s.GetFlow(ctx, scopeOf(f.orgA), rec.ID)
	if err != nil || got.ActiveVersionID != v.ID || got.DraftRevision != 0 {
		t.Errorf("organization A's flow changed: %+v, %v", got, err)
	}
}

func roundTripCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	// Every byte value survives, and an instant keeps microseconds in UTC.
	draft := make([]byte, 256)
	for i := range draft {
		draft[i] = byte(i)
	}
	zone := time.FixedZone("UTC+5", 5*3600)
	instant := time.Date(2026, 1, 2, 3, 4, 5, 987654321, zone)
	rec := flow.Record{ID: next[flow.FlowID](f.ids), OrgID: f.orgA, ProjectID: f.projA, Name: "bytes", Draft: draft, CreatedAt: instant, UpdatedAt: instant}
	if err := f.s.CreateFlow(f.ctx(), scopeOf(rec.OrgID), rec); err != nil {
		t.Fatal(err)
	}
	got, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Draft, draft) {
		t.Error("document bytes changed")
	}
	want := instant.UTC().Truncate(time.Microsecond)
	if !got.CreatedAt.Equal(want) || got.CreatedAt.Location() != time.UTC {
		t.Errorf("created_at = %v (%v), want %v in UTC", got.CreatedAt, got.CreatedAt.Location(), want)
	}
}

func concurrencyCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "race")
	const writers = 16
	// Many writers update the same draft revision: exactly one wins, the rest conflict.
	var wins, conflicts atomic.Int64
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			_, err := f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), rec.ID, []byte(fmt.Sprintf(`{"writer":%d}`, i)), 0, nil, base)
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, flow.ErrDraftConflict):
				conflicts.Add(1)
			default:
				t.Errorf("writer %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != writers-1 {
		t.Fatalf("wins %d conflicts %d, want 1 and %d", wins.Load(), conflicts.Load(), writers-1)
	}
	// Many creators of one name: exactly one wins.
	wins.Store(0)
	conflicts.Store(0)
	for range writers {
		wg.Go(func() {
			err := f.createFlow(flow.Record{ID: next[flow.FlowID](f.ids), OrgID: f.orgA, ProjectID: f.projA, Name: "same", Draft: []byte(`{}`), CreatedAt: base, UpdatedAt: base})
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, flow.ErrConflict):
				conflicts.Add(1)
			default:
				t.Errorf("create: %v", err)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != writers-1 {
		t.Fatalf("creators: wins %d conflicts %d, want 1 and %d", wins.Load(), conflicts.Load(), writers-1)
	}
	// Independent writes all land.
	var landed atomic.Int64
	for i := range 4 * writers {
		wg.Go(func() {
			err := f.createFlow(flow.Record{ID: next[flow.FlowID](f.ids), OrgID: f.orgA, ProjectID: f.projA, Name: fmt.Sprintf("many-%d", i), Draft: []byte(`{}`), CreatedAt: base, UpdatedAt: base})
			if err != nil {
				t.Errorf("create many-%d: %v", i, err)
				return
			}
			landed.Add(1)
		})
	}
	wg.Wait()
	if landed.Load() != 4*writers {
		t.Fatalf("%d of %d independent writes landed", landed.Load(), 4*writers)
	}
}

// closeCase closes the store while writers and readers are running: every
// operation either completes or fails with flow.ErrClosed, nothing panics,
// nothing else is returned, and a second Close is harmless (run under -race).
func closeCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "closing")
	const workers = 12
	var warm, finished sync.WaitGroup
	warm.Add(workers)
	var completed, refused atomic.Int64
	classify := func(err error) bool {
		switch {
		case err == nil:
			completed.Add(1)
			return true
		case errors.Is(err, flow.ErrClosed):
			refused.Add(1)
			return false
		default:
			t.Errorf("an operation around Close failed with %v", err)
			return false
		}
	}
	for w := range workers {
		finished.Go(func() {
			warmed := false
			for i := 0; ; i++ {
				var err error
				if w%2 == 0 {
					err = f.createFlow(flow.Record{ID: next[flow.FlowID](f.ids), OrgID: f.orgA, ProjectID: f.projA,
						Name: fmt.Sprintf("closing-%d-%d", w, i), Draft: []byte(`{}`), CreatedAt: base, UpdatedAt: base})
				} else {
					_, err = f.s.GetFlow(f.ctx(), scopeOf(f.orgA), rec.ID)
				}
				if !classify(err) {
					if !warmed {
						warm.Done() // never completed one: do not hold the closer back
					}
					return
				}
				if !warmed {
					warmed = true
					warm.Done()
				}
			}
		})
	}
	warm.Wait() // every worker completed at least one operation
	if err := f.s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	finished.Wait()
	if refused.Load() != workers {
		t.Errorf("%d workers saw ErrClosed, want %d", refused.Load(), workers)
	}
	if err := f.s.Close(); err != nil {
		t.Errorf("a second close: %v", err)
	}
	_, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), rec.ID)
	wantErr(t, "an operation after Close", err, flow.ErrClosed)
	err = f.createFlow(flow.Record{ID: next[flow.FlowID](f.ids), OrgID: f.orgA, ProjectID: f.projA, Name: "late", Draft: []byte(`{}`)})
	wantErr(t, "a write after Close", err, flow.ErrClosed)
}
