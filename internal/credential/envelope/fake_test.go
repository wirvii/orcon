// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"bytes"
	"context"
	"crypto/rand"
	"sync"
	"testing"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// fakeStore is a hand-written in-memory KeyStore. It counts writes so tests
// can prove a code path did none.
type fakeStore struct {
	mu     sync.Mutex
	master *MasterCanary
	keys   map[KeyID]StoredKey
	writes int
	reads  int
}

func newFakeStore() *fakeStore { return &fakeStore{keys: make(map[KeyID]StoredKey)} }

func (f *fakeStore) MasterCanary(context.Context) (MasterCanary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.master == nil {
		return MasterCanary{}, ErrKeyNotFound
	}
	return *f.master, nil
}

func (f *fakeStore) ActiveKeys(context.Context) ([]StoredKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	var out []StoredKey
	for k := range f.keys {
		if f.keys[k].Status == KeyActive {
			out = append(out, f.keys[k])
		}
	}
	return out, nil
}

func (f *fakeStore) Key(_ context.Context, kid KeyID) (StoredKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	k, ok := f.keys[kid]
	if !ok {
		return StoredKey{}, ErrKeyNotFound
	}
	return k, nil
}

func (f *fakeStore) ActiveKey(_ context.Context, scope Scope) (StoredKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	for id := range f.keys {
		if k := f.keys[id]; k.ID.Scope() == scope && k.Status == KeyActive {
			return k, nil
		}
	}
	return StoredKey{}, ErrKeyNotFound
}

func (f *fakeStore) PutKey(_ context.Context, key StoredKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if _, ok := f.keys[key.ID]; ok {
		return ErrKeyExists
	}
	f.keys[key.ID] = key
	return nil
}

func (f *fakeStore) PutMasterCanary(_ context.Context, c MasterCanary) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if f.master != nil {
		return ErrKeyExists
	}
	f.master = &c
	return nil
}

func (f *fakeStore) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes
}

// countingMasterKey wraps a MasterKey and counts the unwraps it serves.
type countingMasterKey struct {
	MasterKey
	mu      sync.Mutex
	unwraps int
}

func (c *countingMasterKey) Unwrap(ctx context.Context, wrapped, aad []byte) ([]byte, error) {
	c.mu.Lock()
	c.unwraps++
	c.mu.Unlock()
	return c.MasterKey.Unwrap(ctx, wrapped, aad)
}

func (c *countingMasterKey) unwrapCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.unwraps
}

// orgSequence hands out distinct organization identities to every test.
var orgSequence = id.NewSequence()

// newOrg returns a fresh organization identity.
func newOrg(t *testing.T) identity.OrgID {
	t.Helper()
	return identity.OrgID(orgSequence.New())
}

// newMasterKey returns a local master key with random material, named like
// the file provider so startup messages read as in production.
func newMasterKey(t *testing.T) *LocalMasterKey {
	t.Helper()
	raw := make([]byte, KeySize)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("random master key: %v", err)
	}
	key, err := NewKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	return NewLocalMasterKey("file", "/test/"+Fingerprint(key)+".key", key)
}

func mustEqualBytes(t *testing.T, name string, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}
