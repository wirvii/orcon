// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/flow"
)

// countingRunner records how many activations of the execution run at once.
type countingRunner struct {
	inFlight atomic.Int64
	peak     atomic.Int64
}

func (c *countingRunner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if call.Node.ID == "after" || call.Node.ID == "s" || call.Node.ID == "e" {
		return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
	}
	n := c.inFlight.Add(1)
	defer c.inFlight.Add(-1)
	for {
		p := c.peak.Load()
		if n <= p || c.peak.CompareAndSwap(p, n) {
			break
		}
	}
	select {
	case <-ctx.Done():
		return execution.NodeOutput{}, ctx.Err()
	case <-time.After(15 * time.Millisecond):
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
}

// TestExecutionLimitCapsLeasedTokens: eight parallel branches of one execution
// never run more than the execution limit at once, on both stores (ADR-0014 §2.2).
func TestExecutionLimitCapsLeasedTokens(t *testing.T) {
	const limit = 2
	eachStore(t, storetest.EngineOptions{}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(forkJoin(8)))
		runner := &countingRunner{}
		eng := env.Engine(durable.Config{Concurrency: 8, ExecutionLimit: limit, PollEvery: 5 * time.Millisecond, ShutdownGrace: time.Second},
			execution.Deps{Runner: runner})
		execID := env.Start(eng, pub, input())
		env.Drain(eng)
		if got := env.Execution(execID).Status; got != "SUCCEEDED" {
			t.Fatalf("execution status = %s, want SUCCEEDED", got)
		}
		if peak := runner.peak.Load(); peak > limit {
			t.Fatalf("peak concurrent activations = %d, want <= %d", peak, limit)
		}
	})
}

// TestRunnableCountsSnapshot: the claim snapshot counts the runnable tokens per
// fairness key and priority on both stores.
func TestRunnableCountsSnapshot(t *testing.T) {
	eachStore(t, storetest.EngineOptions{}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		pub := env.Publish(docOf(sequence(1)))
		eng := env.Engine(durable.Config{}, execution.Deps{Runner: &countingRunner{}})
		env.Start(eng, pub, input())
		// The database may stamp the token with its own clock: a point well after both clocks.
		counts, err := env.Store.RunnableCounts(t.Context(), time.Now().Add(time.Hour))
		if err != nil {
			t.Fatalf("RunnableCounts: %v", err)
		}
		var total int64
		for _, c := range counts {
			total += c.Runnable
		}
		if total != 1 {
			t.Fatalf("runnable tokens in the snapshot = %d, want 1 (%+v)", total, counts)
		}
	})
}
