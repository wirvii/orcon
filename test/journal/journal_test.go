// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package journal_test runs the external operation journal (SPEC-001 cut C6)
// against the real stores: SQLite always, PostgreSQL when -store asks for it.
package journal_test

import (
	"context"
	"errors"
	"flag"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/journal"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

func TestMain(m *testing.M) {
	flag.Parse()
	kinds, _ := storetest.KindsFor(executiontest.StoreFlag("auto"))
	if slices.Contains(kinds, storetest.PostgreSQL) {
		storetest.EnablePostgres()
		os.Exit(runWithPostgres(m)) //nolint:forbidigo // AL-8: TestMain ends the test process with the code of the run
	}
	goleak.VerifyTestMain(m)
}

func eachStore(t *testing.T, fn func(t *testing.T, env *storetest.EngineEnv)) {
	t.Helper()
	kinds, err := storetest.KindsFor(executiontest.StoreFlag("auto"))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			fn(t, storetest.NewEngineEnv(t, kind, storetest.EngineOptions{Weak: true}))
		})
	}
}

// countingStore counts the transactions the journal opens.
type countingStore struct {
	storetest.Store
	txs atomic.Int64
}

func (c *countingStore) InTx(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx store.Tx) error) error {
	c.txs.Add(1)
	return c.Store.InTx(ctx, org, fn)
}

// provider answers from a script. Before each send it checks, in the store,
// that the operation is DISPATCHING: a send for an item the store does not hold
// as DISPATCHING is a violation of ADR-0012 section 4.
type provider struct {
	t     *testing.T
	env   *storetest.EngineEnv
	mu    sync.Mutex
	sends map[string]int
	reply func(n int, a *journal.Attempt) (journal.Reply, error)
}

func (p *provider) Send(ctx context.Context, a *journal.Attempt) (journal.Reply, error) {
	got, err := p.env.Store.GetExternalOperation(ctx, storetest.ScopeOf(p.env.Org), a.Operation.ID)
	if err != nil || got.State != journal.StateDispatching {
		p.t.Errorf("a send for an operation the store does not hold as DISPATCHING (state %q, err %v)", got.State, err)
	}
	return p.do(a)
}

func (p *provider) Lookup(_ context.Context, a *journal.Attempt) (journal.Reply, error) {
	return journal.Reply{Outcome: journal.StateSucceeded}, nil
}

func (p *provider) do(a *journal.Attempt) (journal.Reply, error) {
	p.mu.Lock()
	p.sends[a.Operation.CorrelationKey]++
	n := p.sends[a.Operation.CorrelationKey]
	p.mu.Unlock()
	return p.reply(n, a)
}

func (p *provider) total() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for _, n := range p.sends {
		total += n
	}
	return total
}

// noRunner is the runner of the scenario executions; they never run a node.
type noRunner struct{}

func (noRunner) Execute(context.Context, execution.NodeCall) (execution.NodeOutput, error) {
	return execution.NodeOutput{}, errors.New("the journal scenarios do not run nodes")
}

// scenario is an execution with one node execution the operations belong to.
type scenario struct {
	env     *storetest.EngineEnv
	execID  store.ExecutionID
	nodeID  store.NodeExecutionID
	tokenID store.TokenID
}

func newScenario(t *testing.T, env *storetest.EngineEnv) scenario {
	t.Helper()
	ctx := t.Context()
	doc := singleTask()
	pub := env.Publish(&doc)
	eng := env.Engine(durable.Config{}, execution.Deps{Runner: noRunner{}})
	execID := env.Start(eng, pub, executiontest.Batch(`{"n":1}`))
	tok := env.LiveTokens(execID)[0]
	node := store.NodeExecution{
		ID:          store.NodeExecutionID(env.IDs.New()),
		OrgID:       env.Org,
		ExecutionID: execID,
		TokenID:     tok.ID,
		NodeID:      "t1",
		Status:      "RUNNING",
		StartedAt:   env.Clock.Now(),
		CreatedAt:   env.Clock.Now(),
		UpdatedAt:   env.Clock.Now(),
	}
	err := env.Store.InTx(ctx, storetest.ScopeOf(env.Org), func(ctx context.Context, tx store.Tx) error {
		return tx.InsertNodeExecution(ctx, node)
	})
	if err != nil {
		t.Fatalf("insert node execution: %v", err)
	}
	return scenario{env: env, execID: execID, nodeID: node.ID, tokenID: tok.ID}
}

