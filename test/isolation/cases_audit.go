// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"context"
	"testing"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
)

// isolationKeys gives every chain the same fixed MAC key: the cases test who can
// read a chain, not the key derivation (that is the audit package's own suite).
type isolationKeys struct{}

func (isolationKeys) ActiveMACKey(context.Context, audit.Chain) (int64, []byte, error) {
	return 1, fixedMACKey(), nil
}

func (isolationKeys) MACKey(context.Context, audit.Chain, int64) ([]byte, error) {
	return fixedMACKey(), nil
}

func fixedMACKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = 0x5A
	}
	return k
}

// caseAppendInstanceEntry: a denied or failed attempt is stored on the instance chain
// only; an organization entry handed to the instance method is refused.
func caseAppendInstanceEntry(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.AppendInstanceEntry(ctx, audittest.Instance(tb, "login.failed")); err != nil {
		tb.Fatalf("append an instance entry: %v", err)
	}
	wantErr(tb, "an organization entry on the instance method", env.Stores.AppendInstanceEntry(ctx, audittest.Org(tb, env.A.UUID, "login.failed")), audit.ErrInvalidEntry)
}

// caseAuditChain seals the pending entries the seed left in both organizations and checks
// that each chain reads back only its own events and verifies.
func caseAuditChain(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	chainA, chainB := audit.OrganizationChain(env.A.UUID), audit.OrganizationChain(env.B.UUID)
	chains, err := env.Stores.PendingChains(ctx, 100)
	if err != nil {
		tb.Fatalf("pending chains: %v", err)
	}
	if !containsChain(chains, chainA) || !containsChain(chains, chainB) {
		tb.Fatalf("pending chains %v lack an organization the seed wrote to", chains)
	}
	sealed, err := audit.NewSealer(env.Stores, isolationKeys{}, env.IDs, clock.NewFake(seededAt)).SealOnce(ctx)
	if err != nil || sealed == 0 {
		tb.Fatalf("seal: %d events, %v", sealed, err)
	}
	for _, c := range []audit.Chain{chainA, chainB, audit.InstanceChain()} {
		rep, err := audit.NewVerifier(env.Stores, isolationKeys{}).Verify(ctx, c, nil)
		if err != nil || !rep.OK() {
			tb.Fatalf("verify %s: %+v, %v", c.Label(), rep.Violations, err)
		}
	}
	events, err := env.Stores.ListSealed(ctx, chainA, 0, 100)
	if err != nil || len(events) == 0 {
		tb.Fatalf("events of A: %d, %v", len(events), err)
	}
	for i := range events {
		if events[i].Entry.Chain != chainA {
			tb.Fatalf("an event of another chain is readable through A: %+v", events[i].Entry.Chain)
		}
	}
	tail, err := env.Stores.Tail(ctx, chainB)
	if err != nil || tail.Sequence == 0 {
		tb.Fatalf("tail of B: %+v, %v", tail, err)
	}
	wantErr(tb, "a batch against a stale tail", env.Stores.CommitBlock(ctx, chainB, audit.Tail{}, audit.Batch{}), audit.ErrTailMoved)
}

// caseCreateUserWithEntry: an account is created with its password hash and its entry in one
// step; a second account with the same address is refused.
func caseCreateUserWithEntry(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	u := identity.UserRecord{
		ID: fresh[identity.UserID](env), PrincipalID: fresh[identity.PrincipalID](env), EmailNormalized: "operator@example.test",
		DisplayName: "Operator", Status: identity.UserActive, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	if err := env.Stores.CreateUserWithEntry(ctx, u, "argon2id-placeholder", audittest.Instance(tb, "account.created")); err != nil {
		tb.Fatalf("create a user with its entry: %v", err)
	}
	again := u
	again.ID, again.PrincipalID = fresh[identity.UserID](env), fresh[identity.PrincipalID](env)
	wantErr(tb, "a repeated address", env.Stores.CreateUserWithEntry(ctx, again, "argon2id-placeholder", audittest.Instance(tb, "account.created")), identity.ErrConflict)
}

// caseChangePassword: the hash changes and the live sessions of the account end in one step;
// an unknown account is refused.
func caseChangePassword(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.ChangePassword(ctx, env.B.User.ID, "argon2id-changed", seededAt, audittest.Instance(tb, "password.changed")); err != nil {
		tb.Fatalf("change the password of B: %v", err)
	}
	wantErr(tb, "an unknown account", env.Stores.ChangePassword(ctx, fresh[identity.UserID](env), "argon2id-changed", seededAt, audittest.Instance(tb, "password.changed")), identity.ErrNotFound)
}

func containsChain(chains []audit.Chain, want audit.Chain) bool {
	for _, c := range chains {
		if c == want {
			return true
		}
	}
	return false
}
