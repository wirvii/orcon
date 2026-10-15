// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// digestN returns a well-formed package digest that differs per n.
func digestN(n int) string { return fmt.Sprintf("sha256:%064x", n) }

// pkg builds a package record of a connector version.
func (f *fixture) pkg(connectorID, version string, n int) connector.PackageRecord {
	return connector.PackageRecord{
		ID: next[connector.PackageID](f.ids), ConnectorID: connectorID, Version: version, Digest: digestN(n),
		Kind: connector.KindDeclarative, Manifest: []byte(fmt.Sprintf(`{"id":%q,"version":%q}`, connectorID, version)),
		Status: connector.StatusInstalled, InstalledAt: base, CreatedAt: base, UpdatedAt: base,
	}
}

// insertGrant stores a grant under the scope of the organization the grant names.
func (f *fixture) insertGrant(g connector.GrantRecord) error {
	return f.s.InsertGrant(f.ctx(), scopeOf(g.OrgID), g)
}

func (f *fixture) insertPackage(p connector.PackageRecord) connector.PackageRecord {
	f.t.Helper()
	if err := f.s.InsertPackage(f.ctx(), p); err != nil {
		f.t.Fatalf("insert package %s %s: %v", p.ConnectorID, p.Version, err)
	}
	return p
}

func (f *fixture) principal() identity.PrincipalID {
	f.t.Helper()
	p := identity.PrincipalRecord{ID: next[identity.PrincipalID](f.ids), Kind: identity.PrincipalUser, CreatedAt: base, UpdatedAt: base}
	if err := f.s.CreatePrincipal(f.ctx(), p); err != nil {
		f.t.Fatalf("create principal: %v", err)
	}
	return p.ID
}

// samePackage compares two package records, treating instants as the stored
// microseconds.
func samePackage(t *testing.T, what string, got, want connector.PackageRecord) {
	t.Helper()
	want.InstalledAt, want.CreatedAt, want.UpdatedAt = want.InstalledAt.UTC().Truncate(time.Microsecond), want.CreatedAt.UTC().Truncate(time.Microsecond), want.UpdatedAt.UTC().Truncate(time.Microsecond)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("%s (-want +got):\n%s", what, diff)
	}
}

func versions(ps []connector.PackageRecord) []string {
	out := make([]string, len(ps))
	for i := range ps {
		out[i] = ps[i].Version
	}
	return out
}

// packageState is what the package sub-cases share.
type packageState struct {
	f                              *fixture
	first, second, third, otherOne connector.PackageRecord
}

func connectorPackagesCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	first := f.pkg("acme.shop", "1.0.0", 1)
	first.InstalledByPrincipalID = f.principal()
	first.Manifest = []byte("{\"id\":\"acme.shop\",\"note\":\"é\\u0000\"}")
	f.insertPackage(first)
	st := &packageState{
		f: f, first: first,
		second:   f.insertPackage(f.pkg("acme.shop", "1.10.0", 2)),
		third:    f.insertPackage(f.pkg("acme.shop", "1.2.0", 3)),
		otherOne: f.insertPackage(f.pkg("other.tool", "0.1.0", 4)),
	}
	t.Run("round trip", st.roundTrip)
	t.Run("not found", st.notFound)
	t.Run("versions of one connector", st.versionsOfOne)
	t.Run("all packages by identity", st.allByIdentity)
	t.Run("constraints", st.constraints)
}

