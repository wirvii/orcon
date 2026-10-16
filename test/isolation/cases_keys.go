// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/identity"
)

// Key store (SPEC-004 R-KEY-01, R-ISO-10; ADR-0042): an organization key is visible only
// under its own organization; the master canary is written once.

// keyOfB is an organization key version 1 of B, wrapped by a fixed test value.
func keyOfB(env *Env) envelope.StoredKey {
	return envelope.StoredKey{
		ID:      envelope.KeyID{Level: envelope.LevelOrg, OrgID: identity.OrgID(env.B.UUID), Version: 1},
		Wrapped: []byte{1, 2, 3, 4}, MasterKeyReference: "file:isolation", Status: envelope.KeyActive,
		CreatedAt: seededAt, Canary: []byte{9, 9},
	}
}

// caseMasterCanary: the master canary is written once and read back.
func caseMasterCanary(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	probe := []byte{7, 7, 7}
	if err := env.Stores.PutMasterCanary(ctx, envelope.MasterCanary{Version: 1, Probe: probe}); err != nil {
		tb.Fatalf("put the master canary: %v", err)
	}
	if err := env.Stores.PutMasterCanary(ctx, envelope.MasterCanary{Version: 1, Probe: probe}); !errors.Is(err, envelope.ErrKeyExists) {
		tb.Fatalf("a second master canary: %v, want ErrKeyExists", err)
	}
	got, err := env.Stores.MasterCanary(ctx)
	if err != nil || got.Version != 1 || !bytes.Equal(got.Probe, probe) {
		tb.Fatalf("master canary = %+v, %v", got, err)
	}
}

// casePutKey: a key version is stored once.
func casePutKey(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.PutKey(ctx, keyOfB(env)); err != nil {
		tb.Fatalf("put the key of B: %v", err)
	}
	if err := env.Stores.PutKey(ctx, keyOfB(env)); !errors.Is(err, envelope.ErrKeyExists) {
		tb.Fatalf("the same key again: %v, want ErrKeyExists", err)
	}
}

// caseKey: the owner reads its key version; a key of another organization is not found.
func caseKey(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.PutKey(ctx, keyOfB(env)); err != nil {
		tb.Fatalf("put the key of B: %v", err)
	}
	got, err := env.Stores.Key(ctx, keyOfB(env).ID)
	if err != nil || got.ID != keyOfB(env).ID {
		tb.Fatalf("key of B = %+v, %v", got, err)
	}
	other := envelope.KeyID{Level: envelope.LevelOrg, OrgID: identity.OrgID(env.A.UUID), Version: 1}
	if _, err := env.Stores.Key(ctx, other); !errors.Is(err, envelope.ErrKeyNotFound) {
		tb.Fatalf("key of A: %v, want ErrKeyNotFound", err)
	}
}

// caseActiveKey: the active key of B is B's; A has none.
func caseActiveKey(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.PutKey(ctx, keyOfB(env)); err != nil {
		tb.Fatalf("put the key of B: %v", err)
	}
	got, err := env.Stores.ActiveKey(ctx, envelope.OrgScope(identity.OrgID(env.B.UUID)))
	if err != nil || got.ID.Version != 1 {
		tb.Fatalf("active key of B = %+v, %v", got, err)
	}
	if _, err := env.Stores.ActiveKey(ctx, envelope.OrgScope(identity.OrgID(env.A.UUID))); !errors.Is(err, envelope.ErrKeyNotFound) {
		tb.Fatalf("active key of A: %v, want ErrKeyNotFound", err)
	}
}

// caseListOrgsWithoutActiveKey: the startup repair read lists the organizations that have no active
// key, across organizations: B has a key, A has none.
func caseListOrgsWithoutActiveKey(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.PutKey(ctx, keyOfB(env)); err != nil {
		tb.Fatalf("put the key of B: %v", err)
	}
	got, err := env.Stores.ListOrgsWithoutActiveKey(ctx, 100)
	if err != nil {
		tb.Fatalf("organizations without a key: %v", err)
	}
	var sawA bool
	for _, org := range got {
		if org == identity.OrgID(env.B.UUID) {
			tb.Fatalf("organizations without a key = %v, which lists B although B has an active key", got)
		}
		sawA = sawA || org == identity.OrgID(env.A.UUID)
	}
	if !sawA {
		tb.Fatalf("organizations without a key = %v, want A listed", got)
	}
}

// caseActiveKeys: the verification read lists the active keys of every organization.
func caseActiveKeys(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.PutKey(ctx, keyOfB(env)); err != nil {
		tb.Fatalf("put the key of B: %v", err)
	}
	got, err := env.Stores.ActiveKeys(ctx)
	if err != nil || len(got) != 1 || got[0].ID.OrgID != identity.OrgID(env.B.UUID) {
		tb.Fatalf("active keys = %+v, %v; want only the key of B", got, err)
	}
	noLeak(tb, "the active keys", got, err, &env.A)
}
