// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

func wantIs(t *testing.T, what string, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("%s: error %v, want %v", what, got, want)
	}
}

func mustCount(t *testing.T, r raw, org identity.OrgID, want int64, query string, args ...any) {
	t.Helper()
	got, err := r.count(t.Context(), org, query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	if got != want {
		t.Errorf("%s = %d, want %d", query, got, want)
	}
}

// TestUsersBackedByPrincipal (E-PRIN-1): every user is exactly one principal row,
// created and deleted with it. Service accounts are checked by TestServiceAccountsBackedByPrincipal,
// which SPEC-005 API3 adds (E-PRIN-3); this test covers users only.
func TestUsersBackedByPrincipal(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		r := rawOf(t, f.s)
		ctx := f.ctx()
		u := f.user("ada@example.test")
		mustCount(t, r, identity.OrgID{}, 1, "SELECT count(*) FROM principal WHERE id = ? AND kind = 'USER'", r.id(u.PrincipalID))

		got, err := f.s.GetUser(ctx, u.ID)
		want := u
		want.CreatedAt, want.UpdatedAt = base.Truncate(time.Microsecond), base.Truncate(time.Microsecond)
		if err != nil || got != want {
			t.Fatalf("user = %+v, %v; want %+v", got, err, want)
		}
		byEmail, err := f.s.GetUserByEmail(ctx, "ada@example.test")
		if err != nil || byEmail.ID != u.ID {
			t.Fatalf("user by email = %+v, %v", byEmail, err)
		}

		// A second user cannot take the principal of the first (principal_id is unique), and the
		// failed call leaves neither a user nor a principal behind.
		clash := f.user("grace@example.test")
		clash.ID, clash.EmailNormalized = next[identity.UserID](f), "clash@example.test"
		clash.PrincipalID = u.PrincipalID
		wantIs(t, "a user on an existing principal", f.s.CreateUser(ctx, clash), identity.ErrConflict)
		_, err = f.s.GetUserByEmail(ctx, "clash@example.test")
		wantIs(t, "the rejected user", err, identity.ErrNotFound)
		// The same rule holds in the table itself, not only in the store method.
		orphan := identity.PrincipalRecord{ID: next[identity.PrincipalID](f), Kind: identity.PrincipalUser, CreatedAt: base, UpdatedAt: base}
		if err := f.s.CreatePrincipal(ctx, orphan); err != nil {
			t.Fatal(err)
		}
		insertUser := "INSERT INTO application_user (id, principal_id, email_normalized, display_name, status, created_at, updated_at) VALUES (?, ?, ?, 'x', 'ACTIVE', ?, ?)"
		at := r.instant(base)
		if err := r.exec(ctx, identity.OrgID{}, insertUser, r.id(next[identity.UserID](f)), r.id(u.PrincipalID), "dup@example.test", at, at); err == nil {
			t.Error("two users share one principal row")
		}
		if err := r.exec(ctx, identity.OrgID{}, insertUser, r.id(next[identity.UserID](f)), r.id(orphan.ID), "ok@example.test", at, at); err != nil {
			t.Errorf("a user on a free principal: %v", err)
		}
		wantIs(t, "a repeated address", f.s.CreateUser(ctx, identity.UserRecord{
			ID: next[identity.UserID](f), PrincipalID: next[identity.PrincipalID](f), EmailNormalized: "ada@example.test",
			DisplayName: "Again", Status: identity.UserActive, CreatedAt: base, UpdatedAt: base,
		}), identity.ErrConflict)
		bad := f.user("bad@example.test")
		bad.ID, bad.PrincipalID, bad.EmailNormalized, bad.Status = next[identity.UserID](f), next[identity.PrincipalID](f), "worse@example.test", "ASLEEP"
		wantIs(t, "an unknown user status", f.s.CreateUser(ctx, bad), identity.ErrInvalidRecord)
		mustCount(t, r, identity.OrgID{}, 0, "SELECT count(*) FROM principal WHERE id = ?", r.id(bad.PrincipalID))
		_, err = f.s.GetUser(ctx, next[identity.UserID](f))
		wantIs(t, "an unknown user", err, identity.ErrNotFound)

		// Deleting the principal deletes the user and its memberships, and nothing else.
		m := f.member(f.orgA, u, identity.PrincipalID{})
		other := f.user("linus@example.test")
		keep := f.member(f.orgA, other, identity.PrincipalID{})
		if err := f.s.DeletePrincipal(ctx, u.PrincipalID, 10); err != nil {
			t.Fatal(err)
		}
		_, err = f.s.GetUser(ctx, u.ID)
		wantIs(t, "the user of a deleted principal", err, identity.ErrNotFound)
		_, err = f.s.GetMembership(ctx, scopeOf(f.orgA), m.ID)
		wantIs(t, "the membership of a deleted user", err, identity.ErrNotFound)
		if _, err := f.s.GetMembership(ctx, scopeOf(f.orgA), keep.ID); err != nil {
			t.Errorf("another user's membership: %v", err)
		}
		wantIs(t, "a principal that is not there", f.s.DeletePrincipal(ctx, u.PrincipalID, 10), identity.ErrNotFound)
	})
}