func (st *packageState) roundTrip(t *testing.T) {
	f := st.f
	got, err := f.s.GetPackage(f.ctx(), "acme.shop", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	samePackage(t, "GetPackage", got, st.first)
	byID, err := f.s.GetPackageByID(f.ctx(), st.first.ID)
	if err != nil {
		t.Fatal(err)
	}
	samePackage(t, "GetPackageByID", byID, st.first)
	if !bytes.Equal(byID.Manifest, st.first.Manifest) {
		t.Errorf("the manifest bytes changed: %q", byID.Manifest)
	}
	noInstaller, err := f.s.GetPackage(f.ctx(), "other.tool", "0.1.0")
	if err != nil || !noInstaller.InstalledByPrincipalID.IsZero() {
		t.Errorf("a package without an installer = %+v, %v", noInstaller, err)
	}
}

func (st *packageState) notFound(t *testing.T) {
	f := st.f
	_, err := f.s.GetPackage(f.ctx(), "acme.shop", "9.9.9")
	wantErr(t, "an unknown version", err, connector.ErrNotFound)
	_, err = f.s.GetPackageByID(f.ctx(), next[connector.PackageID](f.ids))
	wantErr(t, "an unknown identity", err, connector.ErrNotFound)
}

func (st *packageState) versionsOfOne(t *testing.T) {
	f := st.f
	got, err := f.s.ListPackageVersions(f.ctx(), "acme.shop", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	// The store orders by version text; SemVer precedence is the caller's job.
	if len(got) != 2 || got[0].Version != "1.0.0" || got[1].Version != "1.10.0" {
		t.Fatalf("first page = %v", versions(got))
	}
	rest, err := f.s.ListPackageVersions(f.ctx(), "acme.shop", got[1].Version, 10)
	if err != nil || len(rest) != 1 || rest[0].ID != st.third.ID {
		t.Errorf("second page = %v, %v", versions(rest), err)
	}
	none, err := f.s.ListPackageVersions(f.ctx(), "nobody.here", "", 10)
	if err != nil || len(none) != 0 {
		t.Errorf("an unknown connector = %v, %v", versions(none), err)
	}
}

func (st *packageState) allByIdentity(t *testing.T) {
	f := st.f
	var seen []connector.PackageID
	var after connector.PackageID
	for {
		page, err := f.s.ListPackages(f.ctx(), after, 3)
		if err != nil {
			t.Fatal(err)
		}
		for i := range page {
			seen = append(seen, page[i].ID)
		}
		if len(page) < 3 {
			break
		}
		after = page[len(page)-1].ID
	}
	want := []connector.PackageID{st.first.ID, st.second.ID, st.third.ID, st.otherOne.ID}
	if diff := cmp.Diff(want, seen); diff != "" {
		t.Errorf("ListPackages order (-want +got):\n%s", diff)
	}
}

func (st *packageState) constraints(t *testing.T) {
	f := st.f
	wantErr(t, "the same connector and version", f.s.InsertPackage(f.ctx(), f.pkg("acme.shop", "1.0.0", 10)), connector.ErrConflict)
	wantErr(t, "the same digest", f.s.InsertPackage(f.ctx(), f.pkg("acme.shop", "5.0.0", 1)), connector.ErrConflict)
	badKind := f.pkg("acme.shop", "6.0.0", 11)
	badKind.Kind = "declarative"
	wantErr(t, "a lowercase kind", f.s.InsertPackage(f.ctx(), badKind), connector.ErrInvalidRecord)
	badStatus := f.pkg("acme.shop", "6.0.1", 12)
	badStatus.Status = "installed"
	wantErr(t, "a lowercase status", f.s.InsertPackage(f.ctx(), badStatus), connector.ErrInvalidRecord)
	badDigest := f.pkg("acme.shop", "6.0.2", 13)
	badDigest.Digest = "sha256:short"
	wantErr(t, "a short digest", f.s.InsertPackage(f.ctx(), badDigest), connector.ErrInvalidRecord)
	ghost := f.pkg("acme.shop", "6.0.3", 14)
	ghost.InstalledByPrincipalID = next[identity.PrincipalID](f.ids)
	wantErr(t, "an unknown installer", f.s.InsertPackage(f.ctx(), ghost), connector.ErrReference)
}

func (f *fixture) grantOf(org identity.OrgID, p connector.PackageRecord, project identity.ProjectID, n int) connector.GrantRecord {
	return connector.GrantRecord{
		ID: next[connector.GrantID](f.ids), OrgID: org, PackageID: p.ID, ProjectID: project,
		PermissionsDigest: digestN(100 + n), GrantedAt: base, CreatedAt: base, UpdatedAt: base,
	}
}

// grantState is what the grant sub-cases share.
type grantState struct {
	f          *fixture
	p1, p2, p3 connector.PackageRecord
	orgWide    connector.GrantRecord
	scoped     connector.GrantRecord
	otherProj  identity.ProjectID
}

func connectorGrantsCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	st := &grantState{
		f: f, otherProj: f.project(f.orgA, "beta"),
		p1: f.insertPackage(f.pkg("acme.shop", "1.0.0", 1)),
		p2: f.insertPackage(f.pkg("acme.shop", "2.0.0", 2)),
		p3: f.insertPackage(f.pkg("acme.shop", "3.0.0", 3)),
	}
	st.orgWide = f.grantOf(f.orgA, st.p1, identity.ProjectID{}, 1)
	st.orgWide.GrantedByPrincipalID = f.principal()
	if err := f.insertGrant(st.orgWide); err != nil {
		t.Fatalf("insert an organization-wide grant: %v", err)
	}
	st.scoped = f.grantOf(f.orgA, st.p2, f.projA, 2)
	if err := f.insertGrant(st.scoped); err != nil {
		t.Fatalf("insert a project grant: %v", err)
	}
	t.Run("an organization-wide grant covers every project", st.orgWideCovers)
	t.Run("a project grant covers only its project", st.projectOnly)
	t.Run("a grant of another organization is invisible", st.otherOrganization)
	t.Run("constraints", st.constraints)
}

func (st *grantState) orgWideCovers(t *testing.T) {
	f := st.f
	for _, project := range []identity.ProjectID{f.projA, st.otherProj} {
		got, err := f.s.FindGrant(f.ctx(), scopeOf(f.orgA), st.p1.ID, project)
		if err != nil {
			t.Fatalf("FindGrant for project %s: %v", project, err)
		}
		want := st.orgWide
		want.GrantedAt, want.CreatedAt, want.UpdatedAt = want.GrantedAt.Truncate(time.Microsecond), want.CreatedAt.Truncate(time.Microsecond), want.UpdatedAt.Truncate(time.Microsecond)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("grant (-want +got):\n%s", diff)
		}
	}
}

