// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// failingRenewals is a store whose lease renewals can be made to fail, as when the
// worker loses the database (FI-07 from the side of the worker).
type failingRenewals struct {
	durable.Store
	fail atomic.Bool
}

func (f *failingRenewals) RenewLeases(ctx context.Context, w execution.WorkerID, ttl time.Duration, now time.Time) ([]store.Renewed, error) {
	if f.fail.Load() {
		return nil, errors.New("database unreachable")
	}
	return f.Store.RenewLeases(ctx, w, ttl, now) //nolint:wrapcheck // a test double passes the answer through
}

// blockingRunner holds an activation until the context of its lease ends or the
// test lets it go.
type blockingRunner struct {
	started chan struct{}
	release chan struct{}
	err     atomic.Value
}

func (b *blockingRunner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if call.Node.ID == "t1" {
		select {
		case b.started <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			b.err.Store(context.Cause(ctx))
			return execution.NodeOutput{}, ctx.Err()
		case <-b.release:
		}
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
}

// TestLeaseLostEndsTheWork: when a renewal finds the lease taken, the work the
// worker is doing is canceled and dropped, not failed; whoever holds the token now
// finishes the execution.
func TestLeaseLostEndsTheWork(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(2)))
		slow := &blockingRunner{started: make(chan struct{}, 1), release: make(chan struct{})}
		worker := execution.WorkerID(env.IDs.New())
		eng := env.Engine(durable.Config{Worker: worker, Concurrency: 1, RenewEvery: 20 * time.Millisecond, PollEvery: 10 * time.Millisecond, LeaseTTL: 10 * time.Second}, execution.Deps{Runner: slow})
		execID := env.Start(eng, pub, input())
		runCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- eng.Run(runCtx) }()
		defer func() {
			stop()
			if err := <-done; err != nil {
				t.Errorf("Run() = %v", err)
			}
		}()
		select {
		case <-slow.started:
		case <-time.After(10 * time.Second):
			t.Fatal("the activation never started")
		}
		// The lease is taken from the worker while its activation runs.
		if _, err := env.Store.ReleaseWorkerLeases(ctx, worker, 3, time.Now()); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the activation to be canceled", 10*time.Second, func() bool { return slow.err.Load() != nil })
		if cause, _ := slow.err.Load().(error); !errors.Is(cause, durable.ErrLeaseLost) {
			t.Errorf("the activation was canceled with %v, want ErrLeaseLost", cause)
		}
		// Another engine finishes the execution; the first one committed nothing.
		close(slow.release)
		other := env.Engine(durable.Config{Concurrency: 1}, execution.Deps{Runner: slow})
		waitFor(t, "the token to be claimable again", 5*time.Second, func() bool {
			env.Drain(other)
			return statusOf(env, execID) == execution.ExecSucceeded
		})
		if got := countEvents(env, execID, "node.succeeded"); got != 4 {
			t.Errorf("%d node.succeeded events, want 4 (s, t1, t2, e): the work was committed more than once", got)
		}
	})
}

// TestStaleLeasesFenceTheWorker: a worker that cannot renew its leases for longer
// than they last treats them as expired by itself, and commits nothing (local
// self-fencing, ADR-0013 section 4).
func TestStaleLeasesFenceTheWorker(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(2)))
		slow := &blockingRunner{started: make(chan struct{}, 1), release: make(chan struct{})}
		flaky := &failingRenewals{Store: env.Store}
		const lease = 400 * time.Millisecond
		eng := env.EngineOn(flaky, durable.Config{Concurrency: 1, RenewEvery: 30 * time.Millisecond, PollEvery: 10 * time.Millisecond, LeaseTTL: lease}, execution.Deps{Runner: slow})
		execID := env.Start(eng, pub, input())
		started := capture(env, execID)
		flaky.fail.Store(true)
		runCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- eng.Run(runCtx) }()
		defer func() {
			stop()
			if err := <-done; err != nil {
				t.Errorf("Run() = %v", err)
			}
		}()
		select {
		case <-slow.started:
		case <-time.After(10 * time.Second):
			t.Fatal("the activation never started")
		}
		waitFor(t, "the worker to fence itself", 10*time.Second, func() bool { return slow.err.Load() != nil })
		if cause, _ := slow.err.Load().(error); !errors.Is(cause, durable.ErrLeaseStale) {
			t.Errorf("the activation was canceled with %v, want ErrLeaseStale", cause)
		}
		if got := capture(env, execID); got.Revision != started.Revision {
			t.Errorf("a worker that fenced itself committed: revision %d, was %d", got.Revision, started.Revision)
		}
	})
}

// TestEnvironmentsSurviveACrash: the environment an activation was given is
// recorded with the step that committed it, and a worker that takes the token
// after a crash reads the records of the token (ADR-0013 section 7).
func TestEnvironmentsSurviveACrash(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(4)))
		worker := execution.WorkerID(env.IDs.New())
		// One activation per commit, and a crash right after the first commit.
		faults := &faultPlan{only: []durable.Point{durable.PointCommitted}, failAt: 1, err: errInjected}
		cfg := durable.Config{Worker: worker, Concurrency: 1, CheckpointActivations: 2, Faults: faults}
		eng := env.Engine(cfg, runner())
		execID := env.Start(eng, pub, input())
		if failed := runToEnd(t, eng); failed != 1 {
			t.Fatalf("%d steps failed, want 1", failed)
		}
		if got := statusOf(env, execID); got != execution.ExecSucceeded {
			t.Fatalf("execution is %s, want SUCCEEDED", got)
		}
		tokens, err := env.Store.ListTokenNondeterminism(ctx, env.Org, store.TokenID(id.Derive(execID.String(), "root")), 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(tokens) == 0 {
			t.Error("no environment was recorded for the token")
		}
		if diff := cmp.Diff(1, countEvents(env, execID, "execution.succeeded")); diff != "" {
			t.Errorf("execution.succeeded events (-want +got):\n%s", diff)
		}
	})
}