// TestSingleSystemPrincipal (E-PRIN-1): the installation has exactly one SYSTEM principal.
func TestSingleSystemPrincipal(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		_, err := f.s.GetSystemPrincipal(ctx)
		wantIs(t, "the system principal before bootstrap", err, identity.ErrNotFound)
		system := identity.PrincipalRecord{ID: next[identity.PrincipalID](f), Kind: identity.PrincipalSystem, CreatedAt: base, UpdatedAt: base}
		if err := f.s.CreatePrincipal(ctx, system); err != nil {
			t.Fatal(err)
		}
		wantIs(t, "a second SYSTEM principal", f.s.CreatePrincipal(ctx, identity.PrincipalRecord{
			ID: next[identity.PrincipalID](f), Kind: identity.PrincipalSystem, CreatedAt: base, UpdatedAt: base,
		}), identity.ErrConflict)
		got, err := f.s.GetSystemPrincipal(ctx)
		if err != nil || got.ID != system.ID {
			t.Fatalf("system principal = %+v, %v", got, err)
		}
		// Many other principals are fine: the rule is about SYSTEM only.
		f.user("one@example.test")
		f.user("two@example.test")
	})
}

// TestPrincipalKindUpperSnakeCheck (E-PRIN-1): principal.kind is stored with the
// spelling the API exposes and ck_principal_kind refuses anything else.
func TestPrincipalKindUpperSnakeCheck(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		for _, kind := range []string{"user", "system", "service_account", "User", "USER ", "ROBOT", ""} {
			err := f.s.CreatePrincipal(f.ctx(), identity.PrincipalRecord{ID: next[identity.PrincipalID](f), Kind: identity.PrincipalKind(kind), CreatedAt: base, UpdatedAt: base})
			if !errors.Is(err, identity.ErrInvalidRecord) {
				t.Errorf("kind %q: error %v, want ErrInvalidRecord (ck_principal_kind)", kind, err)
			}
		}
		for _, kind := range []identity.PrincipalKind{identity.PrincipalUser, identity.PrincipalServiceAccount, identity.PrincipalSystem} {
			if err := f.s.CreatePrincipal(f.ctx(), identity.PrincipalRecord{ID: next[identity.PrincipalID](f), Kind: kind, CreatedAt: base, UpdatedAt: base}); err != nil {
				t.Errorf("kind %q: %v", kind, err)
			}
		}
	})
}

// actorRows seeds what names a principal as an actor: a version it published, a
// membership it granted and a flow it soft-deleted; versions and invitations are
// n each. It returns the ids to look at afterwards.
type actorRows struct {
	principal identity.UserRecord
	flowID    flow.FlowID
	versions  []flow.FlowVersionID
	members   []identity.MembershipID
}

