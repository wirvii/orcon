// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package journal

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

var (
	testOrg  = identity.OrgID(id.Derive("org"))
	testNode = store.NodeExecutionID(id.Derive("node"))
	t0       = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
)

// fakeDB is an in-memory journal store. It counts committed transactions and
// the transactions open at any moment, so a test can check that no transaction
// is open during a send.
type fakeDB struct {
	mu        sync.Mutex
	rows      map[store.ExternalOperationID]*store.ExternalOperation
	keys      map[string]bool
	open      int
	committed int
}

func newFakeDB() *fakeDB {
	return &fakeDB{rows: map[store.ExternalOperationID]*store.ExternalOperation{}, keys: map[string]bool{}}
}

func (f *fakeDB) InTx(ctx context.Context, _ identity.OrgID, fn func(ctx context.Context, tx store.Tx) error) error {
	f.mu.Lock()
	f.open++
	f.mu.Unlock()
	err := fn(ctx, &fakeTx{db: f})
	f.mu.Lock()
	f.open--
	if err == nil {
		f.committed++
	}
	f.mu.Unlock()
	return err
}

func (f *fakeDB) ListExternalOperations(_ context.Context, _ identity.OrgID, node store.NodeExecutionID, fromItem int64, limit int) ([]store.ExternalOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.ExternalOperation
	for _, r := range f.rows {
		if r.NodeExecutionID == node && r.ItemIndex >= fromItem {
			out = append(out, *r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ItemIndex != out[j].ItemIndex {
			return out[i].ItemIndex < out[j].ItemIndex
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// state returns the state of an operation, or "" when unknown.
func (f *fakeDB) state(id store.ExternalOperationID) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rows[id]; ok {
		return r.State
	}
	return ""
}

func (f *fakeDB) isOpen() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.open > 0
}

// seed puts operations straight into the store in a given state, as a crashed
// worker would have left them.
func (f *fakeDB) seed(ops []store.ExternalOperation, state string, deadline, dispatchedAt time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range ops {
		o := ops[i]
		o.State = state
		o.DispatchDeadlineAt = deadline
		o.FirstDispatchedAt = dispatchedAt
		f.rows[o.ID] = &o
		f.keys[o.CorrelationKey] = true
	}
}

// fakeTx implements the three transaction methods of the journal. Calls to any
// other method of store.Tx panic, which is what a test of this package wants.
type fakeTx struct {
	store.Tx
	db *fakeDB
}

func (t *fakeTx) AppendJournal(_ context.Context, ops []store.ExternalOperation) error {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	for i := range ops {
		if t.db.keys[ops[i].CorrelationKey] {
			return fmt.Errorf("append: %w", flow.ErrConflict)
		}
	}
	for i := range ops {
		c := ops[i]
		c.State = StatePrepared
		t.db.keys[c.CorrelationKey] = true
		t.db.rows[c.ID] = &c
	}
	return nil
}

func (t *fakeTx) MarkDispatching(_ context.Context, ids []store.ExternalOperationID, deadline, now time.Time) (int64, error) {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	var moved int64
	for _, id := range ids {
		r, ok := t.db.rows[id]
		if !ok || r.State != StatePrepared {
			continue
		}
		r.State = StateDispatching
		r.DispatchDeadlineAt = deadline
		r.FirstDispatchedAt = now
		moved++
	}
	return moved, nil
}

func (t *fakeTx) ResolveOperation(_ context.Context, op store.ExternalOperationID, from, to, response string, _ time.Time) (bool, error) {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	r, ok := t.db.rows[op]
	if !ok || r.State != from {
		return false, nil
	}
	r.State = to
	r.ResponseReference = response
	return true, nil
}

// fakeProvider answers from a script and checks, on every call, that the store
// already holds the operation as DISPATCHING and that no transaction is open.
type fakeProvider struct {
	db    *fakeDB
	t     *testing.T
	mu    sync.Mutex
	sends map[string]int
	looks int
	reply func(n int, a *Attempt) (Reply, error)
}

func newFakeProvider(t *testing.T, db *fakeDB, reply func(n int, a *Attempt) (Reply, error)) *fakeProvider {
	t.Helper()
	return &fakeProvider{db: db, t: t, sends: map[string]int{}, reply: reply}
}

func (p *fakeProvider) Send(_ context.Context, a *Attempt) (Reply, error) {
	if p.db.isOpen() {
		p.t.Errorf("a send started while a transaction was open")
	}
	if got := p.db.state(a.Operation.ID); got != StateDispatching {
		p.t.Errorf("a send for an operation in state %q, want DISPATCHING", got)
	}
	p.mu.Lock()
	p.sends[a.Operation.CorrelationKey]++
	n := p.sends[a.Operation.CorrelationKey]
	p.mu.Unlock()
	return p.reply(n, a)
}

func (p *fakeProvider) Lookup(_ context.Context, a *Attempt) (Reply, error) {
	p.mu.Lock()
	p.looks++
	p.mu.Unlock()
	return Reply{Outcome: StateSucceeded}, nil
}

func (p *fakeProvider) totalSends() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for _, n := range p.sends {
		total += n
	}
	return total
}

func makeOps(n int) []store.ExternalOperation {
	ops := make([]store.ExternalOperation, n)
	for i := range n {
		ops[i] = store.ExternalOperation{
			ID:                store.ExternalOperationID(id.Derive("op", strconv.Itoa(i))),
			OrgID:             testOrg,
			ExecutionID:       store.ExecutionID(id.Derive("exec")),
			NodeExecutionID:   testNode,
			ItemIndex:         int64(i),
			CorrelationKey:    CorrelationKey("exec", "t1", "", "tok", int64(i), 0),
			ContractReference: "orcon.http@1",
			CreatedAt:         t0,
		}
	}
	return ops
}

func newJournal(t *testing.T, db *fakeDB, p Provider, clk clock.Clock, cfg Config) *Journal {
	t.Helper()
	j, err := New(db, p, clk, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return j
}

func TestCorrelationKeyIsStableAndDistinct(t *testing.T) {
	a := CorrelationKey("exec", "t1", "", "tok", 3, 0)
	if a != CorrelationKey("exec", "t1", "", "tok", 3, 0) {
		t.Fatal("the key of the same item changed")
	}
	if len(a) != correlationLen {
		t.Fatalf("length %d, want %d", len(a), correlationLen)
	}
	if a == CorrelationKey("exec", "t1", "", "tok", 3, 1) {
		t.Fatal("a new retry epoch kept the key")
	}
	if a == CorrelationKey("exec", "t1", "", "tok", 4, 0) {
		t.Fatal("another item got the same key")
	}
}

func TestPrepareRejectsDuplicateKeysAndWritesNothingOfTheGroup(t *testing.T) {
	db := newFakeDB()
	j := newJournal(t, db, newFakeProvider(t, db, nil), clock.NewFake(t0), Config{})
	ops := makeOps(3)
	if err := j.Prepare(t.Context(), testOrg, ops); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if err := j.Prepare(t.Context(), testOrg, ops[:1]); !errors.Is(err, flow.ErrConflict) {
		t.Fatalf("duplicate prepare = %v, want flow.ErrConflict", err)
	}
	for _, op := range ops {
		if db.state(op.ID) != StatePrepared {
			t.Fatalf("item %d is %q, want PREPARED", op.ItemIndex, db.state(op.ID))
		}
	}
}

func TestDispatchRecordsOutcomesAndNeverSendsBeforeTheMark(t *testing.T) {
	db := newFakeDB()
	clk := clock.NewFake(t0)
	p := newFakeProvider(t, db, func(_ int, a *Attempt) (Reply, error) {
		switch a.Operation.ItemIndex % 3 {
		case 0:
			return Reply{Outcome: StateSucceeded, Response: "ok"}, nil
		case 1:
			return Reply{Outcome: StateFailed}, nil
		default:
			return Reply{Outcome: StateUncertain}, nil
		}
	})
	j := newJournal(t, db, p, clk, Config{})
	ops := makeOps(9)
	if err := j.Prepare(t.Context(), testOrg, ops); err != nil {
		t.Fatal(err)
	}
	sum, err := j.Dispatch(t.Context(), testOrg, ops, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if sum.Succeeded != 3 || sum.Failed != 3 || sum.Uncertain != 3 || sum.Pending != 0 {
		t.Fatalf("summary = %+v", sum)
	}
	if p.totalSends() != 9 {
		t.Fatalf("sends = %d, want 9", p.totalSends())
	}
}

func TestGroupCommitBudget(t *testing.T) {
	const n = 10000
	db := newFakeDB()
	clk := clock.NewFake(t0)
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	j := newJournal(t, db, p, clk, Config{Concurrency: 64})
	ops := makeOps(n)
	if err := j.Prepare(t.Context(), testOrg, ops); err != nil {
		t.Fatal(err)
	}
	sum, err := j.Dispatch(t.Context(), testOrg, ops, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if sum.Succeeded != n {
		t.Fatalf("succeeded = %d, want %d", sum.Succeeded, n)
	}
	limit := ceilDiv(n, outcomeGroup) + ceilDiv(n, markGroup) + 10
	if db.committed > limit {
		t.Fatalf("transactions = %d, want <= %d", db.committed, limit)
	}
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

func TestUncertainSuspends(t *testing.T) {
	db := newFakeDB()
	clk := clock.NewFake(t0)
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{}, errors.New("connection reset")
	})
	j := newJournal(t, db, p, clk, Config{})
	ops := makeOps(2)
	if err := j.Prepare(t.Context(), testOrg, ops); err != nil {
		t.Fatal(err)
	}
	sum, err := j.Dispatch(t.Context(), testOrg, ops, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Uncertain != 2 {
		t.Fatalf("uncertain = %d, want 2", sum.Uncertain)
	}
	// A recovery well after the deadline must not resend an unsafe operation.
	clk.Advance(time.Hour)
	rsum, err := j.Recover(t.Context(), testOrg, testNode, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatal(err)
	}
	if rsum.Pending != 0 || p.totalSends() != 2 {
		t.Fatalf("recovery %+v sent again: sends=%d", rsum, p.totalSends())
	}
}

func TestRecoverDispatching(t *testing.T) {
	db := newFakeDB()
	clk := clock.NewFake(t0)
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	const timeout, grace = 30 * time.Second, 5 * time.Second
	j := newJournal(t, db, p, clk, Config{AttemptTimeout: timeout, Grace: grace})
	ops := makeOps(3)
	deadline := t0.Add(timeout)
	db.seed(ops, StateDispatching, deadline, t0)
	contract := Contract{Idempotency: IdempotencyProviderKey, KeyTTL: time.Hour}

	// Before the deadline plus grace, nothing is touched: the worker may be alive.
	clk.Set(deadline.Add(grace - time.Second))
	sum, err := j.Recover(t.Context(), testOrg, testNode, contract)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Pending != 3 || p.totalSends() != 0 {
		t.Fatalf("early recovery acted: %+v sends=%d", sum, p.totalSends())
	}

	// After it, a key still inside its TTL is resent with the same key.
	clk.Set(deadline.Add(grace + time.Second))
	sum, err = j.Recover(t.Context(), testOrg, testNode, contract)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Succeeded != 3 || p.totalSends() != 3 {
		t.Fatalf("recovery = %+v sends=%d", sum, p.totalSends())
	}
	for _, op := range ops {
		if db.state(op.ID) != StateSucceeded {
			t.Fatalf("item %d is %q", op.ItemIndex, db.state(op.ID))
		}
	}
}

func TestKeyTTLExpired(t *testing.T) {
	db := newFakeDB()
	clk := clock.NewFake(t0)
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	const timeout, grace = 30 * time.Second, 5 * time.Second
	j := newJournal(t, db, p, clk, Config{AttemptTimeout: timeout, Grace: grace})
	ops := makeOps(2)
	dispatched := t0.Add(-2 * time.Hour)
	db.seed(ops, StateDispatching, dispatched.Add(timeout), dispatched)
	clk.Set(t0)

	sum, err := j.Recover(t.Context(), testOrg, testNode, Contract{Idempotency: IdempotencyProviderKey, KeyTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if p.totalSends() != 0 {
		t.Fatalf("a key past its TTL was resent %d times", p.totalSends())
	}
	if sum.Uncertain != 2 {
		t.Fatalf("summary = %+v, want both UNCERTAIN", sum)
	}
}

func TestDispatchFailsClosedWhenItemIsNotPrepared(t *testing.T) {
	db := newFakeDB()
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	j := newJournal(t, db, p, clock.NewFake(t0), Config{})
	ops := makeOps(2)
	// Never prepared: the mark moves nothing and nothing is sent.
	_, err := j.Dispatch(t.Context(), testOrg, ops, Contract{Idempotency: IdempotencyNone})
	if !errors.Is(err, ErrNotPrepared) {
		t.Fatalf("err = %v, want ErrNotPrepared", err)
	}
	if p.totalSends() != 0 {
		t.Fatal("an unprepared operation was sent")
	}
}

func TestSendGuardKeepsLateItemsInDispatching(t *testing.T) {
	db := newFakeDB()
	clk := clock.NewFake(t0)
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	j := newJournal(t, db, p, clk, Config{AttemptTimeout: time.Second})
	ops := makeOps(1)
	if err := j.Prepare(t.Context(), testOrg, ops); err != nil {
		t.Fatal(err)
	}
	// A timeout of one second leaves no room before the guard: nothing is sent.
	sum, err := j.Dispatch(t.Context(), testOrg, ops, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatal(err)
	}
	if p.totalSends() != 0 || sum.Pending != 1 {
		t.Fatalf("sends=%d summary=%+v", p.totalSends(), sum)
	}
}
