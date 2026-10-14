// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
)

// holdRunner holds the node t1 of the executions it is told to, until the test
// lets them go or the context of the activation ends, and tells when one starts.
type holdRunner struct {
	mu      sync.Mutex
	held    map[execution.ID]chan struct{}
	started map[execution.ID]chan struct{}
}

func newHoldRunner() *holdRunner {
	return &holdRunner{held: map[execution.ID]chan struct{}{}, started: map[execution.ID]chan struct{}{}}
}

// hold makes the node t1 of the execution wait for the channel it returns to close.
func (h *holdRunner) hold(id store.ExecutionID) (release func(), started <-chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := execution.ID(id.String())
	gate, begun := make(chan struct{}), make(chan struct{})
	h.held[key], h.started[key] = gate, begun
	var once sync.Once
	return func() { once.Do(func() { close(gate) }) }, begun
}

func (h *holdRunner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if call.Node.ID == "t1" {
		h.mu.Lock()
		gate, begun := h.held[call.Execution], h.started[call.Execution]
		h.mu.Unlock()
		if gate != nil {
			select {
			case <-begun:
			default:
				close(begun)
			}
			select {
			case <-ctx.Done():
				return execution.NodeOutput{}, ctx.Err()
			case <-gate:
			}
		}
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
}

// abandonedOnce starts an execution and leaves its token one abandonment short of
// the threshold, as two lost leases of an earlier worker would.
func abandonedTo(t *testing.T, env *storetest.EngineEnv, eng *durable.Engine, pub *storetest.Published, threshold int) store.ExecutionID {
	t.Helper()
	ctx := t.Context()
	execID := env.Start(eng, pub, input())
	for round := 1; round < threshold; round++ {
		w := execution.WorkerID(env.IDs.New())
		claimed, err := env.Store.ClaimTokens(ctx, store.ClaimRequest{Worker: w, Limit: 10, LeaseTTL: time.Hour, Now: time.Now(), PoisonThreshold: int64(threshold)})
		if err != nil || len(claimed) != 1 {
			t.Fatalf("abandon round %d: claim = %d, %v", round, len(claimed), err)
		}
		if _, err := env.Store.ReleaseLostLeases(ctx, w, int64(threshold), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	return execID
}

// parked starts an execution whose token has been abandoned twice and leaves it
// leased to a worker that is not the engine under test, so that it is not claimable
// until the returned function gives it back (without counting): the test then
// decides when the last chance token appears in front of a busy engine.
func parked(t *testing.T, env *storetest.EngineEnv, eng *durable.Engine, pub *storetest.Published) (execID store.ExecutionID, appear func()) {
	t.Helper()
	ctx := t.Context()
	execID = abandonedTo(t, env, eng, pub, 3)
	holder := execution.WorkerID(env.IDs.New())
	claimed, err := env.Store.ClaimLastChanceToken(ctx, store.ClaimRequest{Worker: holder, Limit: 1, LeaseTTL: time.Hour, Now: time.Now(), PoisonThreshold: 3})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("park the token: claim = %d, %v", len(claimed), err)
	}
	return execID, func() {
		if _, err := env.Store.ReleaseWorkerLeases(ctx, holder, 3, time.Now()); err != nil {
			t.Error(err)
		}
	}
}

func tokenOf(t *testing.T, env *storetest.EngineEnv, execID store.ExecutionID) store.Token {
	t.Helper()
	tokens := env.LiveTokens(execID)
	if len(tokens) != 1 {
		t.Fatalf("%d live tokens, want 1: %+v", len(tokens), tokens)
	}
	return tokens[0]
}

func run(eng *durable.Engine) (stop func() error) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- eng.Run(ctx) }()
	return func() error {
		cancel()
		return <-done
	}
}

// TestAbandonCount (E-REC-5, the worker's part; the stores' part is in
// internal/adapter/storetest): who adds an abandonment and who runs alone.
func TestAbandonCount(t *testing.T) {
	t.Run("a threshold below two is rejected", func(t *testing.T) {
		_, err := durable.New(&durable.Config{Worker: execution.WorkerID{1}, PoisonThreshold: 1}, nil, nil, nil, execution.Deps{}, durable.PlanLimits{})
		if err == nil {
			t.Fatal("a poison threshold of 1 was accepted")
		}
	})
	cases := []struct {
		name string
		run  func(*testing.T, *storetest.EngineEnv)
	}{
		{"shutdown within the grace adds nothing", returnedWithinGrace},
		{"shutdown past the grace adds nothing", returnedPastGrace},
		{"progress puts the count back to zero", progressResets},
		{"a last chance token runs only on an empty worker, which then claims nothing", lastChanceRunsAlone},
		{"a last chance token that waits too long empties the busy workers", lastChanceStarvation},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
				t.Helper()
				c.run(t, env)
			})
		})
	}
}

