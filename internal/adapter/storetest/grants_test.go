// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/idempotencyclaim"
)

// grantFixture is a project of the first organization and a user who will hold a role there.
type grantFixture struct {
	*fixture
	user identity.UserRecord
}

func newGrantFixture(t *testing.T, newStore storetest.Factory) *grantFixture {
	t.Helper()
	f := newFixture(t, newStore)
	return &grantFixture{fixture: f, user: f.user("creator@example.com")}
}

// bindingFor is the Project Admin binding of user in project, as the identity area writes it.
func (g *grantFixture) bindingFor(project identity.ProjectID, user identity.UserID) identity.RoleBindingRecord {
	return identity.RoleBindingRecord{
		ID: identity.RoleBindingID(g.ids.New()), OrgID: g.orgA, ScopeKind: identity.RoleScopeProject,
		ProjectID: project, UserID: user, RoleName: identity.RoleProjectAdmin, CreatedAt: base, UpdatedAt: base,
	}
}

// newProject is a project record of the first organization, not yet stored.
func (g *grantFixture) newProject(slug string) identity.ProjectRecord {
	return identity.ProjectRecord{
		ID: next[identity.ProjectID](g.fixture), OrgID: g.orgA, Name: "Project " + slug, Slug: slug,
		Handle: slug, IsProtected: true, CreatedAt: base, UpdatedAt: base,
	}
}

// claimCtx is the context of a request that claimed key, as the HTTP pipeline makes it.
func (g *grantFixture) claimCtx(key string, fingerprint [32]byte, user identity.UserID) (claim idempotencyclaim.Claim, ctxKey identity.IdempotencyKey) {
	claim = idempotencyclaim.Claim{
		ID: g.ids.New(), Requester: "PRINCIPAL:" + user.String(), Value: key, Fingerprint: fingerprint,
	}
	return claim, identity.IdempotencyKey{Requester: claim.Requester, Value: key}
}

// TestCreateProjectWithGrantBindsCreator (SPEC-016 Q3 = A; SPEC-004 R-AZ-06): the project, the
// Project Admin binding of its creator and the increment of the authorization epoch are written
// together, and the creator then holds the role in that project only.
func TestCreateProjectWithGrantBindsCreator(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		epochBefore, err := g.s.Epoch(g.ctx(), scope)
		if err != nil {
			t.Fatalf("epoch: %v", err)
		}
		p := g.newProject("grant")
		grant := g.bindingFor(p.ID, g.user.ID)
		if err := g.s.CreateProjectWithGrant(g.ctx(), scope, p, grant, []audit.Entry{audittest.Org(t, id.UUID(g.orgA), "project.created")}); err != nil {
			t.Fatalf("create project with grant: %v", err)
		}
		bindings, err := g.s.Bindings(g.ctx(), g.user.ID, scope)
		if err != nil {
			t.Fatalf("bindings: %v", err)
		}
		want := []authz.Binding{{Role: identity.RoleProjectAdmin, Project: p.ID.String()}}
		if len(bindings) != 1 || bindings[0] != want[0] {
			t.Fatalf("bindings = %+v, want %+v", bindings, want)
		}
		epochAfter, err := g.s.Epoch(g.ctx(), scope)
		if err != nil {
			t.Fatalf("epoch: %v", err)
		}
		if epochAfter != epochBefore+1 {
			t.Errorf("epoch = %d, want %d: the binding did not move the epoch", epochAfter, epochBefore+1)
		}
	})
}

// TestCreateProjectWithGrantIsAtomic: a grant that the database refuses (here, a user that does
// not exist) leaves neither the project, nor the binding, nor the epoch change behind.
func TestCreateProjectWithGrantIsAtomic(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		epochBefore, _ := g.s.Epoch(g.ctx(), scope)
		p := g.newProject("atomic")
		ghost := identity.UserID(g.ids.New())
		err := g.s.CreateProjectWithGrant(g.ctx(), scope, p, g.bindingFor(p.ID, ghost), []audit.Entry{audittest.Org(t, id.UUID(g.orgA), "project.created")})
		if !errors.Is(err, identity.ErrReference) {
			t.Fatalf("grant to an unknown user = %v, want ErrReference", err)
		}
		if _, err := g.s.GetProject(g.ctx(), scope, p.ID); !errors.Is(err, identity.ErrNotFound) {
			t.Errorf("the project survived the refused grant: %v", err)
		}
		if epochAfter, _ := g.s.Epoch(g.ctx(), scope); epochAfter != epochBefore {
			t.Errorf("epoch moved to %d after a refused grant", epochAfter)
		}
	})
}

