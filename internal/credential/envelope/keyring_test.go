// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/clock"
)

func TestHierarchyMKtoOKtoDEK(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	mk := newMasterKey(t)
	store := newFakeStore()
	ring := NewKeyring(mk, store)

	if err := ring.InitMasterCanary(ctx); err != nil {
		t.Fatalf("InitMasterCanary = %v", err)
	}
	org := newOrg(t)
	okID, err := ring.CreateKey(ctx, OrgScope(org))
	if err != nil {
		t.Fatalf("CreateKey(org) = %v", err)
	}
	ikID, err := ring.CreateKey(ctx, InstanceScope())
	if err != nil {
		t.Fatalf("CreateKey(instance) = %v", err)
	}
	if want := (KeyID{Level: LevelOrg, OrgID: org, Version: 1}); okID != want {
		t.Errorf("organization key id = %v, want %v", okID, want)
	}
	if ikID.Level != LevelInstance || ikID.Version != 1 {
		t.Errorf("instance key id = %v, want INSTANCE v1", ikID)
	}

	// The store holds only wrapped material: the OK is not recoverable
	// without the master key.
	stored, err := store.Key(ctx, okID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Wrapped) != nonceSize+KeySize+tagSize || stored.MasterKeyReference != mk.Reference() || stored.Status != KeyActive {
		t.Errorf("stored key = %d wrapped bytes, ref %q, status %s", len(stored.Wrapped), stored.MasterKeyReference, stored.Status)
	}

	for _, scope := range []Scope{OrgScope(org), InstanceScope()} {
		dk, err := ring.GenerateDataKey(ctx, scope)
		if err != nil {
			t.Fatalf("GenerateDataKey(%s) = %v", scope.Level, err)
		}
		aad := AAD{OrgID: scope.OrgID, ObjectType: "credential", ObjectID: "c1", Version: 1, Field: "secret"}
		env, err := Seal(dk.Key, dk.ID, aad, []byte("hunter2"))
		if err != nil {
			t.Fatalf("Seal = %v", err)
		}

		// A fresh process: new keyring, same store and same master key.
		restarted := NewKeyring(mk, store)
		dek, err := restarted.UnwrapDataKey(ctx, env.KID, dk.Wrapped)
		if err != nil {
			t.Fatalf("UnwrapDataKey after restart = %v", err)
		}
		got, err := env.Open(dek, aad)
		if err != nil || string(got) != "hunter2" {
			t.Errorf("Open after restart = %q, %v; want hunter2, nil", got, err)
		}

		// Another master key cannot walk the hierarchy.
		stranger := NewKeyring(newMasterKey(t), store)
		if _, err := stranger.UnwrapDataKey(ctx, env.KID, dk.Wrapped); !errors.Is(err, ErrDecrypt) {
			t.Errorf("UnwrapDataKey with another master key = %v, want ErrDecrypt", err)
		}
	}
}

func TestCiphertextCopiedAcrossOrgFails(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newFakeStore()
	ring := NewKeyring(newMasterKey(t), store)
	orgA, orgB := newOrg(t), newOrg(t)
	for _, o := range []Scope{OrgScope(orgA), OrgScope(orgB)} {
		if _, err := ring.CreateKey(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	dkA, err := ring.GenerateDataKey(ctx, OrgScope(orgA))
	if err != nil {
		t.Fatal(err)
	}
	aadA := AAD{OrgID: orgA, ObjectType: "credential", ObjectID: "c1", Version: 1, Field: "secret"}
	env, err := Seal(dkA.Key, dkA.ID, aadA, []byte("org A secret"))
	if err != nil {
		t.Fatal(err)
	}

	// 1. The row is copied into organization B and read with B's AAD.
	aadB := aadA
	aadB.OrgID = orgB
	if _, err := env.Open(dkA.Key, aadB); !errors.Is(err, ErrDecrypt) {
		t.Errorf("Open with organization B AAD = %v, want ErrDecrypt", err)
	}
	// 2. The wrapped data key is relabelled with B's key version.
	idB := KeyID{Level: LevelOrg, OrgID: orgB, Version: 1}
	if _, err := ring.UnwrapDataKey(ctx, idB, dkA.Wrapped); !errors.Is(err, ErrDecrypt) {
		t.Errorf("UnwrapDataKey of A's data key under B's key = %v, want ErrDecrypt", err)
	}
	// 3. B's data key, with B's AAD, cannot open A's ciphertext either.
	dkB, err := ring.GenerateDataKey(ctx, OrgScope(orgB))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Open(dkB.Key, aadB); !errors.Is(err, ErrDecrypt) {
		t.Errorf("Open with B's data key and AAD = %v, want ErrDecrypt", err)
	}
}

func TestKeyringErrors(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newFakeStore()
	ring := NewKeyring(newMasterKey(t), store)
	org := newOrg(t)

	if _, err := ring.GenerateDataKey(ctx, OrgScope(org)); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("GenerateDataKey without a key = %v, want ErrKeyNotFound", err)
	}
	if _, err := ring.CreateKey(ctx, Scope{Level: LevelMaster}); !errors.Is(err, ErrInvalidKeyID) {
		t.Errorf("CreateKey(master) = %v, want ErrInvalidKeyID", err)
	}
	if _, err := ring.CreateKey(ctx, Scope{Level: LevelOrg}); !errors.Is(err, ErrInvalidKeyID) {
		t.Errorf("CreateKey(org without id) = %v, want ErrInvalidKeyID", err)
	}
	if _, err := ring.CreateKey(ctx, OrgScope(org)); err != nil {
		t.Fatal(err)
	}
	if _, err := ring.CreateKey(ctx, OrgScope(org)); !errors.Is(err, ErrKeyExists) {
		t.Errorf("second CreateKey = %v, want ErrKeyExists", err)
	}
	if err := ring.InitMasterCanary(ctx); err != nil {
		t.Fatal(err)
	}
	if err := ring.InitMasterCanary(ctx); !errors.Is(err, ErrKeyExists) {
		t.Errorf("second InitMasterCanary = %v, want ErrKeyExists", err)
	}
	if _, err := ring.UnwrapDataKey(ctx, KeyID{Level: LevelMaster, Version: 1}, make([]byte, 60)); !errors.Is(err, ErrInvalidKeyID) {
		t.Errorf("UnwrapDataKey(master) = %v, want ErrInvalidKeyID", err)
	}
	missing := KeyID{Level: LevelOrg, OrgID: org, Version: 9}
	if _, err := ring.UnwrapDataKey(ctx, missing, make([]byte, 60)); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("UnwrapDataKey of a missing version = %v, want ErrKeyNotFound", err)
	}

	// A retired or destroyed key is not usable.
	id := KeyID{Level: LevelOrg, OrgID: org, Version: 1}
	for _, status := range []KeyStatus{KeyRetired, KeyDestroyed} {
		stored := store.keys[id]
		stored.Status = status
		store.keys[id] = stored
		cold := NewKeyring(ring.mk, store, WithCacheTTL(0))
		if _, err := cold.UnwrapDataKey(ctx, id, make([]byte, 60)); !errors.Is(err, ErrKeyUnavailable) {
			t.Errorf("UnwrapDataKey of a %s key = %v, want ErrKeyUnavailable", status, err)
		}
	}
}