// returnedWithinGrace: a worker that shuts down in order finishes the activation
// in hand, commits and gives the token back; three shutdowns in a row leave the
// count at zero.
func returnedWithinGrace(t *testing.T, env *storetest.EngineEnv) {
	t.Helper()
	pub := env.Publish(docOf(sequence(30)))
	slow := &slowRunner{each: 30 * time.Millisecond}
	first := env.Engine(durable.Config{Concurrency: 1, PollEvery: 5 * time.Millisecond, ShutdownGrace: 10 * time.Second}, execution.Deps{Runner: slow})
	execID := env.Start(first, pub, input())
	for shutdown := 1; shutdown <= 3; shutdown++ {
		eng := env.Engine(durable.Config{Concurrency: 1, PollEvery: 5 * time.Millisecond, ShutdownGrace: 10 * time.Second}, execution.Deps{Runner: slow})
		stop := run(eng)
		waitFor(t, "the token to be leased", 10*time.Second, func() bool { return tokenOf(t, env, execID).Status == "LEASED" })
		time.Sleep(100 * time.Millisecond)
		if err := stop(); err != nil {
			t.Fatalf("shutdown %d: Run() = %v", shutdown, err)
		}
		tok := tokenOf(t, env, execID)
		if tok.AbandonedCount != 0 || tok.Status != "RUNNABLE" {
			t.Fatalf("shutdown %d: token %s with %d abandonments, want RUNNABLE with none", shutdown, tok.Status, tok.AbandonedCount)
		}
	}
	if n := countEvents(env, execID, "lease.expired"); n != 0 {
		t.Errorf("%d lease.expired events for leases the worker gave back", n)
	}
}

// returnedPastGrace: a node that outlasts the grace period is canceled and the
// token goes back without a count.
func returnedPastGrace(t *testing.T, env *storetest.EngineEnv) {
	t.Helper()
	pub := env.Publish(docOf(sequence(2)))
	hold := newHoldRunner()
	eng := env.Engine(durable.Config{Concurrency: 1, PollEvery: 5 * time.Millisecond, ShutdownGrace: 100 * time.Millisecond}, execution.Deps{Runner: hold})
	execID := env.Start(eng, pub, input())
	_, began := hold.hold(execID)
	stop := run(eng)
	select {
	case <-began:
	case <-time.After(10 * time.Second):
		t.Fatal("the node never started")
	}
	if err := stop(); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	tok := tokenOf(t, env, execID)
	if tok.AbandonedCount != 0 || tok.Status != "RUNNABLE" {
		t.Fatalf("token %s with %d abandonments, want RUNNABLE with none", tok.Status, tok.AbandonedCount)
	}
	if got := statusOf(env, execID); got == execution.ExecFailed {
		t.Error("the execution failed because the worker shut down")
	}
}

// progressResets: a token abandoned once finishes a step with a result and its
// count goes back to zero in that commit.
func progressResets(t *testing.T, env *storetest.EngineEnv) {
	t.Helper()
	pub := env.Publish(docOf(sequence(3)))
	hold := newHoldRunner()
	eng := env.Engine(durable.Config{Concurrency: 1, PollEvery: 5 * time.Millisecond, CheckpointActivations: 1}, execution.Deps{Runner: hold})
	execID := abandonedTo(t, env, eng, pub, 2)
	if got := tokenOf(t, env, execID).AbandonedCount; got != 1 {
		t.Fatalf("the token was abandoned %d times, want 1", got)
	}
	release, _ := hold.hold(execID)
	stop := run(eng)
	// The node t1 is held on purpose; it is let go before the engine stops, or the stop would wait out its grace.
	defer func() {
		release()
		_ = stop()
	}()
	// Its first step ends with the start node done: a result, so the count is cleared while t1 is held.
	waitFor(t, "the first result to clear the count", 10*time.Second, func() bool {
		tokens := env.LiveTokens(execID)
		return len(tokens) == 1 && tokens[0].AbandonedCount == 0
	})
}