func (s scenario) ops(n int) []store.ExternalOperation {
	out := make([]store.ExternalOperation, n)
	for i := range n {
		out[i] = store.ExternalOperation{
			ID:                store.ExternalOperationID(s.env.IDs.New()),
			OrgID:             s.env.Org,
			ExecutionID:       s.execID,
			NodeExecutionID:   s.nodeID,
			ItemIndex:         int64(i),
			CorrelationKey:    journal.CorrelationKey(s.execID.String(), "t1", "", s.tokenID.String(), int64(i), 0),
			ContractReference: "orcon.http@1",
			CreatedAt:         s.env.Clock.Now(),
		}
	}
	return out
}

func (s scenario) state(t *testing.T, id store.ExternalOperationID) string {
	t.Helper()
	got, err := s.env.Store.GetExternalOperation(t.Context(), storetest.ScopeOf(s.env.Org), id)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	return got.State
}

func singleTask() flow.Document { return sequenceDoc() }

func newJournal(t *testing.T, st journal.Store, p journal.Provider, clk clock.Clock, cfg journal.Config) *journal.Journal {
	t.Helper()
	j, err := journal.New(st, p, clk, cfg)
	if err != nil {
		t.Fatalf("journal.New: %v", err)
	}
	return j
}

func TestDispatchRecordsOutcomes(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		s := newScenario(t, env)
		p := &provider{t: t, env: env, sends: map[string]int{}, reply: func(_ int, a *journal.Attempt) (journal.Reply, error) {
			switch a.Operation.ItemIndex % 3 {
			case 0:
				return journal.Reply{Outcome: journal.StateSucceeded}, nil
			case 1:
				return journal.Reply{Outcome: journal.StateFailed}, nil
			default:
				return journal.Reply{Outcome: journal.StateUncertain}, nil
			}
		}}
		j := newJournal(t, env.Store, p, env.Clock, journal.Config{})
		ops := s.ops(9)
		if err := j.Prepare(t.Context(), storetest.ScopeOf(env.Org), ops); err != nil {
			t.Fatal(err)
		}
		sum, err := j.Dispatch(t.Context(), storetest.ScopeOf(env.Org), ops, journal.Contract{Idempotency: journal.IdempotencyNone})
		if err != nil {
			t.Fatalf("Dispatch: %v", err)
		}
		if sum.Succeeded != 3 || sum.Failed != 3 || sum.Uncertain != 3 || sum.Pending != 0 {
			t.Fatalf("summary = %+v", sum)
		}
		if s.state(t, ops[1].ID) != journal.StateFailed {
			t.Fatalf("item 1 = %q", s.state(t, ops[1].ID))
		}
	})
}

func TestGroupCommitBudget(t *testing.T) {
	const n = 10000
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		s := newScenario(t, env)
		counting := &countingStore{Store: env.Store}
		p := &provider{t: t, env: env, sends: map[string]int{}, reply: func(int, *journal.Attempt) (journal.Reply, error) {
			return journal.Reply{Outcome: journal.StateSucceeded}, nil
		}}
		j := newJournal(t, counting, p, env.Clock, journal.Config{Concurrency: 64})
		ops := s.ops(n)
		if err := j.Prepare(t.Context(), storetest.ScopeOf(env.Org), ops); err != nil {
			t.Fatal(err)
		}
		sum, err := j.Dispatch(t.Context(), storetest.ScopeOf(env.Org), ops, journal.Contract{Idempotency: journal.IdempotencyNone})
		if err != nil {
			t.Fatalf("Dispatch: %v", err)
		}
		if sum.Succeeded != n {
			t.Fatalf("succeeded = %d, want %d", sum.Succeeded, n)
		}
		limit := (n+511)/512 + (n+999)/1000 + 10
		if got := counting.txs.Load(); got > int64(limit) {
			t.Fatalf("transactions = %d, want <= %d", got, limit)
		}
	})
}

