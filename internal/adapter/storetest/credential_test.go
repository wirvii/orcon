// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// credentialFixture builds the credential service over the store of f and the real keyring (ADR-0042), with an
// organization key for each organization.
func credentialFixture(t *testing.T, f *fixture) *credential.Service {
	t.Helper()
	ctx := f.ctx()
	kr, _ := keyringOf(t, f)
	if err := kr.InitMasterCanary(ctx); err != nil {
		t.Fatalf("master canary: %v", err)
	}
	for _, org := range []identity.OrgID{f.orgA, f.orgB} {
		if _, err := kr.CreateKey(ctx, envelope.OrgScope(org)); err != nil {
			t.Fatalf("organization key of %v: %v", org, err)
		}
	}
	svc, err := credential.New(credential.Config{Store: f.s, Keys: kr, Clock: clock.NewFake(base), IDs: f.ids})
	if err != nil {
		t.Fatalf("credential service: %v", err)
	}
	return svc
}

// TestCredentialStoreRoundTrip (SPEC-004 SE7, R-CRED-01..03, R-AZ-07): a credential is sealed under the organization
// key, the dispatch reads it back, a replacement creates a version, a share lets another project use it without
// reading it, and removing the share stops the next use. The rows never hold the secret in clear.
func TestCredentialStoreRoundTrip(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		svc := credentialFixture(t, f)
		scopeA := scopeOf(f.orgA)
		projA2 := f.project(f.orgA, "beta")
		owner := audit.Actor{PrincipalID: id.UUID(f.user("owner-credential@example.com").PrincipalID), Kind: audit.ActorUser}

		const hiddenValue = "https://hooks.example.com/services/T000/B000/SECRET-ROUND-TRIP"
		created, err := svc.Create(ctx, scopeA, f.projA, credential.Input{
			Name: "Ops webhook", ConnectorReference: "http", AllowedHosts: []string{"hooks.example.com"},
			Public: map[string]string{"channel": "#ops"}, Secret: map[string]string{"webhook": hiddenValue},
		}, owner)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		res, err := svc.Resolve(ctx, scopeA, f.projA, created.ID)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if v, _ := res.Secret("webhook"); v != hiddenValue {
			t.Fatal("the dispatch did not read back the hiddenValue")
		}
		ver, err := f.s.GetCredentialVersion(ctx, scopeA, created.ID, 1)
		if err != nil {
			t.Fatalf("stored version: %v", err)
		}
		if bytes.Contains(ver.SecretCiphertext, []byte(hiddenValue)) {
			t.Fatal("the stored ciphertext holds the hiddenValue in clear")
		}
		if ver.OrgKeyVersion != 1 {
			t.Errorf("org key version = %d, want 1", ver.OrgKeyVersion)
		}

		if _, err := svc.Replace(ctx, scopeA, f.projA, created.ID, credential.Input{
			Public: map[string]string{"channel": "#ops"}, Secret: map[string]string{"webhook": hiddenValue + "-2"},
		}, owner); err != nil {
			t.Fatalf("replace: %v", err)
		}
		res, err = svc.Resolve(ctx, scopeA, f.projA, created.ID)
		if err != nil || res.Version != 2 {
			t.Fatalf("resolve after replace = version %d, %v; want 2", res.Version, err)
		}

		if err := svc.Share(ctx, scopeA, f.projA, created.ID, projA2, owner); err != nil {
			t.Fatalf("share within the organization: %v", err)
		}
		if _, err := svc.Resolve(ctx, scopeA, projA2, created.ID); err != nil {
			t.Fatalf("the shared project cannot use the credential: %v", err)
		}
		seen, err := svc.Get(ctx, scopeA, projA2, created.ID)
		if err != nil || seen.Owned || !seen.Secret.Set || seen.Public["channel"] != "#ops" {
			t.Fatalf("shared view = %+v, %v; want not owned, hiddenValue set, public field readable", seen, err)
		}
		for _, v := range seen.Public {
			if strings.Contains(v, hiddenValue) {
				t.Fatal("the shared view carries the hiddenValue")
			}
		}

		if err := svc.Share(ctx, scopeA, f.projA, created.ID, f.projB, owner); !errors.Is(err, credential.ErrNotFound) {
			t.Errorf("share to a project of another organization = %v, want ErrNotFound", err)
		}

		if err := svc.Unshare(ctx, scopeA, f.projA, created.ID, projA2, owner); err != nil {
			t.Fatalf("unshare: %v", err)
		}
		if _, err := svc.Resolve(ctx, scopeA, projA2, created.ID); !errors.Is(err, credential.ErrNotUsable) {
			t.Errorf("the next use after the unshare = %v, want ErrNotUsable", err)
		}
	})
}

