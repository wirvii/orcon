// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
)

func docOf(b *executiontest.Builder) *flow.Document {
	d := b.Doc()
	return &d
}

func input() execution.Batch { return executiontest.Batch(`{"n":1}`) }

// rows is what a test compares before and after an attempt that must change
// nothing.
type rows struct {
	Status   string
	Revision int64
	Events   []string
	Tokens   []string
}

func capture(env *storetest.EngineEnv, execID store.ExecutionID) rows {
	env.T.Helper()
	snap, err := env.Store.LoadSnapshot(env.T.Context(), env.Org, execID)
	if err != nil {
		env.T.Fatalf("load snapshot: %v", err)
	}
	r := rows{Status: snap.Status, Revision: snap.Revision}
	events := env.Events(execID)
	for i := range events {
		r.Events = append(r.Events, events[i].Type)
	}
	for i := range snap.Tokens {
		t := &snap.Tokens[i]
		r.Tokens = append(r.Tokens, t.NodeID+"/"+t.Status)
	}
	return r
}

// TestFencing (E-REC-1): a worker whose lease was taken fails with ErrFenced on
// every write, and its commit leaves no change behind; the work is done once, by
// the worker that holds the token now.
func TestFencing(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(3)))
		workerA := execution.WorkerID(env.IDs.New())
		runnerA, runnerB := &executiontest.TraceRunner{}, &executiontest.TraceRunner{}
		faults := &faultPlan{}
		engA := env.Engine(durable.Config{Worker: workerA, Concurrency: 1, Faults: faults}, execution.Deps{Runner: runnerA})
		engB := env.Engine(durable.Config{Concurrency: 1}, execution.Deps{Runner: runnerB})
		execID := env.Start(engA, pub, input())

		var settled rows
		stolen := false
		faults.on = func(ctx context.Context, p durable.Point, _ store.TokenID) error {
			if p != durable.PointWorked || stolen {
				return nil
			}
			stolen = true
			// The lease of A is taken from it, as the reaper would after its expiry, and B finishes the execution.
			released, err := env.Store.ReleaseLostLeases(ctx, workerA, 3, time.Now())
			if err != nil || len(released) != 1 || released[0].AbandonedCount != 1 {
				t.Errorf("the lease of A was not taken: %+v, %v", released, err)
			}
			env.Drain(engB)                //nolint:contextcheck // the helper works with the context of its test
			settled = capture(env, execID) //nolint:contextcheck // same
			return nil
		}
		claimed, start, err := engA.Claim(ctx, 1)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim = %d tokens, %v", len(claimed), err)
		}
		if err := engA.Work(ctx, claimed[0], start); err != nil {
			t.Fatalf("a fenced worker must drop its token without an error: %v", err)
		}
		if !stolen {
			t.Fatal("the lease was never taken")
		}
		if diff := cmp.Diff(settled, capture(env, execID)); diff != "" {
			t.Errorf("the fenced worker changed the execution (-before +after):\n%s", diff)
		}
		if got := statusOf(env, execID); got != execution.ExecSucceeded {
			t.Errorf("execution is %s, want SUCCEEDED", got)
		}
		if got := countEvents(env, execID, "node.succeeded"); got != 5 {
			t.Errorf("%d node.succeeded events, want 5 (s, t1, t2, t3, e): the work was done more than once", got)
		}
		if got := len(runnerB.Calls()); got != 3 {
			t.Errorf("the worker that holds the token ran %d tasks, want 3", got)
		}
	})
}