func seedActorRows(t *testing.T, f *fixture, r raw, versions, members int) actorRows {
	t.Helper()
	ctx := f.ctx()
	actor := f.user(fmt.Sprintf("actor-%d@example.test", versions))
	out := actorRows{principal: actor}
	rec := f.flow(f.orgA, f.projA, fmt.Sprintf("actor-flow-%d", versions))
	out.flowID = rec.ID
	for n := 1; n <= versions; n++ {
		v := flow.VersionRecord{
			ID: next[flow.FlowVersionID](f), OrgID: f.orgA, FlowID: rec.ID, Number: int64(n), Document: []byte(`{}`),
			ContentHash: fmt.Sprintf("hash-%d-%d", versions, n), PublishedByPrincipalID: actor.PrincipalID, PublishedAt: base, CreatedAt: base,
		}
		if err := f.s.PublishVersion(ctx, scopeOf(f.orgA), v); err != nil {
			t.Fatal(err)
		}
		out.versions = append(out.versions, v.ID)
	}
	for n := range members {
		invitee := f.user(fmt.Sprintf("invitee-%d-%d@example.test", versions, n))
		out.members = append(out.members, f.member(f.orgA, invitee, actor.PrincipalID).ID)
	}
	// There is no store method to soft-delete a flow yet (the retention cut adds it), so the column is set with SQL.
	deleted := f.flow(f.orgA, f.projA, fmt.Sprintf("deleted-flow-%d", versions))
	if err := r.exec(ctx, f.orgA, "UPDATE flow SET deleted_at = ?, deleted_by_principal_id = ? WHERE id = ?",
		r.instant(base), r.id(actor.PrincipalID), r.id(deleted.ID)); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	return out
}

func (a actorRows) check(t *testing.T, f *fixture, r raw, wantVersions, wantMembers int) {
	t.Helper()
	org := f.orgA
	mustCount(t, r, org, int64(wantVersions), "SELECT count(*) FROM flow_version WHERE flow_id = ?", r.id(a.flowID))
	mustCount(t, r, org, int64(wantVersions), "SELECT count(*) FROM flow_version WHERE flow_id = ? AND published_by_principal_id IS NULL", r.id(a.flowID))
	mustCount(t, r, org, int64(wantMembers), "SELECT count(*) FROM membership WHERE id IN ("+placeholders(len(a.members))+") AND invited_by_principal_id IS NULL", ids(r, a.members)...)
	mustCount(t, r, org, 1, "SELECT count(*) FROM flow WHERE deleted_at IS NOT NULL AND deleted_by_principal_id IS NULL AND name = ?", fmt.Sprintf("deleted-flow-%d", wantVersions))
}

func placeholders(n int) string {
	if n == 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func ids[T ~[16]byte](r raw, in []T) []any {
	out := make([]any, len(in))
	for i := range in {
		out[i] = r.id(in[i])
	}
	return out
}

// TestActorColumnsSetNullOnPrincipalDelete (E-PRIN-1): deleting a principal sets
// every actor column that names it to NULL and deletes no row of history. First the
// foreign key action alone, with a plain DELETE (the safety net of IN-12), then the
// store method.
func TestActorColumnsSetNullOnPrincipalDelete(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		r := rawOf(t, f.s)
		ctx := f.ctx()

		net := seedActorRows(t, f, r, 3, 2)
		if err := r.exec(ctx, identity.OrgID{}, "DELETE FROM principal WHERE id = ?", r.id(net.principal.PrincipalID)); err != nil {
			t.Fatalf("plain delete of the principal: %v", err)
		}
		net.check(t, f, r, 3, 2)

		viaStore := seedActorRows(t, f, r, 4, 1)
		if err := f.s.DeletePrincipal(ctx, viaStore.principal.PrincipalID, 100); err != nil {
			t.Fatal(err)
		}
		viaStore.check(t, f, r, 4, 1)
		// The flows and versions themselves are still readable.
		for _, v := range viaStore.versions {
			got, err := f.s.GetVersion(ctx, scopeOf(f.orgA), v)
			if err != nil || !got.PublishedByPrincipalID.IsZero() {
				t.Errorf("version %v after its publisher was deleted: %+v, %v", v, got, err)
			}
		}
	})
}

