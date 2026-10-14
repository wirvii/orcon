// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// initialized returns a store with a master canary, an instance key and the
// given number of organization keys, all created under mk.
func initialized(t *testing.T, mk MasterKey, orgs int) *fakeStore {
	t.Helper()
	ctx := t.Context()
	store := newFakeStore()
	ring := NewKeyring(mk, store)
	if err := ring.InitMasterCanary(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := ring.CreateKey(ctx, InstanceScope()); err != nil {
		t.Fatal(err)
	}
	for range orgs {
		if _, err := ring.CreateKey(ctx, OrgScope(newOrg(t))); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func TestStartupVerifiesHealthyStore(t *testing.T) {
	t.Parallel()
	mk := newMasterKey(t)
	store := initialized(t, mk, 3)
	rep, err := Verify(t.Context(), mk, store)
	if err != nil {
		t.Fatalf("Verify = %v, want nil", err)
	}
	if rep.Fresh || rep.ActiveKeys != 4 {
		t.Errorf("Verify report = %+v, want 4 active keys and not fresh", rep)
	}
}

func TestStartupFreshStore(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	rep, err := Verify(t.Context(), newMasterKey(t), store)
	if err != nil || !rep.Fresh {
		t.Errorf("Verify of an empty store = %+v, %v; want Fresh, nil", rep, err)
	}
	// Keys without a master canary are a corrupt store, not a first start.
	ring := NewKeyring(newMasterKey(t), store)
	if _, err := ring.CreateKey(t.Context(), InstanceScope()); err != nil {
		t.Fatal(err)
	}
	_, err = Verify(t.Context(), newMasterKey(t), store)
	if !errors.Is(err, ErrVerification) || !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Verify of keys without a master canary = %v, want ErrVerification and ErrKeyNotFound", err)
	}
}

func TestStartupRefusesWrongMasterKey(t *testing.T) {
	t.Parallel()
	right := newMasterKey(t)
	wrong := newMasterKey(t)
	store := initialized(t, right, 2)

	_, err := Verify(t.Context(), wrong, store)
	if !errors.Is(err, ErrVerification) || !errors.Is(err, ErrWrongMasterKey) {
		t.Fatalf("Verify with the wrong master key = %v, want ErrVerification and ErrWrongMasterKey", err)
	}
	var ve *VerifyError
	if !errors.As(err, &ve) {
		t.Fatalf("error %T is not a *VerifyError", err)
	}
	wantFP, _ := right.Fingerprint(t.Context())
	gotFP, _ := wrong.Fingerprint(t.Context())
	if ve.Provider != "file" || ve.Reference != wrong.Reference() || ve.ExpectedFingerprint != wantFP {
		t.Errorf("VerifyError = provider %q, reference %q, expected fingerprint %q; want file, %q, %q",
			ve.Provider, ve.Reference, ve.ExpectedFingerprint, wrong.Reference(), wantFP)
	}
	msg := err.Error()
	for _, want := range []string{"file", wrong.Reference(), wantFP} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q does not name %q", msg, want)
		}
	}
	// The message may show the configured key's fingerprint but never the
	// key bytes.
	if strings.Contains(msg, string(keyBytes(wrong))) {
		t.Error("error message contains the master key")
	}
	if gotFP == wantFP {
		t.Fatal("test setup: both master keys have the same fingerprint")
	}
}

func keyBytes(p *LocalMasterKey) []byte { return p.key.bytes()[:] }

func TestStartupRefusesTamperedKeys(t *testing.T) {
	t.Parallel()
	mk := newMasterKey(t)
	tests := map[string]func(s *fakeStore, k StoredKey){
		"corrupted wrapped key": func(s *fakeStore, k StoredKey) {
			k.Wrapped = append([]byte(nil), k.Wrapped...)
			k.Wrapped[len(k.Wrapped)-1] ^= 1
			s.keys[k.ID] = k
		},
		"canary of another key": func(s *fakeStore, k StoredKey) {
			for id, other := range s.keys {
				if id != k.ID {
					k.Canary = other.Canary
				}
			}
			s.keys[k.ID] = k
		},
		"corrupted canary": func(s *fakeStore, k StoredKey) {
			k.Canary = append([]byte(nil), k.Canary...)
			k.Canary[len(k.Canary)-1] ^= 1
			s.keys[k.ID] = k
		},
	}
	for name, tamper := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := initialized(t, mk, 1)
			var target StoredKey
			for _, k := range store.keys {
				if k.ID.Level == LevelOrg {
					target = k
				}
			}
			tamper(store, target)
			_, err := Verify(t.Context(), mk, store)
			var ve *VerifyError
			if !errors.As(err, &ve) {
				t.Fatalf("Verify = %v, want a *VerifyError", err)
			}
			if ve.Key != target.ID {
				t.Errorf("VerifyError.Key = %v, want %v", ve.Key, target.ID)
			}
		})
	}
}

