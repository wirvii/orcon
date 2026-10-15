// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// TestGracefulShutdown (ADR-0013 section 9): on shutdown the worker stops
// claiming, checkpoints and releases the tokens it holds, and exits; what it did
// not finish is finished by the next process, once.
func TestGracefulShutdown(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(forkJoin(8)))
		slow := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){}}
		var started atomic.Int64
		for i := range 8 {
			slow.Behaviors[flow.NodeID(fmt.Sprintf("b%d", i))] = func(call execution.NodeCall) (execution.NodeOutput, error) {
				started.Add(1)
				time.Sleep(15 * time.Millisecond)
				return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
			}
		}
		first := env.Engine(durable.Config{Concurrency: 4, PollEvery: 5 * time.Millisecond, ShutdownGrace: 10 * time.Second}, execution.Deps{Runner: slow})
		execs := make([]store.ExecutionID, 6)
		for i := range execs {
			execs[i] = env.Start(first, pub, input())
		}
		ctx, stop := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- first.Run(ctx) }()
		waitFor(t, "work to start", 10*time.Second, func() bool { return started.Load() >= 6 })
		stop()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Run() = %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("Run did not return within the grace period")
		}
		// Nothing is left leased to the engine that shut down.
		for _, id := range execs {
			for _, tok := range env.LiveTokens(id) {
				if tok.Status == "LEASED" {
					t.Errorf("token %s of %s is still LEASED after the shutdown", tok.ID, id)
				}
			}
		}
		// The next process finishes everything, each node once.
		second := env.Engine(durable.Config{Concurrency: 4}, runner())
		env.Drain(second)
		for _, id := range execs {
			if got := statusOf(env, id); got != execution.ExecSucceeded {
				t.Errorf("execution %s is %s, want SUCCEEDED", id, got)
			}
			if got := countEvents(env, id, "node.succeeded"); got != 13 {
				t.Errorf("execution %s: %d node.succeeded events, want 13 (s, p, 8 branches, j, after, e)", id, got)
			}
			if got := countEvents(env, id, "join.fired"); got != 1 {
				t.Errorf("execution %s: %d join.fired events, want 1", id, got)
			}
		}
	})
}

// TestPollClaim: with the in-process wake-up off, every claim is a poll, and work
// is claimed within the poll period (C4a polls; the immediate notice is C4b).
func TestPollClaim(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(sequence(2)))
		eng := env.Engine(durable.Config{Concurrency: 2, DisableWake: true, PollEvery: 100 * time.Millisecond}, runner())
		ctx, stop := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- eng.Run(ctx) }()
		defer func() {
			stop()
			if err := <-done; err != nil {
				t.Errorf("Run() = %v", err)
			}
		}()
		start := time.Now()
		execID := env.Start(eng, pub, input())
		waitFor(t, "the execution to end by polling", 5*time.Second, func() bool { return statusOf(env, execID) == execution.ExecSucceeded })
		if took := time.Since(start); took > 2*time.Second {
			t.Errorf("the execution took %v by polling every 100 ms, want at most 2 s", took)
		}
	})
}

// recordingDeliverer records the deliveries and fails the first attempt of each row.
type recordingDeliverer struct {
	mu       sync.Mutex
	attempts map[store.OutboxID][]int64
	failures int
}

func (d *recordingDeliverer) Deliver(_ context.Context, dl durable.Delivery) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.attempts == nil {
		d.attempts = make(map[store.OutboxID][]int64)
	}
	d.attempts[dl.ID] = append(d.attempts[dl.ID], dl.Attempt)
	if d.failures > 0 {
		d.failures--
		return errors.New("receiver unavailable")
	}
	return nil
}

// TestOutbox (ADR-0013 section 8): a delivery that leaves the database is a row
// inserted in the transaction of the state change; a leased dispatcher delivers
// it at least once with a stable identity, retries after a failure, and marks it
// delivered once.
func TestOutbox(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(1)))
		deliverer := &recordingDeliverer{failures: 1}
		rule := func(org identity.OrgID, id store.ExecutionID, e execution.Event) (string, []byte, bool) {
			if e.Type != execution.EventExecutionSucceeded {
				return "", nil, false
			}
			return "execution.succeeded", []byte(`{"execution":"` + id.String() + `"}`), true
		}
		const lease = 100 * time.Millisecond
		eng := env.Engine(durable.Config{Concurrency: 1, Outbox: rule, Deliver: deliverer, LeaseTTL: lease}, runner())
		execID := env.Start(eng, pub, input())
		env.Drain(eng)
		if got := statusOf(env, execID); got != execution.ExecSucceeded {
			t.Fatalf("execution is %s", got)
		}
		// The first delivery fails; the row stays leased until its lease expires, then it is delivered again.
		if n, err := eng.Dispatch(ctx, 10); err != nil || n != 0 {
			t.Fatalf("first dispatch delivered %d, %v; want a failed delivery", n, err)
		}
		if n, err := eng.Dispatch(ctx, 10); err != nil || n != 0 {
			t.Fatalf("a row whose lease is held was dispatched again at once: %d, %v", n, err)
		}
		time.Sleep(2 * lease)
		if n, err := eng.Dispatch(ctx, 10); err != nil || n != 1 {
			t.Fatalf("retry dispatch delivered %d, %v; want 1", n, err)
		}
		if n, err := eng.Dispatch(ctx, 10); err != nil || n != 0 {
			t.Fatalf("a delivered row was dispatched again: %d, %v", n, err)
		}
		deliverer.mu.Lock()
		defer deliverer.mu.Unlock()
		if len(deliverer.attempts) != 1 {
			t.Fatalf("%d distinct rows delivered, want 1", len(deliverer.attempts))
		}
		for id, attempts := range deliverer.attempts {
			if diff := cmp.Diff([]int64{1, 2}, attempts); diff != "" {
				t.Errorf("attempts of %s (-want +got):\n%s", id, diff)
			}
		}
	})
}

