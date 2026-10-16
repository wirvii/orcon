// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
)

// timedRunner runs the nodes like the plain runner and reports when each one
// started, so that a test can measure how long a claim took.
type timedRunner struct {
	inner *executiontest.TraceRunner
	first chan time.Time
}

func (r *timedRunner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	select {
	case r.first <- time.Now():
	default:
	}
	return r.inner.Execute(ctx, call)
}

// claimLatencies starts n executions one after the other, each from an engine that
// only starts them, while a worker engine with cfg runs them. It returns how long
// each took to reach its first node, measured from before the start. The worker is
// idle between two starts, as it is at rest.
//
//nolint:gocritic // hugeParam: the tests write the configuration as a literal at the call
func claimLatencies(t *testing.T, env *storetest.EngineEnv, cfg durable.Config, n int, afterRun func()) []time.Duration {
	t.Helper()
	pub := env.Publish(docOf(sequence(1)))
	starter := env.Engine(durable.Config{}, runner())
	tr := &timedRunner{inner: &executiontest.TraceRunner{}, first: make(chan time.Time, 4*n+8)}
	worker := env.Engine(cfg, execution.Deps{Runner: tr})
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	defer func() {
		stop()
		if err := <-done; err != nil {
			t.Errorf("Run() = %v", err)
		}
	}()
	if afterRun != nil {
		afterRun()
	}
	// One warm-up run loads the plan, which is not what the measure is about.
	var out []time.Duration
	for i := range n + 1 {
		time.Sleep(30 * time.Millisecond)
		start := time.Now()
		execID := env.Start(starter, pub, input())
		select {
		case at := <-tr.first:
			if i > 0 {
				out = append(out, at.Sub(start))
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("execution %s was not claimed within 10 s", execID)
		}
		waitFor(t, "the execution to end", 10*time.Second, func() bool { return statusOf(env, execID) == execution.ExecSucceeded })
	}
	return out
}

// p99 returns the 99th percentile of the samples (the largest one for fewer than 100).
func p99(samples []time.Duration) time.Duration {
	s := slices.Clone(samples)
	slices.Sort(s)
	return s[min(len(s)-1, (len(s)*99+99)/100-1)]
}

// TestNotifyFallbackPoll (E-REC-3, SPEC-001 C4b): with the immediate notice off,
// work is claimed by polling within 2 s; with it on (PostgreSQL LISTEN/NOTIFY, sent
// by another engine), a claim at rest takes p99 of at most 50 ms; and when the
// listening connection is lost, the work still is claimed within 2 s by polling.
func TestNotifyFallbackPoll(t *testing.T) {
	t.Run("polling alone claims within 2 s", func(t *testing.T) {
		eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) { //nolint:thelper // the body of a subtest, not a helper
			lat := claimLatencies(t, env, durable.Config{Concurrency: 2, DisableNotify: true, DisableWake: true, PollEvery: time.Second}, 5, nil)
			for _, d := range lat {
				if d > 2*time.Second {
					t.Errorf("a claim by polling took %v, want at most 2 s (poll every 1 s)", d)
				}
			}
		})
	})

	t.Run("the notice claims at rest within p99 of 50 ms", func(t *testing.T) {
		eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) { //nolint:thelper // the body of a subtest, not a helper
			if env.Kind != storetest.PostgreSQL {
				t.Skip("SQLite has no notice across processes: its claims are the in-process wake-up, covered by the polling case")
			}
			// The poll is far longer than the test: only the notice can make these claims.
			lat := claimLatencies(t, env, durable.Config{Concurrency: 2, PollEvery: 10 * time.Second}, 100, nil)
			if got := p99(lat); got > 50*time.Millisecond {
				t.Errorf("claim p99 by notice = %v over %d claims, want at most 50 ms", got, len(lat))
			}
		})
	})

	t.Run("work is claimed within 2 s when the listening connection is lost", func(t *testing.T) {
		eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) { //nolint:thelper // the body of a subtest, not a helper
			if env.Kind != storetest.PostgreSQL {
				t.Skip("the listening connection exists on PostgreSQL only")
			}
			lat := claimLatencies(t, env, durable.Config{Concurrency: 2, PollEvery: time.Second}, 5, func() { killListeners(t, env) })
			for _, d := range lat {
				if d > 2*time.Second {
					t.Errorf("a claim after the listener was lost took %v, want at most 2 s", d)
				}
			}
		})
	})
}

// killListeners ends the listening connection of the engine that runs, as a network
// fault or a restart of the server would (storetest.EndNoticeListeners).
func killListeners(t *testing.T, env *storetest.EngineEnv) {
	t.Helper()
	storetest.EndNoticeListeners(t, env.Store)
}