func TestKeyCacheExpiresAndCanBeDisabled(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newFakeStore()
	mk := &countingMasterKey{MasterKey: newMasterKey(t)}
	clk := clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
	ring := NewKeyring(mk, store, WithClock(clk), WithCacheTTL(time.Hour))
	org := newOrg(t)
	if _, err := ring.CreateKey(ctx, OrgScope(org)); err != nil {
		t.Fatal(err)
	}

	// CreateKey leaves the key cached: no unwrap yet.
	for range 3 {
		if _, err := ring.GenerateDataKey(ctx, OrgScope(org)); err != nil {
			t.Fatal(err)
		}
	}
	if got := mk.unwrapCount(); got != 0 {
		t.Errorf("master key unwraps within the TTL = %d, want 0", got)
	}
	clk.Advance(time.Hour + time.Second)
	for range 3 {
		if _, err := ring.GenerateDataKey(ctx, OrgScope(org)); err != nil {
			t.Fatal(err)
		}
	}
	if got := mk.unwrapCount(); got != 1 {
		t.Errorf("master key unwraps after the TTL = %d, want 1 (re-unwrapped once, then cached)", got)
	}

	nocache := &countingMasterKey{MasterKey: mk.MasterKey}
	cold := NewKeyring(nocache, store, WithCacheTTL(0))
	for range 3 {
		if _, err := cold.GenerateDataKey(ctx, OrgScope(org)); err != nil {
			t.Fatal(err)
		}
	}
	if got := nocache.unwrapCount(); got != 3 {
		t.Errorf("master key unwraps with the cache disabled = %d, want 3", got)
	}
}

func TestKeyringRandomFailure(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ring := NewKeyring(newMasterKey(t), newFakeStore(), WithRandom(zeroReader{}))
	if _, err := ring.CreateKey(ctx, OrgScope(newOrg(t))); err == nil {
		t.Error("CreateKey with a failing random source = nil, want error")
	}
}

// TestKeyringCacheConcurrentRefresh hammers one key while its cache entry
// keeps expiring, so cache hits copy the cached bytes while refreshes wipe the
// entry they replace. A copy made outside the keyring's lock can read the
// entry after the wipe and yield an all-zero key, so the unwrap fails with a
// decryption error; that failure, not a race report, is what this test catches.
func TestKeyringCacheConcurrentRefresh(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ring := NewKeyring(newMasterKey(t), newFakeStore(), WithCacheTTL(20*time.Microsecond))
	if err := ring.InitMasterCanary(ctx); err != nil {
		t.Fatal(err)
	}
	scope := OrgScope(newOrg(t))
	if _, err := ring.CreateKey(ctx, scope); err != nil {
		t.Fatal(err)
	}
	dk, err := ring.GenerateDataKey(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	dk.Key.Destroy()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				key, err := ring.UnwrapDataKey(ctx, dk.ID, dk.Wrapped)
				if err != nil {
					t.Errorf("UnwrapDataKey = %v", err)
					return
				}
				key.Destroy()
			}
		}()
	}
	wg.Wait()
}

type zeroReader struct{}

func (zeroReader) Read([]byte) (int, error) { return 0, errors.New("entropy exhausted") }