// TestDispatchLoopDeliversTheOutbox: a running engine delivers its outbox on its own.
func TestDispatchLoopDeliversTheOutbox(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(sequence(1)))
		deliverer := &recordingDeliverer{}
		rule := func(_ identity.OrgID, id store.ExecutionID, e execution.Event) (string, []byte, bool) {
			if e.Type != execution.EventExecutionSucceeded {
				return "", nil, false
			}
			return "execution.succeeded", []byte(`{"execution":"` + id.String() + `"}`), true
		}
		eng := env.Engine(durable.Config{Concurrency: 1, Outbox: rule, Deliver: deliverer, DispatchEvery: 10 * time.Millisecond, PollEvery: 10 * time.Millisecond}, runner())
		ctx, stop := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- eng.Run(ctx) }()
		defer func() {
			stop()
			if err := <-done; err != nil {
				t.Errorf("Run() = %v", err)
			}
		}()
		env.Start(eng, pub, input())
		waitFor(t, "the delivery", 10*time.Second, func() bool {
			deliverer.mu.Lock()
			defer deliverer.mu.Unlock()
			return len(deliverer.attempts) == 1
		})
		if entries, _ := eng.Plans().Len(); entries != 1 {
			t.Errorf("%d documents cached, want 1", entries)
		}
	})
}

// gateRunner blocks the node t1 of one execution until the test lets it go.
type gateRunner struct {
	block   atomic.Value // execution.ID
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *gateRunner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if blocked, _ := g.block.Load().(execution.ID); call.Execution == blocked && call.Node.ID == "t1" {
		g.once.Do(func() { close(g.started) })
		select {
		case <-ctx.Done():
			return execution.NodeOutput{}, ctx.Err()
		case <-g.release:
		}
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
}

// TestFailedStepGivesItsLeaseBack (BL-065): a step that fails with an ordinary
// error must not leave its token LEASED. Renewals go by worker, so while the
// worker holds any other lease (here, execution A is blocked in a node) a lease
// that was only dropped locally would be renewed for ever.
func TestFailedStepGivesItsLeaseBack(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(sequence(2)))
		g := &gateRunner{started: make(chan struct{}), release: make(chan struct{})}
		// The first step to reach its commit fails once: A is blocked before that, so it is B's.
		faults := &faultPlan{only: []durable.Point{durable.PointWorked}, failAt: 1, err: errInjected}
		cfg := durable.Config{Concurrency: 2, RenewEvery: 30 * time.Millisecond, PollEvery: 10 * time.Millisecond, ReapEvery: 50 * time.Millisecond, LeaseTTL: time.Minute, Faults: faults}
		eng := env.Engine(cfg, execution.Deps{Runner: g})
		a := env.Start(eng, pub, input())
		g.block.Store(execution.ID(a.String()))
		b := env.Start(eng, pub, input())
		ctx, stop := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- eng.Run(ctx) }()
		defer func() {
			close(g.release)
			stop()
			if err := <-done; err != nil {
				t.Errorf("Run() = %v", err)
			}
		}()
		select {
		case <-g.started:
		case <-time.After(10 * time.Second):
			t.Fatal("the activation of A never started")
		}
		// B finishes while A still holds its lease: the lease of B came back after the failure.
		waitFor(t, "B to finish after its failed step", 20*time.Second, func() bool { return statusOf(env, b) == execution.ExecSucceeded })
		for _, tk := range env.LiveTokens(b) {
			t.Errorf("token %s of B is still %s", tk.ID, tk.Status)
		}
		if got := countEvents(env, b, "lease.expired"); got != 1 {
			t.Errorf("%d lease.expired events for B, want 1: the failure counts as one abandonment", got)
		}
	})
}

// TestCommitsDoNotRenewTheLocalLease (BL-068): a commit that keeps the same token
// does not extend its lease in the database, so it must not extend the local
// validity either. A worker that cannot renew (no renewal loop here) and keeps
// committing steps has to fence itself within the lease, not run on until the
// flow ends.
func TestCommitsDoNotRenewTheLocalLease(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(40)))
		slow := &slowRunner{each: 40 * time.Millisecond}
		const lease = 400 * time.Millisecond
		eng := env.Engine(durable.Config{Concurrency: 1, LeaseTTL: lease, CheckpointActivations: 1}, execution.Deps{Runner: slow})
		execID := env.Start(eng, pub, input())
		claimed, start, err := eng.Claim(ctx, 1)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim = %d, %v", len(claimed), err)
		}
		began := time.Now()
		if err := eng.Work(ctx, claimed[0], start); err != nil {
			t.Fatalf("Work() = %v", err)
		}
		if took := time.Since(began); took > 3*lease {
			t.Errorf("the worker ran %v on a lease of %v with no renewal", took, lease)
		}
		if got := statusOf(env, execID); got == execution.ExecSucceeded {
			t.Error("the flow ended on a lease that was never renewed: commits extended the local validity")
		}
	})
}

// slowRunner takes a fixed time for every node and passes its input on.
type slowRunner struct{ each time.Duration }

func (s *slowRunner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	select {
	case <-ctx.Done():
		return execution.NodeOutput{}, ctx.Err()
	case <-time.After(s.each):
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
}