func (st *grantState) projectOnly(t *testing.T) {
	f := st.f
	got, err := f.s.FindGrant(f.ctx(), scopeOf(f.orgA), st.p2.ID, f.projA)
	if err != nil || got.ID != st.scoped.ID || got.ProjectID != f.projA {
		t.Errorf("FindGrant(own project) = %+v, %v", got, err)
	}
	_, err = f.s.FindGrant(f.ctx(), scopeOf(f.orgA), st.p2.ID, st.otherProj)
	wantErr(t, "another project of the organization", err, connector.ErrNotFound)
	_, err = f.s.FindGrant(f.ctx(), scopeOf(f.orgA), st.p2.ID, identity.ProjectID{})
	wantErr(t, "no project", err, connector.ErrNotFound)
}

func (st *grantState) otherOrganization(t *testing.T) {
	f := st.f
	_, err := f.s.FindGrant(f.ctx(), scopeOf(f.orgB), st.p1.ID, f.projB)
	wantErr(t, "the other organization's view of an organization-wide grant", err, connector.ErrNotFound)
	own := f.grantOf(f.orgB, st.p3, identity.ProjectID{}, 3)
	if err := f.insertGrant(own); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.FindGrant(f.ctx(), scopeOf(f.orgA), st.p3.ID, f.projA)
	wantErr(t, "organization A looking for organization B's grant", err, connector.ErrNotFound)
	if got, err := f.s.FindGrant(f.ctx(), scopeOf(f.orgB), st.p3.ID, f.projB); err != nil || got.ID != own.ID {
		t.Errorf("organization B's own grant = %+v, %v", got, err)
	}
}

func (st *grantState) constraints(t *testing.T) {
	f := st.f
	wantErr(t, "a second organization-wide grant", f.insertGrant(f.grantOf(f.orgA, st.p1, identity.ProjectID{}, 4)), connector.ErrConflict)
	wantErr(t, "a second grant for the same project", f.insertGrant(f.grantOf(f.orgA, st.p2, f.projA, 5)), connector.ErrConflict)
	unknownPackage := f.grantOf(f.orgA, connector.PackageRecord{ID: next[connector.PackageID](f.ids)}, identity.ProjectID{}, 6)
	wantErr(t, "an unknown package", f.insertGrant(unknownPackage), connector.ErrReference)
	wantErr(t, "a project of another organization", f.insertGrant(f.grantOf(f.orgA, st.p3, f.projB, 7)), connector.ErrReference)
	badDigest := f.grantOf(f.orgA, st.p3, identity.ProjectID{}, 8)
	badDigest.PermissionsDigest = "sha256:short"
	wantErr(t, "a short permissions digest", f.insertGrant(badDigest), connector.ErrInvalidRecord)
	ghost := f.grantOf(f.orgA, st.p3, identity.ProjectID{}, 9)
	ghost.GrantedByPrincipalID = next[identity.PrincipalID](f.ids)
	wantErr(t, "an unknown principal", f.insertGrant(ghost), connector.ErrReference)
}