// TestCredentialStoreCompareAndSwapOnSettingsAndDelete (SPEC-004 SE7, PR 165 review): the revision a settings change or
// a delete carries is compared in the write itself, so concurrent calls that hold the same revision cannot both win,
// and a change made after the revision was read is refused with ErrConflict.
func TestCredentialStoreCompareAndSwapOnSettingsAndDelete(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		svc := credentialFixture(t, f)
		scopeA := scopeOf(f.orgA)
		owner := audit.Actor{PrincipalID: id.UUID(f.user("owner-cas@example.com").PrincipalID), Kind: audit.ActorUser}

		const racers = 8
		for _, op := range []string{"update", "delete"} {
			created, err := svc.Create(ctx, scopeA, f.projA, credential.Input{
				Name: "Racing " + op, ConnectorReference: "http", AllowedHosts: []string{"api.example.com"},
				Secret: map[string]string{"token": "race-" + op},
			}, owner)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			seen := created.UpdatedAt
			var wins, conflicts, other atomic.Int32
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := range racers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					var err error
					if op == "update" {
						_, err = svc.Update(ctx, scopeA, f.projA, created.ID, credential.Settings{
							Name: fmt.Sprintf("Racing update %d", i), AllowedHosts: []string{"api.example.com"},
						}, seen, owner)
					} else {
						err = svc.Delete(ctx, scopeA, f.projA, created.ID, seen, owner)
					}
					switch {
					case err == nil:
						wins.Add(1)
					case errors.Is(err, credential.ErrConflict), errors.Is(err, credential.ErrNotFound):
						conflicts.Add(1)
					default:
						other.Add(1)
						t.Errorf("%s: unexpected error: %v", op, err)
					}
				}()
			}
			close(start)
			wg.Wait()
			if wins.Load() != 1 || conflicts.Load() != racers-1 {
				t.Fatalf("%s with the same revision: %d wins and %d refusals, want 1 and %d", op, wins.Load(), conflicts.Load(), racers-1)
			}
		}

		// A revision read before another change is stale, for the settings and for the delete.
		created, err := svc.Create(ctx, scopeA, f.projA, credential.Input{
			Name: "Stale revision", ConnectorReference: "http", AllowedHosts: []string{"api.example.com"},
			Secret: map[string]string{"token": "stale"},
		}, owner)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := svc.Update(ctx, scopeA, f.projA, created.ID, credential.Settings{Name: "Renamed once", AllowedHosts: []string{"api.example.com"}}, created.UpdatedAt, owner); err != nil {
			t.Fatalf("first update: %v", err)
		}
		if _, err := svc.Update(ctx, scopeA, f.projA, created.ID, credential.Settings{Name: "Renamed twice", AllowedHosts: []string{"api.example.com"}}, created.UpdatedAt, owner); !errors.Is(err, credential.ErrConflict) {
			t.Fatalf("update with a stale revision = %v, want ErrConflict", err)
		}
		if err := svc.Delete(ctx, scopeA, f.projA, created.ID, created.UpdatedAt, owner); !errors.Is(err, credential.ErrConflict) {
			t.Fatalf("delete with a stale revision = %v, want ErrConflict", err)
		}
		got, err := svc.Get(ctx, scopeA, f.projA, created.ID)
		if err != nil || got.Name != "Renamed once" {
			t.Fatalf("the credential after the refused calls = %q, %v; want the first rename", got.Name, err)
		}
	})
}

