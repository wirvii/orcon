// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/credential/envelope"
)

// keyringOf builds the keyring of a fixture over a fresh local master key.
func keyringOf(t *testing.T, f *fixture) (*envelope.Keyring, envelope.MasterKey) { //nolint:ireturn // the envelope API hands out the master key port
	t.Helper()
	key, err := envelope.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("master key: %v", err)
	}
	mk := envelope.NewLocalMasterKey("file", "storetest-master", key)
	return envelope.NewKeyring(mk, f.s), mk
}

// TestKeyHierarchyRoundTrip (SPEC-004 R-KEY-01, R-KEY-05; SE3b): the keyring creates the
// master canary and the instance and organization keys through the store, wraps data
// keys under them, and verification at startup reads them back.
func TestKeyHierarchyRoundTrip(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		kr, mk := keyringOf(t, f)

		rep, err := envelope.Verify(ctx, mk, f.s)
		if err != nil || !rep.Fresh {
			t.Fatalf("verify of an empty store = %+v, %v; want a fresh store", rep, err)
		}
		if err := kr.InitMasterCanary(ctx); err != nil {
			t.Fatalf("master canary: %v", err)
		}
		if err := kr.InitMasterCanary(ctx); !errors.Is(err, envelope.ErrKeyExists) {
			t.Errorf("second master canary: %v, want ErrKeyExists", err)
		}
		if _, err := kr.CreateKey(ctx, envelope.InstanceScope()); err != nil {
			t.Fatalf("instance key: %v", err)
		}
		orgKey, err := kr.CreateKey(ctx, envelope.OrgScope(f.orgA))
		if err != nil {
			t.Fatalf("organization key of A: %v", err)
		}
		if _, err := kr.CreateKey(ctx, envelope.OrgScope(f.orgA)); !errors.Is(err, envelope.ErrKeyExists) {
			t.Errorf("second organization key of A: %v, want ErrKeyExists", err)
		}

		dk, err := kr.GenerateDataKey(ctx, envelope.OrgScope(f.orgA))
		if err != nil {
			t.Fatalf("data key: %v", err)
		}
		if dk.ID != orgKey {
			t.Errorf("data key wrapped by %v, want the organization key %v", dk.ID, orgKey)
		}
		back, err := kr.UnwrapDataKey(ctx, dk.ID, dk.Wrapped)
		if err != nil {
			t.Fatalf("unwrap the data key: %v", err)
		}
		if envelope.Fingerprint(back) != envelope.Fingerprint(dk.Key) {
			t.Error("the unwrapped data key differs from the one generated")
		}

		rep, err = envelope.Verify(ctx, mk, f.s)
		if err != nil || rep.Fresh || rep.ActiveKeys != 2 {
			t.Errorf("verify after the ceremony = %+v, %v; want two active keys", rep, err)
		}
		other := envelope.NewLocalMasterKey("file", "storetest-other", mustKey(t))
		if _, err := envelope.Verify(ctx, other, f.s); !errors.Is(err, envelope.ErrVerification) {
			t.Errorf("verify with another master key: %v, want ErrVerification", err)
		}
	})
}

// TestOrganizationKeysAreIsolated (SPEC-004 R-ISO-10): the active key of an organization is
// not visible under another organization's scope.
func TestOrganizationKeysAreIsolated(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		kr, mk := keyringOf(t, f)
		if err := kr.InitMasterCanary(f.ctx()); err != nil {
			t.Fatalf("master canary: %v", err)
		}
		if _, err := kr.CreateKey(f.ctx(), envelope.OrgScope(f.orgB)); err != nil {
			t.Fatalf("organization key of B: %v", err)
		}
		if _, err := f.s.ActiveKey(f.ctx(), envelope.OrgScope(f.orgA)); !errors.Is(err, envelope.ErrKeyNotFound) {
			t.Errorf("active key of A: %v, want ErrKeyNotFound", err)
		}
		if _, err := envelope.Verify(f.ctx(), mk, f.s); err != nil {
			t.Errorf("verify: %v", err)
		}
	})
}

func mustKey(t *testing.T) envelope.Key {
	t.Helper()
	key, err := envelope.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	return key
}