func (f *fixture) reference(connectorID, version string) flow.ConnectorReference {
	return flow.ConnectorReference{ID: next[flow.ConnectorReferenceID](f.ids), ConnectorID: connectorID, ConnectorVersion: version}
}

func referenceNames(recs []flow.ConnectorReferenceRecord) []string {
	out := make([]string, len(recs))
	for i := range recs {
		out[i] = recs[i].ConnectorID + "@" + recs[i].ConnectorVersion
	}
	return out
}

// referenceState is what the reference sub-cases share.
type referenceState struct {
	f   *fixture
	rec flow.Record
}

func (st *referenceState) draft(t *testing.T, org identity.OrgID) []string {
	t.Helper()
	got, err := st.f.s.ListDraftConnectorReferences(st.f.ctx(), scopeOf(org), st.rec.ID, flow.ConnectorReferenceID{}, 100)
	if err != nil {
		t.Fatalf("list draft references: %v", err)
	}
	return referenceNames(got)
}

func (st *referenceState) version(t *testing.T, org identity.OrgID, v flow.VersionRecord) []string {
	t.Helper()
	got, err := st.f.s.ListVersionConnectorReferences(st.f.ctx(), scopeOf(org), v.ID, flow.ConnectorReferenceID{}, 100)
	if err != nil {
		t.Fatalf("list version references: %v", err)
	}
	return referenceNames(got)
}

// versionOf builds a version record that carries references.
func (st *referenceState) versionOf(number int64, refs ...flow.ConnectorReference) flow.VersionRecord {
	f := st.f
	return flow.VersionRecord{
		ID: next[flow.FlowVersionID](f.ids), OrgID: f.orgA, FlowID: st.rec.ID, Number: number, Document: []byte(`{"v":1}`),
		ContentHash: fmt.Sprintf("%064d", number), PublishedAt: base, CreatedAt: base, ConnectorReferences: refs,
	}
}

func connectorReferencesCase(t *testing.T, newStore Factory) {
	t.Helper()
	f := newFixture(t, newStore)
	st := &referenceState{f: f, rec: f.flow(f.orgA, f.projA, "uses-connectors")}
	// The sub-cases build on one another, in this order.
	t.Run("saving a draft replaces its references", st.replace)
	t.Run("a stale save changes nothing", st.stale)
	t.Run("a bad reference rolls the save back", st.badDraft)
	t.Run("publishing records the references of the version", st.publish)
	t.Run("a refused publication leaves no version and no references", st.refusedPublication)
	t.Run("a version of a missing flow is refused", st.missingFlow)
}

func (st *referenceState) replace(t *testing.T) {
	f := st.f
	save := func(n int64, revision int64, refs ...flow.ConnectorReference) {
		t.Helper()
		got, err := f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), st.rec.ID, []byte(fmt.Sprintf(`{"n":%d}`, n)), revision, refs, base.Add(time.Duration(n)*time.Second))
		if err != nil || got != revision+1 {
			t.Fatalf("save %d = revision %d, %v", n, got, err)
		}
	}
	save(1, 0, f.reference("acme.shop", "1.0.0"), f.reference("other.tool", "0.1.0"))
	if diff := cmp.Diff([]string{"acme.shop@1.0.0", "other.tool@0.1.0"}, st.draft(t, f.orgA)); diff != "" {
		t.Errorf("after the first save (-want +got):\n%s", diff)
	}
	save(2, 1, f.reference("acme.shop", "2.0.0"))
	if diff := cmp.Diff([]string{"acme.shop@2.0.0"}, st.draft(t, f.orgA)); diff != "" {
		t.Errorf("after the second save (-want +got):\n%s", diff)
	}
	save(3, 2)
	if got := st.draft(t, f.orgA); len(got) != 0 {
		t.Errorf("a draft without connectors keeps references: %v", got)
	}
}