// TestFencingStoreWrites: every write of the commit refuses a stale fence and
// leaves nothing behind (the same rule at the level of the ports).
func TestFencingStoreWrites(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(1)))
		eng := env.Engine(durable.Config{Concurrency: 1}, runner())
		execID := env.Start(eng, pub, input())
		claimed, _, err := eng.Claim(ctx, 1)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim = %d tokens, %v", len(claimed), err)
		}
		c := claimed[0]
		stale := []store.Fence{
			{OrgID: env.Org, Token: c.TokenID, Epoch: c.LeaseEpoch + 1, Owner: execution.WorkerID(env.IDs.New())},
			{OrgID: env.Org, Token: c.TokenID, Epoch: c.LeaseEpoch + 1, Owner: execution.WorkerID{}},
		}
		before := capture(env, execID)
		for _, f := range stale {
			upd := store.TokenUpdate{OrgID: env.Org, Token: c.TokenID, Status: "RUNNABLE", NodeID: c.NodeID, ScopePath: c.ScopePath, ForkLineage: []byte(`{}`)}
			writes := map[string]func(tx store.Tx) error{
				"assert":  func(tx store.Tx) error { return tx.AssertFence(ctx, f, time.Now()) },
				"advance": func(tx store.Tx) error { return tx.AdvanceToken(ctx, f, upd, time.Now()) },
				"move":    func(tx store.Tx) error { return tx.MoveHeldToken(ctx, f, upd, time.Now()) },
			}
			for name, write := range writes {
				err := env.Store.InTx(ctx, env.Org, func(ctx context.Context, tx store.Tx) error {
					if _, err := tx.AppendEvent(ctx, store.Event{
						ID: store.EventID(env.IDs.New()), OrgID: env.Org, ExecutionID: execID, OccurredAt: time.Now(), Type: "lost",
						SchemaVersion: 1, Reference: []byte(`{}`), CreatedAt: time.Now(),
					}); err != nil {
						return err
					}
					return write(tx)
				})
				if !errors.Is(err, store.ErrFenced) {
					t.Errorf("%s with a stale fence = %v, want ErrFenced", name, err)
				}
			}
		}
		if diff := cmp.Diff(before, capture(env, execID)); diff != "" {
			t.Errorf("writes with a stale fence left changes (-before +after):\n%s", diff)
		}
	})
}

// TestLeaseExpiry (E-REC-1): a token whose worker died returns to RUNNABLE when
// its lease expires, and no token stays LEASED with an expired lease for more
// than twice the TTL.
func TestLeaseExpiry(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		const ttl = 300 * time.Millisecond
		pub := env.Publish(docOf(sequence(2)))
		dead := env.Engine(durable.Config{Concurrency: 1, LeaseTTL: ttl}, runner())
		execID := env.Start(dead, pub, input())
		claimed, _, err := dead.Claim(ctx, 1)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim = %d tokens, %v", len(claimed), err)
		}
		// The worker that claimed the token dies: nothing renews its lease or works the token.
		alive := env.Engine(durable.Config{Concurrency: 2, LeaseTTL: ttl, RenewEvery: ttl / 3, PollEvery: 10 * time.Millisecond, ReapEvery: 20 * time.Millisecond}, runner())
		runCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- alive.Run(runCtx) }()
		t.Cleanup(func() {
			stop()
			if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("Run() = %v", err)
			}
		})
		claimedAt := time.Now()
		for statusOf(env, execID) != execution.ExecSucceeded {
			if time.Since(claimedAt) > 30*time.Second {
				t.Fatalf("execution is %s after %v: the expired lease was never recovered", statusOf(env, execID), time.Since(claimedAt))
			}
			for _, tok := range env.LiveTokens(execID) {
				if tok.Status == "LEASED" && time.Since(tok.LeaseExpiresAt) > ttl {
					t.Fatalf("token %s is LEASED %v after its lease expired, more than the TTL %v allows", tok.ID, time.Since(tok.LeaseExpiresAt), ttl)
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
		if got := countEvents(env, execID, "lease.expired"); got != 1 {
			t.Errorf("%d lease.expired events, want 1", got)
		}
		if got := countEvents(env, execID, "node.succeeded"); got != 4 {
			t.Errorf("%d node.succeeded events, want 4 (s, t1, t2, e)", got)
		}
	})
}