// clearOneBatch runs one batch of the clearing that DeletePrincipal repeats, as the
// adapter runs it, and returns how many rows it changed.
func clearOneBatch(t *testing.T, s storetest.Store, principal identity.PrincipalID, batch int32) int64 {
	t.Helper()
	ctx := t.Context()
	switch v := s.(type) {
	case *sqlite.Store:
		q := sqlcgen.New(v.Writer())
		var total int64
		for _, fn := range []func(context.Context, []byte, int64) (int64, error){
			q.UpdateFlowDeletedByPrincipalToNull, q.UpdateFlowVersionPublishedByPrincipalToNull, q.UpdateMembershipInvitedByPrincipalToNull,
		} {
			n, err := fn(ctx, principal[:], int64(batch))
			if err != nil {
				t.Fatal(err)
			}
			if n > int64(batch) {
				t.Fatalf("one statement changed %d rows with a batch of %d", n, batch)
			}
			total += n
		}
		return total
	case *postgres.Store:
		var n int32
		if err := v.Pool().QueryRow(ctx, "SELECT orcon_engine.clear_principal_references($1, $2)", [16]byte(principal), batch).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return int64(n)
	default:
		t.Fatalf("no batch access for %T", s)
		return 0
	}
}

// TestPrincipalDeleteBatchesActorColumns (E-PRIN-1): the clearing is bounded.
// One batch changes at most the batch size per table, repeated batches reach every
// row and then report zero, and DeletePrincipal with a batch smaller than the
// number of rows gets to the same end.
func TestPrincipalDeleteBatchesActorColumns(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		r := rawOf(t, f.s)
		const batch int32 = 2

		stepped := seedActorRows(t, f, r, 7, 5)
		// Tables with rows: flow (1 deleted flow), flow_version (7), membership (5): at most 3 x batch per call.
		var calls []int64
		var total int64
		for {
			n := clearOneBatch(t, f.s, stepped.principal.PrincipalID, batch)
			if n == 0 {
				break
			}
			if n > int64(3*batch) {
				t.Fatalf("a batch changed %d rows, more than %d tables x %d", n, 3, batch)
			}
			calls = append(calls, n)
			total += n
			if len(calls) > 20 {
				t.Fatal("the clearing does not converge")
			}
		}
		if total != 1+7+5 {
			t.Errorf("batches changed %d rows in total (%v), want %d", total, calls, 1+7+5)
		}
		if len(calls) < 3 {
			t.Errorf("%d batches for 7 rows with a batch of %d: the clearing is not bounded", len(calls), batch)
		}
		stepped.check(t, f, r, 7, 5)

		whole := seedActorRows(t, f, r, 9, 4)
		if err := f.s.DeletePrincipal(f.ctx(), whole.principal.PrincipalID, int(batch)); err != nil {
			t.Fatal(err)
		}
		whole.check(t, f, r, 9, 4)
		mustCount(t, r, identity.OrgID{}, 0, "SELECT count(*) FROM principal WHERE id = ?", r.id(whole.principal.PrincipalID))
	})
}