// TestTimerWakesItsToken (BL-052): firing a timer and waking the token that waits
// for it are one commit, so a FIRED timer never leaves a WAITING token behind.
func TestTimerWakesItsToken(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(1)))
		eng := env.Engine(durable.Config{Concurrency: 1}, runner())
		execID := env.Start(eng, pub, input())
		claimed, _, err := eng.Claim(ctx, 1)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim = %d, %v", len(claimed), err)
		}
		c := claimed[0]
		// The token waits for a timer that is already due. The wait nodes are cut C8; the rows are the ones C8 will write.
		fence := store.Fence{OrgID: env.Org, Token: c.TokenID, Epoch: c.LeaseEpoch, Owner: eng.Worker()}
		err = env.Store.InTx(ctx, storetest.ScopeOf(env.Org), func(ctx context.Context, tx store.Tx) error {
			now := time.Now()
			timer := store.Timer{
				ID: store.TimerID(env.IDs.New()), OrgID: env.Org, ExecutionID: execID, TokenID: c.TokenID, Kind: "WAIT",
				DueAt: now.Add(-time.Second), CreatedAt: now,
			}
			if err := tx.InsertTimer(ctx, timer); err != nil {
				return err
			}
			if err := tx.MoveToken(ctx, fence, "WAITING", now); err != nil {
				return err
			}
			return tx.RefreshCounters(ctx, env.Org, execID, now)
		})
		if err != nil {
			t.Fatal(err)
		}
		tokens := env.LiveTokens(execID)
		if len(tokens) != 1 || tokens[0].Status != "WAITING" {
			t.Fatalf("tokens = %+v, want one WAITING", tokens)
		}
		fired, err := eng.Reap(ctx)
		if err != nil || fired != 1 {
			t.Fatalf("reap = %d, %v; want the timer fired", fired, err)
		}
		tokens = env.LiveTokens(execID)
		if len(tokens) != 1 || tokens[0].Status != "RUNNABLE" {
			t.Fatalf("after the timer fired the token is %+v, want RUNNABLE", tokens)
		}
		if got := countEvents(env, execID, "timer.fired"); got != 1 {
			t.Errorf("%d timer.fired events, want 1", got)
		}
		env.Drain(eng)
		if got := statusOf(env, execID); got != execution.ExecSucceeded {
			t.Errorf("execution is %s after the wake-up, want SUCCEEDED", got)
		}
		// Firing again changes nothing.
		if n, err := eng.Reap(ctx); err != nil || n != 0 {
			t.Errorf("a second reap fired %d, %v", n, err)
		}
	})
}

// TestClaimIsExclusive: several workers polling one store never work a token twice.
func TestClaimIsExclusive(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(forkJoin(8)))
		var ran sync.Map
		var dup atomic.Int64
		mk := func() *durable.Engine {
			r := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){}}
			for i := range 8 {
				id := flow.NodeID(fmt.Sprintf("b%d", i))
				r.Behaviors[id] = func(call execution.NodeCall) (execution.NodeOutput, error) {
					if _, loaded := ran.LoadOrStore(string(call.NodeExecution), true); loaded {
						dup.Add(1)
					}
					return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
				}
			}
			return env.Engine(durable.Config{Concurrency: 3, PollEvery: 5 * time.Millisecond}, execution.Deps{Runner: r})
		}
		engines := []*durable.Engine{mk(), mk(), mk()}
		var execs []store.ExecutionID
		for range 12 {
			execs = append(execs, env.Start(engines[0], pub, input()))
		}
		ctx, stop := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		for _, eng := range engines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := eng.Run(ctx); err != nil {
					t.Errorf("Run() = %v", err)
				}
			}()
		}
		defer func() { stop(); wg.Wait() }()
		for _, id := range execs {
			waitFor(t, "execution "+id.String(), 30*time.Second, func() bool { return statusOf(env, id) == execution.ExecSucceeded })
		}
		if got := dup.Load(); got != 0 {
			t.Errorf("%d node executions were activated more than once", got)
		}
		var states []string
		for _, id := range execs {
			states = append(states, string(statusOf(env, id)))
		}
		if want := slices.Repeat([]string{"SUCCEEDED"}, len(execs)); !slices.Equal(want, states) {
			t.Errorf("states = %v", states)
		}
	})
}