// TestReaper (E-REC-1): the reaper returns expired leases to RUNNABLE, counts
// the abandonment, and suspends a token abandoned three times as poison with its
// events and attention request in the same transaction (ADR-0013 section 9),
// without touching another execution.
func TestReaper(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		const ttl = 30 * time.Millisecond
		pub := env.Publish(docOf(sequence(2)))
		eng := env.Engine(durable.Config{Concurrency: 1, LeaseTTL: ttl}, runner())
		poisoned := env.Start(eng, pub, input())
		for round := 1; round <= 3; round++ {
			claimed, _, err := claimRound(ctx, eng, round, 3)
			if err != nil || len(claimed) != 1 {
				t.Fatalf("round %d: claim = %d tokens, %v", round, len(claimed), err)
			}
			if claimed[0].LeaseEpoch != int64(round) {
				t.Fatalf("round %d: claimed at epoch %d", round, claimed[0].LeaseEpoch)
			}
			time.Sleep(2 * ttl)
			n, err := eng.Reap(ctx)
			if err != nil || n != 1 {
				t.Fatalf("round %d: reaped %d, %v", round, n, err)
			}
			tokens := env.LiveTokens(poisoned)
			if len(tokens) != 1 || tokens[0].AbandonedCount != int64(round) {
				t.Fatalf("round %d: tokens %+v", round, tokens)
			}
			wantStatus := "RUNNABLE"
			if round == 3 {
				wantStatus = "SUSPENDED"
			}
			if tokens[0].Status != wantStatus {
				t.Fatalf("round %d: token is %s, want %s", round, tokens[0].Status, wantStatus)
			}
		}
		tokens := env.LiveTokens(poisoned)
		if tokens[0].SuspendReason != "POISON" {
			t.Errorf("suspend reason %q, want POISON", tokens[0].SuspendReason)
		}
		if got := statusOf(env, poisoned); got != execution.ExecSuspended {
			t.Errorf("execution is %s, want SUSPENDED", got)
		}
		if got := countEvents(env, poisoned, "lease.expired"); got != 3 {
			t.Errorf("%d lease.expired events, want 3", got)
		}
		if got := countEvents(env, poisoned, "token.poisoned"); got != 1 {
			t.Errorf("%d token.poisoned events, want 1", got)
		}
		requests, err := env.Store.ListAttentionRequests(ctx, env.Org, "OPEN", store.Cursor{}, 10)
		if err != nil || len(requests) != 1 || requests[0].Cause != "POISON_TOKEN" || requests[0].ExecutionID != poisoned {
			t.Errorf("attention requests = %+v, %v; want one POISON_TOKEN for the suspended execution", requests, err)
		}
		// Another execution is not affected: it runs to its end.
		// It is worked by an engine with the normal lease: the 30 ms lease above is for the reaper,
		// and a slow machine would let it expire in the middle of an honest step.
		worker := env.Engine(durable.Config{Concurrency: 1}, runner())
		other := env.Start(worker, pub, input())
		env.Drain(worker)
		if got := statusOf(env, other); got != execution.ExecSucceeded {
			t.Errorf("another execution is %s, want SUCCEEDED", got)
		}
		// The reaper has nothing more to do, and the suspended token stays suspended.
		if n, err := eng.Reap(ctx); err != nil || n != 0 {
			t.Errorf("a further pass reaped %d, %v; want nothing", n, err)
		}
	})
}

// TestLostLeasesCountAbandonment (BL-051): the leases a previous run of a worker
// left count as abandoned and reach the poison threshold, like an expired lease.
func TestLostLeasesCountAbandonment(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true, PoisonThreshold: 2}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(1)))
		worker := execution.WorkerID(env.IDs.New())
		eng := env.Engine(durable.Config{Worker: worker, Concurrency: 1, PoisonThreshold: 2}, runner())
		execID := env.Start(eng, pub, input())
		for round := 1; round <= 2; round++ {
			if claimed, _, err := claimRound(ctx, eng, round, 2); err != nil || len(claimed) != 1 {
				t.Fatalf("round %d: claim = %d, %v", round, len(claimed), err)
			}
			released, err := env.Store.ReleaseLostLeases(ctx, worker, 2, time.Now())
			if err != nil || len(released) != 1 || released[0].AbandonedCount != int64(round) || released[0].Poisoned != (round == 2) {
				t.Fatalf("round %d: released = %+v, %v", round, released, err)
			}
		}
		tokens := env.LiveTokens(execID)
		if len(tokens) != 1 || tokens[0].Status != "SUSPENDED" || tokens[0].SuspendReason != "POISON" {
			t.Errorf("tokens = %+v, want one SUSPENDED POISON token", tokens)
		}
	})
}

// claimRound claims the token in the round-th time it is claimed in a scenario
// that abandons it every time: the round that finds it one abandonment short of
// the threshold takes it as the last chance, the others normally.
func claimRound(ctx context.Context, eng *durable.Engine, round int, threshold int) ([]store.Claimed, time.Time, error) {
	if round == threshold {
		return eng.ClaimLastChance(ctx)
	}
	return eng.Claim(ctx, 1)
}
