// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"context"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
)

// AbandonCount runs the contract of the abandon count (SPEC-001 Enmienda 12
// section 12-1) on a store: when the count grows, when it goes back to zero and
// which tokens a normal claim leaves for the worker that runs them alone. The
// parts that need a worker (shutdown within and past the grace, a worker that
// empties itself, the wait before it does) are tested with the engine.
func AbandonCount(t *testing.T, newStore Factory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(*testing.T, Factory)
	}{
		{"ReaperAddsOne", abandonReaperCase},
		{"LostLeasesOfAPreviousRunAddOne", abandonLostCase},
		{"ReturnByTheOwnerAddsNothing", abandonReturnCase},
		{"ProgressResetsTheCount", abandonProgressCase},
		{"LastChanceTokenIsClaimedApart", abandonLastChanceCase},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { c.run(t, newStore) })
	}
}

// leased inserts a runnable token and leases it to a new worker with a lease that
// has expired (ttl < 0) or that lasts an hour.
func (e *execFixture) leased(ex *store.Execution, ttl time.Duration) (store.Token, store.Fence) {
	e.t.Helper()
	tok := e.token(ex, "a", 0, realNow().Add(-time.Hour))
	w := worker(e)
	claimed, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{
		Worker: w, FairnessKeys: []string{"flow/" + ex.FlowID.String()}, Limit: 100, LeaseTTL: ttl, Now: realNow(),
	})
	if err != nil {
		e.t.Fatalf("claim: %v", err)
	}
	for i := range claimed {
		if claimed[i].TokenID == tok.ID {
			return tok, store.Fence{OrgID: e.orgA, Token: tok.ID, Epoch: claimed[i].LeaseEpoch, Owner: w}
		}
	}
	e.t.Fatalf("the token was not claimed: %+v", claimed)
	return tok, store.Fence{}
}

func (e *execFixture) eventCount(ex store.ExecutionID, typ string) int {
	e.t.Helper()
	events, err := e.s.ListExecutionEvents(e.ctx(), e.orgA, ex, 0, 100)
	if err != nil {
		e.t.Fatal(err)
	}
	n := 0
	for i := range events {
		if events[i].Type == typ {
			n++
		}
	}
	return n
}

func abandonReaperCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok, _ := e.leased(ex, -time.Second)
	reaped, err := e.s.ReleaseExpiredLeases(e.ctx(), 10, 3, realNow())
	if err != nil || len(reaped) != 1 || reaped[0].AbandonedCount != 1 {
		t.Fatalf("reaped = %+v, %v; want one abandonment", reaped, err)
	}
	if got := e.mustToken(e.orgA, tok.ID); got.AbandonedCount != 1 || got.Status != "RUNNABLE" {
		t.Fatalf("token = %+v", got)
	}
	if n := e.eventCount(ex.ID, "lease.expired"); n != 1 {
		t.Errorf("%d lease.expired events, want 1", n)
	}
}

func abandonLostCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok, f := e.leased(ex, time.Hour)
	lost, err := e.s.ReleaseLostLeases(e.ctx(), f.Owner, 3, realNow())
	if err != nil || len(lost) != 1 || lost[0].AbandonedCount != 1 {
		t.Fatalf("lost = %+v, %v; want one abandonment", lost, err)
	}
	if got := e.mustToken(e.orgA, tok.ID); got.AbandonedCount != 1 || got.Status != "RUNNABLE" {
		t.Fatalf("token = %+v", got)
	}
}

func abandonReturnCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok, f := e.leased(ex, time.Hour)
	returned, err := e.s.ReleaseWorkerLeases(e.ctx(), f.Owner, 3, realNow())
	if err != nil || len(returned) != 1 || returned[0].AbandonedCount != 0 || returned[0].Poisoned {
		t.Fatalf("returned = %+v, %v; want no abandonment", returned, err)
	}
	got := e.mustToken(e.orgA, tok.ID)
	if got.AbandonedCount != 0 || got.Status != "RUNNABLE" || !got.LeaseOwner.IsZero() {
		t.Fatalf("token = %+v", got)
	}
	if n := e.eventCount(ex.ID, "lease.expired"); n != 0 {
		t.Errorf("%d lease.expired events for a lease its owner returned, want none", n)
	}
	// Returned as often as the threshold, a token is still not poison.
	for range 5 {
		claimed, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: f.Owner, Limit: 10, LeaseTTL: time.Hour, Now: realNow()})
		if err != nil || len(claimed) == 0 {
			t.Fatalf("claim after a return = %d, %v", len(claimed), err)
		}
		if _, err := e.s.ReleaseWorkerLeases(e.ctx(), f.Owner, 3, realNow()); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.mustToken(e.orgA, tok.ID); got.AbandonedCount != 0 || got.Status != "RUNNABLE" {
		t.Fatalf("token after many returns = %+v", got)
	}
}

func abandonProgressCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	// Two tokens abandoned once each, then leased again.
	var fences [2]store.Fence
	var toks [2]store.Token
	for i := range toks {
		toks[i] = e.token(ex, "n", 0, realNow().Add(-time.Hour))
	}
	claimAll := func(w execution.WorkerID) []store.Claimed {
		t.Helper()
		claimed, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, FairnessKeys: []string{"flow/" + ex.FlowID.String()}, Limit: 100, LeaseTTL: time.Hour, Now: realNow()})
		if err != nil || len(claimed) != len(toks) {
			t.Fatalf("claim = %d tokens, %v; want %d", len(claimed), err, len(toks))
		}
		return claimed
	}
	first := worker(e)
	claimAll(first)
	if _, err := e.s.ReleaseLostLeases(e.ctx(), first, 3, realNow()); err != nil {
		t.Fatal(err)
	}
	w := worker(e)
	claimed := claimAll(w)
	for i := range toks {
		for j := range claimed {
			if claimed[j].TokenID == toks[i].ID {
				fences[i] = store.Fence{OrgID: e.orgA, Token: toks[i].ID, Epoch: claimed[j].LeaseEpoch, Owner: w}
			}
		}
	}
	update := func(reset bool) store.TokenUpdate {
		return store.TokenUpdate{NodeID: "next", ScopePath: "", ForkLineage: []byte(`[]`), ResetAbandoned: reset}
	}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		// A step with no new result moves the token and leaves the count; a step with a result clears it.
		if err := tx.AdvanceToken(ctx, fences[0], update(false), realNow()); err != nil {
			return err
		}
		return tx.AdvanceToken(ctx, fences[1], update(true), realNow())
	}), "advance the tokens")
	if got := e.mustToken(e.orgA, toks[0].ID); got.AbandonedCount != 1 {
		t.Errorf("a step without new results left the count at %d, want 1", got.AbandonedCount)
	}
	if got := e.mustToken(e.orgA, toks[1].ID); got.AbandonedCount != 0 {
		t.Errorf("a step with a new result left the count at %d, want 0", got.AbandonedCount)
	}
	// Moving the held token out of the worker clears it as well.
	upd := update(true)
	upd.Status = "RUNNABLE"
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.MoveHeldToken(ctx, fences[0], upd, realNow()) }), "move the token")
	if got := e.mustToken(e.orgA, toks[0].ID); got.AbandonedCount != 0 {
		t.Errorf("a move with a new result left the count at %d, want 0", got.AbandonedCount)
	}
}

func abandonLastChanceCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	const threshold = 2
	tok, f := e.leased(ex, time.Hour)
	if _, err := e.s.ReleaseLostLeases(e.ctx(), f.Owner, threshold, realNow()); err != nil {
		t.Fatal(err)
	}
	healthy := e.token(ex, "b", 0, realNow().Add(-time.Hour))
	w := worker(e)
	request := store.ClaimRequest{Worker: w, FairnessKeys: []string{"flow/" + ex.FlowID.String()}, Limit: 10, LeaseTTL: time.Hour, Now: realNow(), PoisonThreshold: threshold}
	if since, found, err := e.s.LastChanceWaiting(e.ctx(), threshold); err != nil || !found || since.IsZero() {
		t.Fatalf("LastChanceWaiting = %v, %v, %v; want the token that was abandoned once", since, found, err)
	}
	// A normal claim leaves the token one abandonment short of the threshold.
	claimed, err := e.s.ClaimTokens(e.ctx(), request)
	if err != nil || len(claimed) != 1 || claimed[0].TokenID != healthy.ID {
		t.Fatalf("claim = %+v, %v; want only the healthy token", claimed, err)
	}
	// The last chance claim takes it, and only it, one at a time.
	last, err := e.s.ClaimLastChanceToken(e.ctx(), request)
	if err != nil || len(last) != 1 || last[0].TokenID != tok.ID || last[0].LeaseEpoch != 2 {
		t.Fatalf("last chance claim = %+v, %v; want the abandoned token at epoch 2", last, err)
	}
	if again, err := e.s.ClaimLastChanceToken(e.ctx(), request); err != nil || len(again) != 0 {
		t.Fatalf("a second last chance claim took %d tokens, %v", len(again), err)
	}
	if _, found, err := e.s.LastChanceWaiting(e.ctx(), threshold); err != nil || found {
		t.Fatalf("LastChanceWaiting after the claim = %v, %v; want nothing waiting", found, err)
	}
	// Abandoned once more, alone, the token reaches the threshold.
	lost, err := e.s.ReleaseLostLeases(e.ctx(), w, threshold, realNow())
	if err != nil {
		t.Fatal(err)
	}
	var poisoned bool
	for i := range lost {
		if lost[i].TokenID == tok.ID {
			poisoned = lost[i].Poisoned
		}
	}
	if !poisoned {
		t.Fatalf("released = %+v; want the token poisoned at its second abandonment", lost)
	}
}

// LeftLeased names a token a store was closed with while it was leased.
type LeftLeased struct {
	Org   identity.OrgID
	Token store.TokenID
}

// SeedLeftLeased inserts a token leased to a worker with a lease that has not
// expired, as a process leaves it when it dies.
func SeedLeftLeased(t *testing.T, s Store) LeftLeased {
	t.Helper()
	e := newExecFixture(t, func(*testing.T) Store { return s })
	tok, _ := e.leased(e.execution(), time.Hour)
	return LeftLeased{Org: e.orgA, Token: tok.ID}
}

// CheckStartupRelease verifies, on the store reopened after SeedLeftLeased, that
// opening it released the lease and counted one abandonment: nobody returned it.
func CheckStartupRelease(t *testing.T, s Store, l LeftLeased) {
	t.Helper()
	got, err := s.GetToken(t.Context(), l.Org, l.Token)
	if err != nil {
		t.Fatalf("get token: %v", err)
	}
	if got.Status != "RUNNABLE" || !got.LeaseOwner.IsZero() || got.AbandonedCount != 1 {
		t.Fatalf("token after the reopen = %+v; want RUNNABLE, released, abandoned once", got)
	}
}
