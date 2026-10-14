// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// envRunner makes the output of every activation depend on the nondeterminism
// the engine gives it (the activation instant and the seed), the way `$now` and
// `Math.random()` do, so that a recomputation that regenerated them differently
// would show in the result.
type envRunner struct {
	inner execution.NodeRunner
}

func (r envRunner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	out, err := r.inner.Execute(ctx, call)
	if err != nil {
		return out, err
	}
	stamp := strconv.FormatInt(call.Env.Time.UnixMicro(), 10) + "/" + strconv.FormatUint(call.Env.Seed, 16)
	for i := range out.Ports {
		items := out.Ports[i].Batch.Items()
		for j := range items {
			items[j].JSON = items[j].JSON.With("env_"+string(call.Node.ID), jsonvalue.String(stamp))
		}
		out.Ports[i].Batch = execution.NewBatch(items...)
	}
	return out, nil
}

// canon renders a result without the identities of its rows, which depend on the
// execution, and without the order in which concurrent branches ran, which depends
// on scheduling (ADR-0009 section 7: events of concurrent branches may interleave),
// so two executions of the same flow can be compared. The order that matters is
// kept by the rows themselves: a node execution exists once, in a state, in a scope.
func canon(res *execution.Result) []string {
	var out []string
	out = append(out, "status "+string(res.Status))
	for i := range res.Events {
		e := res.Events[i]
		e.Sequence = 0
		e.Scope = execution.ScopePath(executiontest.MaskEntries(string(e.Scope)))
		out = append(out, "event "+e.String())
	}
	for _, ne := range res.NodeExecutions {
		out = append(out, fmt.Sprintf("node %s %s %s", ne.Node, executiontest.MaskEntries(string(ne.Scope)), ne.State))
	}
	for _, tok := range res.Tokens {
		out = append(out, fmt.Sprintf("token %s %s %s %d", tok.Node, executiontest.MaskEntries(string(tok.Scope)), tok.State, len(tok.Lineage)))
	}
	for _, end := range res.Ends {
		out = append(out, fmt.Sprintf("end %s %s %v", end.Node, executiontest.MaskEntries(string(end.Scope)), executiontest.Texts(end.Batch)))
	}
	slices.Sort(out)
	return out
}

// collect gathers the deltas of the commits of an execution that several
// engines (a worker and the one that replaces it) work in turn.
type collect struct {
	mu     sync.Mutex
	res    execution.Result
	order  []execution.TokenID
	tokens map[execution.TokenID]execution.TokenRow
}

func (c *collect) committed(d *execution.Delta, _ bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens == nil {
		c.tokens = make(map[execution.TokenID]execution.TokenRow)
	}
	c.res.Status = d.State
	for i := range d.Events {
		e := d.Events[i]
		e.Sequence = len(c.res.Events) + 1
		c.res.Events = append(c.res.Events, e)
	}
	c.res.NodeExecutions = append(c.res.NodeExecutions, d.NodeExecutions...)
	c.res.Ends = append(c.res.Ends, d.Ends...)
	for i := range d.Tokens {
		row := &d.Tokens[i]
		c.order = append(c.order, row.ID)
		c.tokens[row.ID] = *row
	}
	for i := range d.Changed {
		c.tokens[d.Changed[i].Row.ID] = d.Changed[i].Row
	}
}

func (c *collect) result() execution.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	res := c.res
	res.Tokens = nil
	for _, id := range c.order {
		row := c.tokens[id]
		res.Tokens = append(res.Tokens, execution.TokenRecord{ID: row.ID, State: row.State, Node: row.Node, Scope: row.Scope, Lineage: row.Lineage})
	}
	return res
}

// runProgram runs a flow to its end. When crashAt is not zero, the nth point of
// the run where a process could die returns an error as if it had: the engine is
// replaced by a new one, as after a restart, once the lease of the dead worker is
// given back.
func runProgram(t *testing.T, env *storetest.EngineEnv, pub *storetest.Published, prog executiontest.Program, execID store.ExecutionID, crashAt int) (execution.Result, int) {
	t.Helper()
	fixed := clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
	deps := prog.Deps()
	deps.Runner = envRunner{inner: deps.Runner}
	var col collect
	worker := execution.WorkerID(env.IDs.New())
	faults := &faultPlan{failAt: crashAt, err: errInjected}
	cfg := durable.Config{Worker: worker, Concurrency: 1, Clock: fixed, Committed: col.committed, Faults: faults}
	eng := env.Engine(cfg, deps)
	execID = env.StartAs(eng, pub, prog.Input, execID)
	for crashes := 0; ; {
		claimed, start, err := eng.Claim(t.Context(), 1)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(claimed) == 0 {
			break
		}
		err = eng.Work(t.Context(), claimed[0], start)
		if err == nil {
			continue
		}
		if !errors.Is(err, errInjected) {
			t.Fatalf("work: %v", err)
		}
		crashes++
		if crashes > 1 {
			t.Fatal("the injected failure happened twice")
		}
		// The process died: its leases come back, a new process takes over with nothing in memory.
		if _, err := env.Store.ReleaseWorkerLeases(t.Context(), worker, 100, time.Now()); err != nil {
			t.Fatalf("release the leases of the dead worker: %v", err)
		}
		worker = execution.WorkerID(env.IDs.New())
		cfg.Worker, cfg.Faults = worker, nil
		eng = env.Engine(cfg, deps)
	}
	if got := statusOf(env, execID); got != execution.ExecSucceeded && got != execution.ExecFailed {
		t.Fatalf("execution is %s after the run", got)
	}
	return col.result(), faults.count()
}