func (st *referenceState) stale(t *testing.T) {
	f := st.f
	if _, err := f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), st.rec.ID, []byte(`{"n":4}`), 3, []flow.ConnectorReference{f.reference("acme.shop", "3.0.0")}, base.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err := f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), st.rec.ID, []byte(`{"n":5}`), 0, []flow.ConnectorReference{f.reference("stale.one", "9.9.9")}, base.Add(4*time.Second))
	wantErr(t, "a stale revision", err, flow.ErrDraftConflict)
	if diff := cmp.Diff([]string{"acme.shop@3.0.0"}, st.draft(t, f.orgA)); diff != "" {
		t.Errorf("a failed save changed the references (-want +got):\n%s", diff)
	}
}

func (st *referenceState) badDraft(t *testing.T) {
	f := st.f
	before, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), st.rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	twice := f.reference("acme.shop", "4.0.0")
	again := twice
	again.ID = next[flow.ConnectorReferenceID](f.ids)
	_, err = f.s.UpdateFlowDraft(f.ctx(), scopeOf(f.orgA), st.rec.ID, []byte(`{"n":6}`), before.DraftRevision, []flow.ConnectorReference{twice, again}, base.Add(5*time.Second))
	wantErr(t, "the same pin twice", err, flow.ErrConflict)
	after, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), st.rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.DraftRevision != before.DraftRevision || !bytes.Equal(after.Draft, before.Draft) {
		t.Errorf("the draft moved although its references were refused: %+v -> %+v", before, after)
	}
	if diff := cmp.Diff([]string{"acme.shop@3.0.0"}, st.draft(t, f.orgA)); diff != "" {
		t.Errorf("the references moved (-want +got):\n%s", diff)
	}
}

func (st *referenceState) publish(t *testing.T) {
	f := st.f
	v := st.versionOf(1, f.reference("acme.shop", "3.0.0"), f.reference("other.tool", "0.1.0"))
	if err := f.s.PublishVersion(f.ctx(), scopeOf(v.OrgID), v); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"acme.shop@3.0.0", "other.tool@0.1.0"}, st.version(t, f.orgA, v)); diff != "" {
		t.Errorf("version references (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"acme.shop@3.0.0"}, st.draft(t, f.orgA)); diff != "" {
		t.Errorf("publishing changed the draft references (-want +got):\n%s", diff)
	}
	// Another organization sees none of it.
	if got := st.version(t, f.orgB, v); len(got) != 0 {
		t.Errorf("organization B sees the references of organization A: %v", got)
	}
	if got := st.draft(t, f.orgB); len(got) != 0 {
		t.Errorf("organization B sees the draft references of organization A: %v", got)
	}
}

func (st *referenceState) refusedPublication(t *testing.T) {
	f := st.f
	dup := f.reference("acme.shop", "5.0.0")
	again := dup
	again.ID = next[flow.ConnectorReferenceID](f.ids)
	v := st.versionOf(2, dup, again)
	wantErr(t, "the same pin twice in a version", f.s.PublishVersion(f.ctx(), scopeOf(v.OrgID), v), flow.ErrConflict)
	if _, err := f.s.GetVersion(f.ctx(), scopeOf(f.orgA), v.ID); err == nil {
		t.Error("the version was stored although its references were refused")
	}
	got, err := f.s.GetFlow(f.ctx(), scopeOf(f.orgA), st.rec.ID)
	if err != nil || got.ActiveVersionID == v.ID {
		t.Errorf("the flow points at the refused version: %+v, %v", got, err)
	}
	if refs := st.version(t, f.orgA, v); len(refs) != 0 {
		t.Errorf("references of a refused version remain: %v", refs)
	}
}

func (st *referenceState) missingFlow(t *testing.T) {
	f := st.f
	ghost := st.versionOf(1, f.reference("acme.shop", "1.0.0"))
	ghost.FlowID = next[flow.FlowID](f.ids)
	if err := f.s.PublishVersion(f.ctx(), scopeOf(ghost.OrgID), ghost); err == nil {
		t.Error("a version of a flow that does not exist was published")
	}
}