// TestCredentialStoreRejectsStaleAndDuplicate (SPEC-004 SE7, R-CRED-02): two replacements that read the same current
// version cannot both win, and a second credential with the same name in one project is a conflict.
func TestCredentialStoreRejectsStaleAndDuplicate(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		svc := credentialFixture(t, f)
		scopeA := scopeOf(f.orgA)
		owner := audit.Actor{PrincipalID: id.UUID(f.user("owner-stale@example.com").PrincipalID), Kind: audit.ActorUser}
		in := credential.Input{Name: "Token", ConnectorReference: "http", AllowedHosts: []string{"api.example.com"},
			Secret: map[string]string{"token": "first"}}
		created, err := svc.Create(ctx, scopeA, f.projA, in, owner)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		// The audit entry of the creation is written with the credential (R-AUD-02); a refused creation leaves none.
		pendingBefore := pendingAuditActions(t, f, scopeA)
		if !slices.Contains(pendingBefore, "credential.created") {
			t.Errorf("the creation left no credential.created entry: %v", pendingBefore)
		}
		if _, err := svc.Create(ctx, scopeA, f.projA, in, owner); !errors.Is(err, credential.ErrConflict) {
			t.Errorf("the same name twice = %v, want ErrConflict", err)
		}
		if got := pendingAuditActions(t, f, scopeA); len(got) != len(pendingBefore) {
			t.Errorf("a refused creation left audit entries: %v, before %v", got, pendingBefore)
		}

		first := credentialVersion(f, scopeA, created.ID, 2)
		if err := f.s.AddVersion(ctx, scopeA, created.ID, 0, first, credential.StatusOK, base, audittest.Org(t, scopeA.UUID(), "credential.updated")); !errors.Is(err, credential.ErrConflict) {
			t.Errorf("a version written on a stale current version = %v, want ErrConflict", err)
		}
		if err := f.s.AddVersion(ctx, scopeA, created.ID, 1, first, credential.StatusOK, base, audittest.Org(t, scopeA.UUID(), "credential.updated")); err != nil {
			t.Fatalf("the version on the current pointer: %v", err)
		}
		if err := f.s.AddVersion(ctx, scopeA, created.ID, 1, credentialVersion(f, scopeA, created.ID, 2), credential.StatusOK, base, audittest.Org(t, scopeA.UUID(), "credential.updated")); err == nil {
			t.Error("two versions both claimed version 2")
		}
	})
}

// pendingAuditActions returns the actions of the organization's audit entries that wait to be sealed.
func pendingAuditActions(t *testing.T, f *fixture, org orgscope.Org) []string {
	t.Helper()
	entries, err := f.s.ListPending(f.ctx(), audit.OrganizationChain(org.UUID()), 1000)
	if err != nil {
		t.Fatalf("list pending audit entries: %v", err)
	}
	out := make([]string, 0, len(entries))
	for i := range entries {
		out = append(out, entries[i].Action)
	}
	return out
}

// credentialVersion is a version row for the store tests; the store never opens the ciphertext.
func credentialVersion(f *fixture, org orgscope.Org, cred id.UUID, number int) credential.Version {
	return credential.Version{
		ID: f.ids.New(), OrgID: identity.OrgIDOf(org), CredentialID: cred, Number: number,
		PublicFields: []byte(`{}`), SecretCiphertext: bytes.Repeat([]byte{7}, 40), DataKeyWrapped: []byte{1, 2, 3},
		OrgKeyVersion: 1, CreatedAt: base,
	}
}
