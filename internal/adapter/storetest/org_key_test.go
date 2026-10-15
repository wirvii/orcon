// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

// keyringIssuer prepares the first key of a new organization from a keyring, as cmd/orcon does.
type keyringIssuer struct{ kr *envelope.Keyring }

func (i keyringIssuer) NewOrgKey(ctx context.Context, org identity.OrgID) (identity.OrgKeyRecord, error) {
	k, err := i.kr.NewKey(ctx, envelope.OrgScope(org))
	if err != nil {
		return identity.OrgKeyRecord{}, fmt.Errorf("new key: %w", err)
	}
	return identity.OrgKeyRecord{
		Version: k.ID.Version, Wrapped: k.Wrapped, MasterKeyReference: k.MasterKeyReference,
		Status: string(k.Status), Probe: k.Canary, CreatedAt: k.CreatedAt,
	}, nil
}

// macKeys gives the audit sealer the MAC keys of each chain from a keyring, as cmd/orcon does.
type macKeys struct{ kr *envelope.Keyring }

func scopeOfChain(c audit.Chain) envelope.Scope {
	if c.IsInstance() {
		return envelope.InstanceScope()
	}
	return envelope.OrgScope(identity.OrgID(c.Org()))
}

func (m macKeys) ActiveMACKey(ctx context.Context, c audit.Chain) (int64, []byte, error) {
	kid, sub, err := m.kr.ActiveSubkey(ctx, scopeOfChain(c), audit.MACPurpose)
	if err != nil {
		return 0, nil, fmt.Errorf("active audit key: %w", err)
	}
	return int64(kid.Version), sub, nil
}

func (m macKeys) MACKey(ctx context.Context, c audit.Chain, version int64) ([]byte, error) {
	kid := envelope.KeyID{Level: envelope.LevelInstance, Version: int(version)}
	if !c.IsInstance() {
		kid = envelope.KeyID{Level: envelope.LevelOrg, OrgID: identity.OrgID(c.Org()), Version: int(version)}
	}
	sub, err := m.kr.SubkeyFor(ctx, kid, audit.MACPurpose)
	if err != nil {
		return nil, fmt.Errorf("audit key version %d: %w", version, err)
	}
	return sub, nil
}

// TestOrganizationCreatedWithItsKeyIsSealed (SPEC-004 R-KEY-05, SE3b, SE8): creating an organization
// through the service stores its active key in the same transaction, so the audit entries of the
// creation seal at once and the key wraps data keys.
func TestOrganizationCreatedWithItsKeyIsSealed(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		ctx := g.ctx()
		kr, _ := keyringOf(t, g.fixture)
		if err := kr.InitMasterCanary(ctx); err != nil {
			t.Fatalf("master canary: %v", err)
		}
		// The organizations of the fixture are not under test; give them keys so the whole pass seals.
		for _, org := range []identity.OrgID{g.orgA, g.orgB} {
			if _, err := kr.CreateKey(ctx, envelope.OrgScope(org)); err != nil {
				t.Fatalf("organization key: %v", err)
			}
		}
		svc := identity.NewOrganizations(g.s, keyringIssuer{kr: kr}, clock.System{}, id.NewV7Generator(time.Now))
		org, err := svc.Create(ctx, identity.NewOrganization{Name: "Wirvii", Slug: "wirvii", AdminEmail: "creator@example.com"})
		if err != nil {
			t.Fatalf("create organization: %v", err)
		}
		active, err := g.s.ActiveKey(ctx, envelope.OrgScope(org.ID))
		if err != nil || active.Status != envelope.KeyActive || active.ID.Version != 1 {
			t.Fatalf("active key of the new organization = %+v, %v; want version 1 active", active, err)
		}
		if _, err := kr.GenerateDataKey(ctx, envelope.OrgScope(org.ID)); err != nil {
			t.Fatalf("data key under the new organization: %v", err)
		}
		gaps, err := g.s.ListOrgsWithoutActiveKey(ctx, 100)
		if err != nil || len(gaps) != 0 {
			t.Fatalf("organizations without a key = %v, %v; want none", gaps, err)
		}
		sealer := audit.NewSealer(g.s, macKeys{kr: kr}, id.NewV7Generator(time.Now), clock.System{})
		if _, err := sealer.SealOnce(ctx); err != nil {
			t.Fatalf("seal: %v", err)
		}
		tail, err := g.s.Tail(ctx, audit.OrganizationChain(id.UUID(org.ID)))
		if err != nil || tail.Sequence != 3 {
			t.Errorf("tail of the new organization's chain = %+v, %v; want 3 sealed entries (created, joined, role binding)", tail, err)
		}
	})
}

// TestOrganizationsWithoutAKeyAreRepaired (SPEC-004 R-KEY-05): the startup repair gives a key to
// every organization that lacks one, skips the ones scheduled for deletion and the ones that have a
// key, and is idempotent. After the repair the organization's chain seals.
func TestOrganizationsWithoutAKeyAreRepaired(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		kr, mk := keyringOf(t, f)
		if err := kr.InitMasterCanary(ctx); err != nil {
			t.Fatalf("master canary: %v", err)
		}
		if _, err := kr.CreateKey(ctx, envelope.OrgScope(f.orgB)); err != nil {
			t.Fatalf("organization key of B: %v", err)
		}
		gone := identity.OrgRecord{
			ID: next[identity.OrgID](f), Name: "Gone", Slug: "gone", Status: identity.OrgDeletionScheduled,
			DeletionScheduledAt: base.Add(time.Hour), CreatedAt: base, UpdatedAt: base,
		}
		if err := f.s.CreateOrganization(ctx, gone); err != nil {
			t.Fatalf("create the organization scheduled for deletion: %v", err)
		}

		gaps, err := f.s.ListOrgsWithoutActiveKey(ctx, 100)
		if err != nil || len(gaps) != 1 || gaps[0] != f.orgA {
			t.Fatalf("organizations without a key = %v, %v; want only A", gaps, err)
		}
		var repaired []identity.OrgID
		if err := kr.EnsureOrgKeys(ctx, f.s, func(o identity.OrgID) { repaired = append(repaired, o) }); err != nil {
			t.Fatalf("ensure organization keys: %v", err)
		}
		if len(repaired) != 1 || repaired[0] != f.orgA {
			t.Fatalf("repaired = %v, want only A", repaired)
		}
		if _, err := f.s.ActiveKey(ctx, envelope.OrgScope(f.orgA)); err != nil {
			t.Fatalf("active key of A after the repair: %v", err)
		}
		if _, err := f.s.ActiveKey(ctx, envelope.OrgScope(gone.ID)); !errors.Is(err, envelope.ErrKeyNotFound) {
			t.Errorf("active key of the organization scheduled for deletion = %v, want ErrKeyNotFound", err)
		}
		repaired = nil
		if err := kr.EnsureOrgKeys(ctx, f.s, func(o identity.OrgID) { repaired = append(repaired, o) }); err != nil || len(repaired) != 0 {
			t.Errorf("second repair = %v, %v; want nothing to repair", repaired, err)
		}
		if _, err := envelope.Verify(ctx, mk, f.s); err != nil {
			t.Errorf("verify after the repair: %v", err)
		}
		sealer := audit.NewSealer(f.s, macKeys{kr: kr}, id.NewV7Generator(time.Now), clock.System{})
		if _, err := sealer.SealOnce(ctx); err != nil {
			t.Errorf("seal after the repair: %v", err)
		}
	})
}