// TestCreateProjectClaimsIdempotencyKeyInItsTransaction: the record of a claimed key commits with
// the project; a second claim of the key is taken; a refused grant leaves no record behind.
func TestCreateProjectClaimsIdempotencyKeyInItsTransaction(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		fp := sha256.Sum256([]byte("create project"))
		claim, key := g.claimCtx("key-create", fp, g.user.ID)
		ctx := idempotencyclaim.With(g.ctx(), claim)

		p := g.newProject("claimed")
		if err := g.s.CreateProjectWithGrant(ctx, scope, p, g.bindingFor(p.ID, g.user.ID), []audit.Entry{audittest.Org(t, id.UUID(g.orgA), "project.created")}); err != nil {
			t.Fatalf("claimed create: %v", err)
		}
		rec, found, err := g.s.LookupIdempotency(g.ctx(), scope, key, base.Add(time.Second))
		if err != nil || !found {
			t.Fatalf("lookup after the claimed create = %v, %v; want the record", found, err)
		}
		if rec.Completed || rec.ResourceRef != "project:"+id.UUID(p.ID).String() || rec.Fingerprint != fp {
			t.Errorf("record = %+v, want an in-progress record of the project", rec)
		}

		second := g.newProject("second")
		err = g.s.CreateProjectWithGrant(ctx, scope, second, g.bindingFor(second.ID, g.user.ID), []audit.Entry{audittest.Org(t, id.UUID(g.orgA), "project.created")})
		if !errors.Is(err, idempotencyclaim.ErrTaken) {
			t.Fatalf("second create under the same claim = %v, want ErrTaken", err)
		}
		if _, err := g.s.GetProject(g.ctx(), scope, second.ID); !errors.Is(err, identity.ErrNotFound) {
			t.Errorf("the taken claim still wrote its project: %v", err)
		}

		refusedClaim, refusedKey := g.claimCtx("key-refused", fp, g.user.ID)
		refusedCtx := idempotencyclaim.With(g.ctx(), refusedClaim)
		ghost := identity.UserID(g.ids.New())
		third := g.newProject("third")
		if err := g.s.CreateProjectWithGrant(refusedCtx, scope, third, g.bindingFor(third.ID, ghost), []audit.Entry{audittest.Org(t, id.UUID(g.orgA), "project.created")}); !errors.Is(err, identity.ErrReference) {
			t.Fatalf("refused grant = %v, want ErrReference", err)
		}
		if _, found, _ := g.s.LookupIdempotency(g.ctx(), scope, refusedKey, base); found {
			t.Errorf("a refused change left its idempotency record behind")
		}
	})
}

// TestCompleteIdempotencyStoresTheResponse: completing a claimed record stores the response; an
// unclaimed success is stored as a completed record; the record is read back with its response.
func TestCompleteIdempotencyStoresTheResponse(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		fp := sha256.Sum256([]byte("unclaimed"))
		key := identity.IdempotencyKey{Requester: "PRINCIPAL:unclaimed", Value: "key-unclaimed"}
		resp := identity.IdempotencyResponse{Status: 201, ContentType: "application/json", Location: "/api/v1/things/1", ETag: `"0"`, Body: []byte(`{"id":"1"}`)}
		if err := g.s.CompleteIdempotency(g.ctx(), scope, key, identity.IdempotencyCompletion{ID: g.ids.New(), Fingerprint: fp, Response: resp}, base); err != nil {
			t.Fatalf("complete an unclaimed success: %v", err)
		}
		rec, found, err := g.s.LookupIdempotency(g.ctx(), scope, key, base)
		if err != nil || !found {
			t.Fatalf("lookup = %v, %v; want the completed record", found, err)
		}
		if !rec.Completed || rec.Fingerprint != fp || rec.Response.Status != 201 || rec.Response.Location != resp.Location ||
			rec.Response.ETag != resp.ETag || !bytes.Equal(rec.Response.Body, resp.Body) || rec.Response.SecretIssued {
			t.Errorf("record = %+v, want the completed response", rec)
		}
		if err := g.s.CompleteIdempotency(g.ctx(), scope, key, identity.IdempotencyCompletion{ID: g.ids.New(), Fingerprint: fp, Response: resp}, base); err == nil {
			t.Errorf("a second completion of the same key succeeded; the key must complete once")
		}
	})
}

// TestIdempotencyRecordExpiresAndIsReclaimed: a record past its 24 hours is absent, and a new
// claim of its key takes it over.
func TestIdempotencyRecordExpiresAndIsReclaimed(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		key := identity.IdempotencyKey{Requester: "PRINCIPAL:expiry", Value: "key-expiry"}
		resp := identity.IdempotencyResponse{Status: 201, Body: []byte(`{}`)}
		if err := g.s.CompleteIdempotency(g.ctx(), scope, key, identity.IdempotencyCompletion{ID: g.ids.New(), Response: resp}, base); err != nil {
			t.Fatalf("complete: %v", err)
		}
		later := base.Add(identity.IdempotencyTTL + time.Minute)
		if _, found, _ := g.s.LookupIdempotency(g.ctx(), scope, key, later); found {
			t.Fatalf("a record past its expiry is still found")
		}

		claim := idempotencyclaim.Claim{ID: g.ids.New(), Requester: key.Requester, Value: key.Value, Fingerprint: sha256.Sum256([]byte("new"))}
		ctx := idempotencyclaim.With(g.ctx(), claim)
		p := g.newProject("reclaimed")
		ctxAt := later
		p.CreatedAt, p.UpdatedAt = ctxAt, ctxAt
		if err := g.s.CreateProjectWithGrant(ctx, scope, p, g.bindingFor(p.ID, g.user.ID), []audit.Entry{audittest.Org(t, id.UUID(g.orgA), "project.created")}); err != nil {
			t.Fatalf("claim over an expired record: %v", err)
		}
		rec, found, _ := g.s.LookupIdempotency(g.ctx(), scope, key, ctxAt)
		if !found || rec.Completed || rec.Fingerprint != claim.Fingerprint {
			t.Errorf("reclaimed record = %+v (found %v), want the new in-progress claim", rec, found)
		}
	})
}