// TestProjectHandleUniqueAcrossOrganizations: the handle is unique in the whole
// instance, not per organization, and a missing handle never collides.
func TestProjectHandleUniqueAcrossOrganizations(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		mk := func(org identity.OrgID, slug, handle string) error {
			return f.s.CreateProject(ctx, scopeOf(org), identity.ProjectRecord{
				ID: next[identity.ProjectID](f), OrgID: org, Name: slug, Slug: slug, Handle: handle, IsProtected: true, CreatedAt: base, UpdatedAt: base,
			}, audittest.Org(t, id.UUID(org), "project.created"))
		}
		if err := mk(f.orgA, "payments", "payments"); err != nil {
			t.Fatal(err)
		}
		err := mk(f.orgB, "payments", "payments")
		wantIs(t, "the handle of another organization", err, identity.ErrHandleTaken)
		wantIs(t, "the handle of another organization, as a conflict", err, identity.ErrConflict)
		if err := mk(f.orgB, "payments", "orgb-payments"); err != nil {
			t.Errorf("the prefixed handle: %v", err)
		}
		wantIs(t, "the same slug in the organization", mk(f.orgB, "payments", "other"), identity.ErrConflict)
		if err := mk(f.orgA, "ledger-1", ""); err != nil {
			t.Fatal(err)
		}
		if err := mk(f.orgB, "ledger-2", ""); err != nil {
			t.Errorf("two projects without a handle: %v", err)
		}
		wantIs(t, "a handle over 201 characters", mk(f.orgA, "long", strings.Repeat("h", 202)), identity.ErrInvalidRecord)
		got, err := f.s.ListProjects(ctx, scopeOf(f.orgB), identity.ProjectID{}, 50)
		if err != nil {
			t.Fatal(err)
		}
		handles := map[string]bool{}
		for _, p := range got {
			handles[p.Handle] = true
			if p.OrgID != f.orgB {
				t.Errorf("project %v of %v listed for %v", p.ID, p.OrgID, f.orgB)
			}
		}
		if !handles["orgb-payments"] || handles["payments"] {
			t.Errorf("handles of organization B = %v", handles)
		}
	})
}

// TestMembershipStore covers the membership table through its port, both stores.
func TestMembershipStore(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		a := scopeOf(f.orgA)
		ada, grace := f.user("ada@example.test"), f.user("grace@example.test")
		inviter := f.user("owner@example.test")
		m := f.member(f.orgA, ada, inviter.PrincipalID)
		got, err := f.s.GetMembership(ctx, a, m.ID)
		want := m
		want.CreatedAt, want.UpdatedAt = base.Truncate(time.Microsecond), base.Truncate(time.Microsecond)
		if err != nil || got != want {
			t.Fatalf("membership = %+v, %v; want %+v", got, err, want)
		}
		ofUser, err := f.s.GetMembershipOfUser(ctx, a, ada.ID)
		if err != nil || ofUser.ID != m.ID {
			t.Fatalf("membership of user = %+v, %v", ofUser, err)
		}
		dup := m
		dup.ID = next[identity.MembershipID](f)
		wantIs(t, "a second membership of one user", f.s.CreateMembership(ctx, a, dup), identity.ErrConflict)
		if err := f.s.CreateMembership(ctx, scopeOf(f.orgB), identity.MembershipRecord{
			ID: next[identity.MembershipID](f), OrgID: f.orgB, UserID: ada.ID, Status: identity.MembershipInvited, CreatedAt: base, UpdatedAt: base,
		}); err != nil {
			t.Errorf("the same user in another organization: %v", err)
		}
		ghost := m
		ghost.ID, ghost.UserID = next[identity.MembershipID](f), next[identity.UserID](f)
		wantIs(t, "an unknown user", f.s.CreateMembership(ctx, a, ghost), identity.ErrReference)
		ghost.UserID, ghost.Status = grace.ID, "PENDING"
		wantIs(t, "an unknown status", f.s.CreateMembership(ctx, a, ghost), identity.ErrInvalidRecord)
		ghost.Status = identity.MembershipActive
		wantIs(t, "a record of another organization", f.s.CreateMembership(ctx, scopeOf(f.orgB), ghost), identity.ErrScopeMismatch)
		_, err = f.s.GetMembershipOfUser(ctx, a, grace.ID)
		wantIs(t, "a user without a membership", err, identity.ErrNotFound)

		later := base.Add(time.Hour)
		if err := f.s.UpdateMembershipStatus(ctx, a, m.ID, identity.MembershipSuspended, later); err != nil {
			t.Fatal(err)
		}
		got, err = f.s.GetMembership(ctx, a, m.ID)
		if err != nil || got.Status != identity.MembershipSuspended || !got.UpdatedAt.Equal(later.Truncate(time.Microsecond)) {
			t.Fatalf("after suspension = %+v, %v", got, err)
		}
		wantIs(t, "an unknown status on update", f.s.UpdateMembershipStatus(ctx, a, m.ID, "GONE", later), identity.ErrInvalidRecord)
		wantIs(t, "an unknown membership", f.s.UpdateMembershipStatus(ctx, a, next[identity.MembershipID](f), identity.MembershipActive, later), identity.ErrNotFound)

		// Keyset pages in identity order.
		for _, email := range []string{"c@example.test", "d@example.test", "e@example.test"} {
			f.member(f.orgA, f.user(email), identity.PrincipalID{})
		}
		var all []identity.MembershipID
		var after identity.MembershipID
		for {
			page, err := f.s.ListMemberships(ctx, a, after, 2)
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
				if page[i].OrgID != f.orgA {
					t.Errorf("membership of %v listed for %v", page[i].OrgID, f.orgA)
				}
				all = append(all, page[i].ID)
			}
			after = page[len(page)-1].ID
		}
		if len(all) != 4 {
			t.Fatalf("listed %d memberships of organization A, want 4", len(all))
		}
		for i := 1; i < len(all); i++ {
			if string(all[i-1][:]) >= string(all[i][:]) {
				t.Fatalf("memberships are not in identity order: %v", all)
			}
		}
	})
}

