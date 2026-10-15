// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/identity"
)

// fakeGaps lists the organizations of a fake key store that have no active key.
type fakeGaps struct {
	store *fakeStore
	orgs  []identity.OrgID
	err   error
}

func (g fakeGaps) ListOrgsWithoutActiveKey(ctx context.Context, limit int) ([]identity.OrgID, error) {
	if g.err != nil {
		return nil, g.err
	}
	var out []identity.OrgID
	for _, org := range g.orgs {
		if _, err := g.store.ActiveKey(ctx, OrgScope(org)); errors.Is(err, ErrKeyNotFound) && len(out) < limit {
			out = append(out, org)
		}
	}
	return out, nil
}

// TestEnsureOrgKeysCreatesTheMissingOnesOnce: every organization without a key gets version 1,
// the ones with a key are left alone, and a second run has nothing to do.
func TestEnsureOrgKeysCreatesTheMissingOnesOnce(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newFakeStore()
	ring := NewKeyring(newMasterKey(t), store)
	have, missingA, missingB := newOrg(t), newOrg(t), newOrg(t)
	if _, err := ring.CreateKey(ctx, OrgScope(have)); err != nil {
		t.Fatalf("CreateKey = %v", err)
	}
	gaps := fakeGaps{store: store, orgs: []identity.OrgID{have, missingA, missingB}}

	var created []identity.OrgID
	if err := ring.EnsureOrgKeys(ctx, gaps, func(o identity.OrgID) { created = append(created, o) }); err != nil {
		t.Fatalf("EnsureOrgKeys = %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("created keys for %v, want the two organizations without one", created)
	}
	for _, org := range []identity.OrgID{have, missingA, missingB} {
		if k, err := store.ActiveKey(ctx, OrgScope(org)); err != nil || k.ID.Version != 1 {
			t.Errorf("active key of %v = %+v, %v; want version 1", org, k, err)
		}
	}
	created = nil
	if err := ring.EnsureOrgKeys(ctx, gaps, nil); err != nil || len(created) != 0 {
		t.Errorf("second run = %v, %v; want nothing to do", created, err)
	}
}

// TestEnsureOrgKeysReportsFailuresAndKeepsGoing: a listing failure and a failure to create one key
// are returned, and the other organizations still get theirs.
func TestEnsureOrgKeysReportsFailuresAndKeepsGoing(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newFakeStore()
	ring := NewKeyring(newMasterKey(t), store, WithRandom(zeroReader{}))
	org := newOrg(t)
	if err := ring.EnsureOrgKeys(ctx, fakeGaps{store: store, orgs: []identity.OrgID{org}}, nil); err == nil {
		t.Error("EnsureOrgKeys with a failing random source = nil, want the failure")
	}
	boom := errors.New("store down")
	if err := ring.EnsureOrgKeys(ctx, fakeGaps{store: store, err: boom}, nil); !errors.Is(err, boom) {
		t.Errorf("EnsureOrgKeys with a failing listing = %v, want %v", err, boom)
	}
}

// TestNewKeyStoresNothingAndOpens: a prepared key is returned for the caller to store; once stored
// it is the active key and wraps data keys.
func TestNewKeyStoresNothingAndOpens(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newFakeStore()
	ring := NewKeyring(newMasterKey(t), store)
	org := newOrg(t)
	stored, err := ring.NewKey(ctx, OrgScope(org))
	if err != nil {
		t.Fatalf("NewKey = %v", err)
	}
	if store.writes != 0 {
		t.Errorf("NewKey wrote %d times, want none", store.writes)
	}
	if err := store.PutKey(ctx, stored); err != nil {
		t.Fatalf("PutKey = %v", err)
	}
	if _, err := ring.GenerateDataKey(ctx, OrgScope(org)); err != nil {
		t.Errorf("GenerateDataKey under the prepared key = %v", err)
	}
	if _, err := ring.NewKey(ctx, Scope{Level: LevelMaster}); err == nil {
		t.Error("NewKey of the master level = nil, want an error")
	}
}