func TestRecoverDispatching(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		s := newScenario(t, env)
		start := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
		clk := clock.NewFake(start)
		const timeout, grace = 30 * time.Second, 5 * time.Second
		failing := &provider{t: t, env: env, sends: map[string]int{}, reply: func(int, *journal.Attempt) (journal.Reply, error) {
			return journal.Reply{}, errors.New("connection reset")
		}}
		contract := journal.Contract{Idempotency: journal.IdempotencyProviderKey, KeyTTL: time.Hour}
		j := newJournal(t, env.Store, failing, clk, journal.Config{AttemptTimeout: timeout, Grace: grace})
		ops := s.ops(3)
		if err := j.Prepare(t.Context(), storetest.ScopeOf(env.Org), ops); err != nil {
			t.Fatal(err)
		}
		// A provider key keeps a transport failure in DISPATCHING for recovery.
		sum, err := j.Dispatch(t.Context(), storetest.ScopeOf(env.Org), ops, contract)
		if err != nil || sum.Pending != 3 {
			t.Fatalf("dispatch = %+v, %v", sum, err)
		}

		healthy := &provider{t: t, env: env, sends: map[string]int{}, reply: func(int, *journal.Attempt) (journal.Reply, error) {
			return journal.Reply{Outcome: journal.StateSucceeded}, nil
		}}
		j = newJournal(t, env.Store, healthy, clk, journal.Config{AttemptTimeout: timeout, Grace: grace})
		clk.Set(start.Add(timeout + grace - time.Second))
		sum, err = j.Recover(t.Context(), storetest.ScopeOf(env.Org), s.nodeID, contract)
		if err != nil || sum.Pending != 3 || healthy.total() != 0 {
			t.Fatalf("early recovery acted: %+v sends=%d err=%v", sum, healthy.total(), err)
		}
		clk.Set(start.Add(timeout + grace + time.Second))
		sum, err = j.Recover(t.Context(), storetest.ScopeOf(env.Org), s.nodeID, contract)
		if err != nil || sum.Succeeded != 3 {
			t.Fatalf("recovery = %+v err=%v", sum, err)
		}
		if healthy.total() != 3 || s.state(t, ops[0].ID) != journal.StateSucceeded {
			t.Fatalf("sends=%d state=%q", healthy.total(), s.state(t, ops[0].ID))
		}
	})
}

func TestUncertainSuspends(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		s := newScenario(t, env)
		clk := clock.NewFake(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
		p := &provider{t: t, env: env, sends: map[string]int{}, reply: func(int, *journal.Attempt) (journal.Reply, error) {
			return journal.Reply{}, errors.New("connection reset")
		}}
		j := newJournal(t, env.Store, p, clk, journal.Config{})
		ops := s.ops(2)
		if err := j.Prepare(t.Context(), storetest.ScopeOf(env.Org), ops); err != nil {
			t.Fatal(err)
		}
		sum, err := j.Dispatch(t.Context(), storetest.ScopeOf(env.Org), ops, journal.Contract{Idempotency: journal.IdempotencyNone})
		if err != nil || sum.Uncertain != 2 {
			t.Fatalf("dispatch = %+v, %v", sum, err)
		}
		clk.Advance(time.Hour)
		if _, err := j.Recover(t.Context(), storetest.ScopeOf(env.Org), s.nodeID, journal.Contract{Idempotency: journal.IdempotencyNone}); err != nil {
			t.Fatal(err)
		}
		if p.total() != 2 {
			t.Fatalf("an UNSAFE operation was sent again: %d sends", p.total())
		}
		if s.state(t, ops[0].ID) != journal.StateUncertain {
			t.Fatalf("state = %q, want UNCERTAIN", s.state(t, ops[0].ID))
		}
	})
}

func TestKeyTTLExpired(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		s := newScenario(t, env)
		now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
		clk := clock.NewFake(now.Add(-2 * time.Hour))
		failing := &provider{t: t, env: env, sends: map[string]int{}, reply: func(int, *journal.Attempt) (journal.Reply, error) {
			return journal.Reply{}, errors.New("connection reset")
		}}
		contract := journal.Contract{Idempotency: journal.IdempotencyProviderKey, KeyTTL: time.Hour}
		j := newJournal(t, env.Store, failing, clk, journal.Config{AttemptTimeout: 30 * time.Second, Grace: 5 * time.Second})
		ops := s.ops(2)
		if err := j.Prepare(t.Context(), storetest.ScopeOf(env.Org), ops); err != nil {
			t.Fatal(err)
		}
		if _, err := j.Dispatch(t.Context(), storetest.ScopeOf(env.Org), ops, contract); err != nil {
			t.Fatal(err)
		}
		clk.Set(now)
		sum, err := j.Recover(t.Context(), storetest.ScopeOf(env.Org), s.nodeID, contract)
		if err != nil {
			t.Fatal(err)
		}
		if failing.total() != 2 || sum.Uncertain != 2 {
			t.Fatalf("sends=%d summary=%+v: a key past its TTL must not be resent", failing.total(), sum)
		}
	})
}

// sequenceDoc is a flow of one task between a start and an end event.
func sequenceDoc() flow.Document {
	return executiontest.NewFlow("journal").Start("s").Task("t1").End("e").Chain("s", "t1", "e").Doc()
}