func seededBytes(seed uint64, size int) []byte {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) //nolint:gosec // G404: deterministic test data, not a secret
	data := make([]byte, 0, size+8)
	for len(data) < size {
		data = binary.LittleEndian.AppendUint64(data, r.Uint64())
	}
	return data[:size]
}

// TestRecomputeDeterminism (E-REC-4, FI-16): runs interrupted at a random point
// between confirmation points produce the outputs, the journal and the business
// events of the run that was not interrupted. The environment of every activation
// (instant and seed) is part of the output, so a recomputation that drew new ones
// would show.
func TestRecomputeDeterminism(t *testing.T) {
	iterations := iterations()
	if testing.Short() {
		iterations = min(iterations, 10)
	}
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		// The two runs of an iteration are the same execution in two databases, so every id they derive
		// from the execution (and the seed of each activation) is the same.
		other := storetest.NewEngineEnv(t, env.Kind, storetest.EngineOptions{Weak: true})
		rng := rand.New(rand.NewPCG(7, 11)) //nolint:gosec // G404: picks the point to interrupt, not a secret
		crashes := map[string]int{}
		for i := range iterations {
			prog := executiontest.Generate(seededBytes(uint64(10_000+i), 160), executiontest.GenOptions{MaxNodes: 30})
			doc := prog.Flow.Doc()
			execID := store.ExecutionID(id.Derive("determinism", strconv.Itoa(i)))
			reference, points := runProgram(t, env, env.Publish(&doc), prog, execID, 0)
			if points == 0 {
				t.Fatalf("iteration %d: the run passed no point where a process could die", i)
			}
			crashAt := 1 + rng.IntN(points)
			interrupted, _ := runProgram(t, other, other.Publish(&doc), prog, execID, crashAt)
			if diff := cmp.Diff(canon(&reference), canon(&interrupted)); diff != "" {
				t.Fatalf("iteration %d: the run interrupted at point %d of %d differs from the run that was not (-uninterrupted +interrupted):\n%s", i, crashAt, points, diff)
			}
			crashes[bucket(crashAt, points)]++
		}
		t.Logf("%d interrupted runs: %v", iterations, crashes)
	})
}

func bucket(at, of int) string {
	switch {
	case at*4 <= of:
		return "early"
	case at*4 <= 3*of:
		return "middle"
	default:
		return "late"
	}
}

// TestActivationEnvironment: every activation gets the instant of the clock of the
// engine and a seed of its own (ADR-0013 section 7).
func TestActivationEnvironment(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		var seen []string
		var mu sync.Mutex
		rec := recordingRunner(func(call execution.NodeCall) {
			mu.Lock()
			defer mu.Unlock()
			seen = append(seen, string(call.Node.ID)+"/"+call.Env.String())
		})
		pub := env.Publish(docOf(sequence(2)))
		clk := clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
		eng := env.Engine(durable.Config{Concurrency: 1, Clock: clk}, execution.Deps{Runner: rec})
		env.Start(eng, pub, input())
		env.Drain(eng)
		mu.Lock()
		defer mu.Unlock()
		if len(seen) != 2 {
			t.Fatalf("activations = %v, want two", seen)
		}
		for _, s := range seen {
			if !strings.Contains(s, "2026-10-13T12:00:00") {
				t.Errorf("activation %s does not carry the instant of the clock of the engine", s)
			}
		}
		if seen[0][strings.Index(seen[0], "/")+1:] == seen[1][strings.Index(seen[1], "/")+1:] {
			t.Errorf("two node executions got the same environment: %v", seen)
		}
	})
}

type recordingRunner func(call execution.NodeCall)

func (f recordingRunner) Execute(_ context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	f(call)
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: "main", Batch: call.Input}}}, nil
}