// lastChanceRunsAlone: a token one abandonment short of the threshold is not
// claimed while the worker has other tokens, and the worker that takes it claims
// nothing else until it is done.
func lastChanceRunsAlone(t *testing.T, env *storetest.EngineEnv) {
	t.Helper()
	pub := env.Publish(docOf(sequence(1)))
	hold := newHoldRunner()
	eng := env.Engine(durable.Config{Concurrency: 4, PollEvery: 5 * time.Millisecond, IsolationWait: time.Hour}, execution.Deps{Runner: hold})
	last, appear := parked(t, env, eng, pub)
	healthy := []store.ExecutionID{env.Start(eng, pub, input()), env.Start(eng, pub, input())}
	var releases []func()
	var begun []<-chan struct{}
	for _, h := range healthy {
		r, b := hold.hold(h)
		releases, begun = append(releases, r), append(begun, b)
	}
	releaseLast, lastBegan := hold.hold(last)
	defer releaseLast()
	stop := run(eng)
	defer func() { _ = stop() }()
	for _, b := range begun {
		select {
		case <-b:
		case <-time.After(10 * time.Second):
			t.Fatal("a healthy node never started")
		}
	}
	// The process is busy when the token appears: it waits, whatever the idle goroutines do.
	appear()
	time.Sleep(300 * time.Millisecond)
	if tok := tokenOf(t, env, last); tok.Status != "RUNNABLE" {
		t.Fatalf("the last chance token was claimed while the worker had other tokens: %+v", tok)
	}
	for _, r := range releases {
		r()
	}
	select {
	case <-lastBegan:
	case <-time.After(10 * time.Second):
		t.Fatal("the last chance token never ran once the worker was empty")
	}
	// It runs alone: an execution that arrives meanwhile is not claimed.
	late := env.Start(eng, pub, input())
	time.Sleep(300 * time.Millisecond)
	if got := statusOf(env, late); got != execution.ExecQueued {
		t.Fatalf("an execution was claimed (%s) while the worker held a last chance token", got)
	}
	releaseLast()
	waitFor(t, "every execution to end", 20*time.Second, func() bool {
		for _, x := range append(healthy, last, late) {
			if statusOf(env, x) != execution.ExecSucceeded {
				return false
			}
		}
		return true
	})
}

// lastChanceStarvation: a last chance token that has waited longer than the
// isolation wait makes the busy worker claim nothing new until it is empty, so
// that it can take the token.
func lastChanceStarvation(t *testing.T, env *storetest.EngineEnv) {
	t.Helper()
	pub := env.Publish(docOf(sequence(1)))
	hold := newHoldRunner()
	eng := env.Engine(durable.Config{Concurrency: 4, PollEvery: 5 * time.Millisecond, IsolationWait: 200 * time.Millisecond}, execution.Deps{Runner: hold})
	last, appear := parked(t, env, eng, pub)
	busy := env.Start(eng, pub, input())
	releaseBusy, busyBegan := hold.hold(busy)
	defer releaseBusy()
	releaseLast, lastBegan := hold.hold(last)
	defer releaseLast()
	stop := run(eng)
	defer func() { _ = stop() }()
	select {
	case <-busyBegan:
	case <-time.After(10 * time.Second):
		t.Fatal("the busy node never started")
	}
	appear()
	time.Sleep(600 * time.Millisecond) // three times the isolation wait
	late := env.Start(eng, pub, input())
	time.Sleep(300 * time.Millisecond)
	if got := statusOf(env, late); got != execution.ExecQueued {
		t.Fatalf("a new execution was claimed (%s) though a last chance token had waited past the isolation wait", got)
	}
	releaseBusy()
	select {
	case <-lastBegan:
	case <-time.After(10 * time.Second):
		t.Fatal("the last chance token never ran once the worker emptied")
	}
	if got := statusOf(env, late); got != execution.ExecQueued {
		t.Errorf("the late execution was claimed (%s) before the last chance token finished", got)
	}
	releaseLast()
	waitFor(t, "every execution to end", 20*time.Second, func() bool {
		for _, x := range []store.ExecutionID{busy, last, late} {
			if statusOf(env, x) != execution.ExecSucceeded {
				return false
			}
		}
		return true
	})
}