func TestStartupRefusesBrokenMasterCanary(t *testing.T) {
	t.Parallel()
	mk := newMasterKey(t)
	store := initialized(t, mk, 0)
	store.master.Probe = []byte{9, 9, 9}
	if _, err := Verify(t.Context(), mk, store); !errors.Is(err, ErrMalformed) {
		t.Errorf("Verify with a malformed master canary = %v, want ErrMalformed", err)
	}
}

// writeTrapStore fails the test on any write and records the order of reads.
type writeTrapStore struct {
	*fakeStore
	t *testing.T
}

func (w writeTrapStore) PutKey(context.Context, StoredKey) error {
	w.t.Error("PutKey called during startup verification")
	return errors.New("write during verification")
}

func (w writeTrapStore) PutMasterCanary(context.Context, MasterCanary) error {
	w.t.Error("PutMasterCanary called during startup verification")
	return errors.New("write during verification")
}

func TestStartupVerifiesBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	mk := newMasterKey(t)
	store := initialized(t, mk, 2)
	before := store.writeCount()
	trap := writeTrapStore{fakeStore: store, t: t}

	// Verify accepts only the read port, so a store that traps writes is
	// passed as a KeyReader: the compiler, not discipline, rules writes out.
	var reader KeyReader = trap
	for _, key := range []MasterKey{mk, newMasterKey(t)} {
		_, _ = Verify(t.Context(), key, reader)
	}
	if got := store.writeCount(); got != before {
		t.Errorf("store writes during verification = %d, want %d", got-before, 0)
	}
	if store.reads == 0 {
		t.Error("Verify read nothing from the store")
	}
}

func TestMasterProbeRoundTrip(t *testing.T) {
	t.Parallel()
	probe, err := encodeMasterProbe("abc123", []byte("wrapped"))
	if err != nil {
		t.Fatal(err)
	}
	fp, wrapped, err := decodeMasterProbe(probe)
	if err != nil || fp != "abc123" || string(wrapped) != "wrapped" {
		t.Errorf("decodeMasterProbe = %q, %q, %v", fp, wrapped, err)
	}
	for name, bad := range map[string][]byte{
		"empty": nil, "wrong version": {2, 1, 'a'}, "zero length": {1, 0}, "truncated": {1, 5, 'a'},
	} {
		if _, _, err := decodeMasterProbe(bad); !errors.Is(err, ErrMalformed) {
			t.Errorf("decodeMasterProbe(%s) = %v, want ErrMalformed", name, err)
		}
	}
	if _, err := encodeMasterProbe("", nil); !errors.Is(err, ErrMalformed) {
		t.Errorf("encodeMasterProbe(empty fingerprint) = %v, want ErrMalformed", err)
	}
}

func TestMasterCanaryFingerprintIsAuthenticated(t *testing.T) {
	t.Parallel()
	mk := newMasterKey(t)
	actual, _ := mk.Fingerprint(t.Context())
	id := KeyID{Level: LevelMaster, Version: 1}

	t.Run("right key, edited stored fingerprint", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		// The authenticated data names the real fingerprint; the stored copy
		// is changed. Only the binding of the fingerprint into the data can
		// tell this from a wrong key.
		store := initialized(t, mk, 0)
		store.master.Probe = bytes.Clone(store.master.Probe)
		store.master.Probe[2] ^= 1 // first character of the stored fingerprint
		_, err := Verify(ctx, mk, store)
		if !errors.Is(err, ErrCanaryTampered) || errors.Is(err, ErrWrongMasterKey) {
			t.Errorf("Verify = %v, want ErrCanaryTampered and not ErrWrongMasterKey", err)
		}
	})
	t.Run("wrong key", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store := initialized(t, mk, 0)
		_, err := Verify(ctx, newMasterKey(t), store)
		if !errors.Is(err, ErrWrongMasterKey) || errors.Is(err, ErrCanaryTampered) {
			t.Errorf("Verify = %v, want ErrWrongMasterKey and not ErrCanaryTampered", err)
		}
	})
	t.Run("stored fingerprint matches but data names another", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		wrapped, err := mk.Wrap(ctx, []byte(canaryPlaintext), masterCanaryAAD(id, strings.Repeat("0", 32)))
		if err != nil {
			t.Fatal(err)
		}
		probe, err := encodeMasterProbe(actual, wrapped)
		if err != nil {
			t.Fatal(err)
		}
		store := newFakeStore()
		store.master = &MasterCanary{Version: 1, Probe: probe}
		_, err = Verify(ctx, mk, store)
		if !errors.Is(err, ErrCanaryTampered) || errors.Is(err, ErrWrongMasterKey) {
			t.Errorf("Verify = %v, want ErrCanaryTampered and not ErrWrongMasterKey", err)
		}
	})
}

func TestMasterCanaryRejectsUnknownFormatVersion(t *testing.T) {
	t.Parallel()
	mk := newMasterKey(t)
	store := initialized(t, mk, 0)
	store.master.Probe = bytes.Clone(store.master.Probe)
	store.master.Probe[0] = 2
	_, err := Verify(t.Context(), mk, store)
	if !errors.Is(err, ErrMalformed) || !strings.Contains(err.Error(), "version 2") {
		t.Errorf("Verify with canary format 2 = %v, want ErrMalformed naming version 2", err)
	}
}
