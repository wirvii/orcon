// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/store"
)

var errInjected = errors.New("injected failure")

// runToEnd claims and works until nothing is left, tolerating the errors a fault
// plan injects: a worker that fails gives the lease of its token back by itself.
func runToEnd(t *testing.T, eng *durable.Engine) (failed int) {
	t.Helper()
	for {
		claimed, start, err := eng.Claim(t.Context(), 1)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(claimed) == 0 {
			return failed
		}
		if err := eng.Work(t.Context(), claimed[0], start); err != nil {
			if !errors.Is(err, errInjected) {
				t.Fatalf("work: %v", err)
			}
			failed++ // the engine gave the lease of the failed token back by itself
		}
	}
}

// TestAtomicCheckpoint (E-REC-2): the result of a step, the advance of the token,
// the new tokens and the events commit in one transaction. An error injected
// before the commit leaves nothing partial, and the retry produces the rows once.
func TestAtomicCheckpoint(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(sequence(3)))
		for _, point := range []durable.Point{durable.PointInCommit, durable.PointWorked} {
			t.Run(string(point), func(t *testing.T) {
				worker := execution.WorkerID(env.IDs.New())
				faults := &faultPlan{only: []durable.Point{point}, failAt: 1, err: errInjected}
				eng := env.Engine(durable.Config{Worker: worker, Concurrency: 1, Faults: faults}, runner())
				execID := env.Start(eng, pub, input())
				before := capture(env, execID)
				claimed, start, err := eng.Claim(t.Context(), 1)
				if err != nil || len(claimed) != 1 {
					t.Fatalf("claim = %d, %v", len(claimed), err)
				}
				if err := eng.Work(t.Context(), claimed[0], start); !errors.Is(err, errInjected) {
					t.Fatalf("Work() = %v, want the injected error", err)
				}
				after := capture(env, execID)
				// Only the claim may have changed: the token is leased now, and the execution started.
				if before.Revision != after.Revision {
					t.Errorf("engine state moved from revision %d to %d though the commit failed", before.Revision, after.Revision)
				}
				if got := countEvents(env, execID, "node.started"); got != 0 {
					t.Errorf("%d node.started events survived a failed commit", got)
				}
				if got := countEvents(env, execID, "execution.succeeded"); got != 0 {
					t.Error("the failed commit ended the execution")
				}
				// The engine gave the lease back when the step failed: the retry commits the whole step once.
				env.Drain(eng)
				if got := statusOf(env, execID); got != execution.ExecSucceeded {
					t.Errorf("execution is %s, want SUCCEEDED", got)
				}
				if got := countEvents(env, execID, "node.succeeded"); got != 5 {
					t.Errorf("%d node.succeeded events, want 5", got)
				}
			})
		}
	})
}

// TestForkAtomic (E-REC-2): a fork creates every branch or none. A failure while
// the fork commits leaves the execution as it was, and the retry creates each
// branch exactly once.
func TestForkAtomic(t *testing.T) {
	const branches = 5
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(forkJoin(branches)))
		worker := execution.WorkerID(env.IDs.New())
		faults := &faultPlan{only: []durable.Point{durable.PointInCommit}, failAt: 1, err: errInjected}
		eng := env.Engine(durable.Config{Worker: worker, Concurrency: 1, Faults: faults}, runner())
		execID := env.Start(eng, pub, input())
		if failed := runToEnd(t, eng); failed != 1 {
			t.Fatalf("%d steps failed, want 1", failed)
		}
		if got := statusOf(env, execID); got != execution.ExecSucceeded {
			t.Fatalf("execution is %s, want SUCCEEDED", got)
		}
		if got := countEvents(env, execID, "token.forked"); got != 1 {
			t.Errorf("%d token.forked events, want 1: the fork committed more than once", got)
		}
		started := map[string]int{}
		for _, e := range env.Events(execID) {
			if e.Type == "node.started" {
				started[string(e.Reference)]++
			}
		}
		// s, p, j, after, e and every branch run once.
		if want := 5 + branches; len(started) != want {
			t.Errorf("%d distinct node executions started, want %d", len(started), want)
		}
		for ref, n := range started {
			if n != 1 {
				t.Errorf("node execution %s started %d times", ref, n)
			}
		}
	})
}

// TestJoinExactlyOneContinuation (E-REC-2): 64 tokens arrive at a join at the same
// time on 64 workers, and the join creates exactly one continuation, repeated
// join-repeats times.
func TestJoinExactlyOneContinuation(t *testing.T) {
	const branches = 64
	repeats := joinRepeats()
	if testing.Short() {
		repeats = min(repeats, 10)
	}
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		t.Parallel() // the stores are independent: they run at the same time
		pub := env.Publish(docOf(forkJoin(branches)))
		var replays, commits int
		var mu sync.Mutex
		eng := env.Engine(durable.Config{
			Concurrency: branches, PollEvery: 200 * time.Millisecond, ReapEvery: time.Second, RenewEvery: time.Second, LeaseTTL: 30 * time.Second,
			Committed: func(_ *execution.Delta, replayed bool) {
				mu.Lock()
				defer mu.Unlock()
				commits++
				if replayed {
					replays++
				}
			},
		}, runner())
		ctx, stop := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- eng.Run(ctx) }()
		t.Cleanup(func() {
			stop()
			if err := <-done; err != nil {
				t.Errorf("Run() = %v", err)
			}
		})
		// The repetitions go in rounds of several executions at once: more arrivals
		// compete for the same rows, and the 1000 repetitions fit the time of the criterion.
		const round = 4
		for first := 0; first < repeats; first += round {
			n := min(round, repeats-first)
			execs := make([]store.ExecutionID, n)
			for j := range execs {
				execs[j] = env.Start(eng, pub, input())
			}
			for j, execID := range execs {
				i := first + j
				waitFor(t, fmt.Sprintf("repeat %d to end", i), 60*time.Second, func() bool {
					return statusOf(env, execID) == execution.ExecSucceeded
				})
				if got := countEvents(env, execID, "join.fired"); got != 1 {
					t.Fatalf("repeat %d: %d join.fired events, want exactly one continuation", i, got)
				}
				if got := countEvents(env, execID, "token.consumed"); got < branches {
					t.Fatalf("repeat %d: %d token.consumed events, want at least %d", i, got, branches)
				}
				if live := env.LiveTokens(execID); len(live) != 0 {
					t.Fatalf("repeat %d: %d live tokens after the end", i, len(live))
				}
			}
		}
		mu.Lock()
		defer mu.Unlock()
		t.Logf("%d repeats of %d concurrent arrivals: %d commits, %d recomputed on a newer state", repeats, branches, commits, replays)
		if replays == 0 {
			t.Error("no commit was recomputed: the test did not exercise concurrent arrivals")
		}
	})
}

// TestEnginesShareAnExecution: two engines (two processes, on PostgreSQL) work the
// branches of one execution at the same time and the result is the one of a single
// worker.
func TestEnginesShareAnExecution(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(forkJoin(16)))
		var engines []*durable.Engine
		for range 2 {
			engines = append(engines, env.Engine(durable.Config{Concurrency: 4, PollEvery: 5 * time.Millisecond}, runner()))
		}
		execs := make([]store.ExecutionID, 10)
		for i := range execs {
			execs[i] = env.Start(engines[0], pub, input())
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
			want := []string{"join.fired"}
			var got []string
			for _, e := range env.Events(id) {
				if e.Type == "join.fired" {
					got = append(got, e.Type)
				}
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("execution %s: join.fired events (-want +got):\n%s", id, diff)
			}
		}
	})
}