// TestIdempotencyRecordIsScopedToItsOrganization: a key of one organization is not visible from
// another, even when the requester and the key text are the same.
func TestIdempotencyRecordIsScopedToItsOrganization(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		key := identity.IdempotencyKey{Requester: "PRINCIPAL:shared", Value: "key-shared"}
		if err := g.s.CompleteIdempotency(g.ctx(), scopeOf(g.orgA), key, identity.IdempotencyCompletion{ID: g.ids.New(), Response: identity.IdempotencyResponse{Status: 201}}, base); err != nil {
			t.Fatalf("complete in A: %v", err)
		}
		if _, found, err := g.s.LookupIdempotency(g.ctx(), scopeOf(g.orgB), key, base); err != nil || found {
			t.Fatalf("organization B sees the record of A: found %v, err %v", found, err)
		}
	})
}

// TestCreateOrganizationWithAdminGrantsTheAdministrator (SPEC-004 R-ID-02): the organization, the
// administrator's active membership and Admin binding, and the entries of the creation are one write.
func TestCreateOrganizationWithAdminGrantsTheAdministrator(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		org := identity.OrgRecord{ID: next[identity.OrgID](g.fixture), Name: "Acme", Slug: "acme", Status: identity.OrgActive, CreatedAt: base, UpdatedAt: base}
		m := identity.MembershipRecord{ID: identity.MembershipID(g.ids.New()), OrgID: org.ID, UserID: g.user.ID, Status: identity.MembershipActive, CreatedAt: base, UpdatedAt: base}
		grant := identity.RoleBindingRecord{
			ID: identity.RoleBindingID(g.ids.New()), OrgID: org.ID, ScopeKind: identity.RoleScopeOrg, UserID: g.user.ID,
			RoleName: identity.RoleAdmin, CreatedAt: base, UpdatedAt: base,
		}
		entries := []audit.Entry{audittest.Org(t, id.UUID(org.ID), "org.created"), audittest.Org(t, id.UUID(org.ID), "member.joined"), audittest.Org(t, id.UUID(org.ID), "role_binding.created")}
		if err := g.s.CreateOrganizationWithAdmin(g.ctx(), org, m, grant, entries); err != nil {
			t.Fatalf("create organization with admin: %v", err)
		}
		scope := scopeOf(org.ID)
		if got, err := g.s.GetOrganization(g.ctx(), scope); err != nil || got.Slug != "acme" {
			t.Fatalf("get organization = %+v, %v", got, err)
		}
		active, err := g.s.ActiveMember(g.ctx(), g.user.ID, scope)
		if err != nil || !active {
			t.Fatalf("active member = %v, %v; want the administrator to be active", active, err)
		}
		bindings, err := g.s.Bindings(g.ctx(), g.user.ID, scope)
		if err != nil || len(bindings) != 1 || bindings[0].Role != identity.RoleAdmin || bindings[0].Project != "" {
			t.Fatalf("bindings = %+v, %v; want one organization Admin binding", bindings, err)
		}
	})
}

// TestCreateFlowWithEntryStartsInactiveWithAnEmptyDraft (SPEC-016 SH4, ADR-0080 section 3): the flow
// is inactive, its draft is the document given and its revision is zero; a repeated name is a conflict.
func TestCreateFlowWithEntryStartsInactiveWithAnEmptyDraft(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		rec := flow.Record{
			ID: next[flow.FlowID](g.fixture), OrgID: g.orgA, ProjectID: g.projA, Name: "Welcome",
			Draft: []byte(`{"format":"orcon/flow@1"}`), ActivationState: flow.ActivationInactive, CreatedAt: base, UpdatedAt: base,
		}
		entry := audittest.Org(t, id.UUID(g.orgA), "flow.created")
		if err := g.s.CreateFlowWithEntry(g.ctx(), scope, rec, entry); err != nil {
			t.Fatalf("create flow: %v", err)
		}
		got, err := g.s.GetFlow(g.ctx(), scope, rec.ID)
		if err != nil {
			t.Fatalf("get flow: %v", err)
		}
		if got.ActivationState != flow.ActivationInactive || got.DraftRevision != 0 || !bytes.Equal(got.Draft, rec.Draft) {
			t.Errorf("flow = %+v, want an inactive flow at draft revision 0", got)
		}
		dup := rec
		dup.ID = next[flow.FlowID](g.fixture)
		if err := g.s.CreateFlowWithEntry(g.ctx(), scope, dup, audittest.Org(t, id.UUID(g.orgA), "flow.created")); !errors.Is(err, flow.ErrConflict) {
			t.Errorf("duplicate name = %v, want ErrConflict", err)
		}
	})
}