// TestCreateOrganizationBootstrap: the organization slug is unique and the
// organization is readable only under its own scope.
func TestOrganizationScope(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		a, err := f.s.GetOrganization(f.ctx(), scopeOf(f.orgA))
		if err != nil || a.ID != f.orgA || a.Slug != "org-a" {
			t.Fatalf("organization A = %+v, %v", a, err)
		}
		b, err := f.s.GetOrganization(f.ctx(), scopeOf(f.orgB))
		if err != nil || b.ID != f.orgB || b.Slug != "org-b" {
			t.Fatalf("organization B = %+v, %v", b, err)
		}
	})
}

// TestEveryActorColumnIsClearedInBatches (E-PRIN-1): the batched clearing of
// DeletePrincipal knows every actor column of the schema. A migration that adds a
// <verb>_by_principal_id column and does not teach the clearing about it fails here,
// instead of leaving the column to the foreign key action alone, which is not bounded.
func TestEveryActorColumnIsClearedInBatches(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		s := newStore(t)
		cat := readCatalog(t.Context(), t, s)
		if len(cat.actorColumns) == 0 {
			t.Fatal("no actor column found: the catalog read is wrong")
		}
		switch v := s.(type) {
		case *sqlite.Store:
			if got, want := sqlite.PrincipalReferenceColumns(), cat.actorColumns; !slices.Equal(sortedCopy(got), want) {
				t.Errorf("the SQLite clearing covers %v; the schema has the actor columns %v", got, want)
			}
		case *postgres.Store:
			var body string
			if err := v.Pool().QueryRow(t.Context(), "SELECT pg_get_functiondef('orcon_engine.clear_principal_references(uuid, integer)'::regprocedure)").Scan(&body); err != nil {
				t.Fatal(err)
			}
			for _, column := range cat.actorColumns {
				table, col, _ := strings.Cut(column, ".")
				if !strings.Contains(body, "UPDATE "+table+" SET "+col+" = NULL") {
					t.Errorf("orcon_engine.clear_principal_references does not clear %s", column)
				}
			}
		default:
			t.Fatalf("no coverage check for %T", s)
		}
	})
}

func sortedCopy(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}
