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
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

var (
	testOrg   = identity.OrgID(id.Derive("org"))
	testScope = scopeFor(testOrg)
	testNode  = store.NodeExecutionID(id.Derive("node"))
	t0        = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
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
	// dbClock is the database clock. It is separate from the application clock
	// of a test, so a test can show which one decides.
	dbClock *clock.Fake
}

func newFakeDB() *fakeDB {
	return &fakeDB{rows: map[store.ExternalOperationID]*store.ExternalOperation{}, keys: map[string]bool{}, dbClock: clock.NewFake(t0)}
}

// image is a copy of the rows and keys of a fakeDB, taken to roll a transaction back.
type image struct {
	rows map[store.ExternalOperationID]store.ExternalOperation
	keys map[string]bool
}

// imageLocked copies the content of f. The caller holds f.mu.
func (f *fakeDB) imageLocked() image {
	img := image{rows: make(map[store.ExternalOperationID]store.ExternalOperation, len(f.rows)), keys: make(map[string]bool, len(f.keys))}
	for id, r := range f.rows {
		img.rows[id] = *r
	}
	for k := range f.keys {
		img.keys[k] = true
	}
	return img
}

// restoreLocked puts back the content of img. The caller holds f.mu.
func (f *fakeDB) restoreLocked(img image) {
	f.rows = make(map[store.ExternalOperationID]*store.ExternalOperation, len(img.rows))
	for id := range img.rows {
		r := img.rows[id]
		f.rows[id] = &r
	}
	f.keys = img.keys
}

// InTx runs fn as one transaction: when fn returns an error, the writes of fn are
// rolled back, as the storage adapters do.
func (f *fakeDB) InTx(ctx context.Context, _ orgscope.Org, fn func(ctx context.Context, tx store.Tx) error) error {
	f.mu.Lock()
	f.open++
	before := f.imageLocked()
	f.mu.Unlock()
	err := fn(ctx, &fakeTx{db: f})
	f.mu.Lock()
	f.open--
	if err == nil {
		f.committed++
	} else {
		f.restoreLocked(before)
	}
	f.mu.Unlock()
	return err
}

// ListExternalOperations returns the rows of node after the cursor (fromItem, fromID)
// in (item index, id) order, as the adapters do.
func (f *fakeDB) ListExternalOperations(_ context.Context, _ orgscope.Org, node store.NodeExecutionID, fromItem int64, fromID store.ExternalOperationID, limit int) ([]store.ExternalOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.ExternalOperation
	for _, r := range f.rows {
		after := r.ItemIndex > fromItem || (r.ItemIndex == fromItem && r.ID.String() > fromID.String())
		if r.NodeExecutionID == node && after {
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

// deadline returns the dispatch deadline stored for an operation.
func (f *fakeDB) deadline(id store.ExternalOperationID) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rows[id]; ok {
		return r.DispatchDeadlineAt
	}
	return time.Time{}
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
func (f *fakeDB) seed(ops []store.ExternalOperation, deadline, dispatchedAt time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range ops {
		o := ops[i]
		o.State = StateDispatching
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

// MarkDispatching sets the deadline from the database clock, as the adapters do.
func (t *fakeTx) MarkDispatching(_ context.Context, ids []store.ExternalOperationID, attemptTimeout time.Duration) (int64, error) {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	now := t.db.dbClock.Now()
	var moved int64
	for _, id := range ids {
		r, ok := t.db.rows[id]
		if !ok || r.State != StatePrepared {
			continue
		}
		r.State = StateDispatching
		r.DispatchDeadlineAt = now.Add(attemptTimeout)
		r.FirstDispatchedAt = now
		moved++
	}
	return moved, nil
}

func (t *fakeTx) DatabaseNow(context.Context) (time.Time, error) {
	return t.db.dbClock.Now(), nil
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
	if err := j.Prepare(t.Context(), testScope, ops); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if err := j.Prepare(t.Context(), testScope, ops[:1]); !errors.Is(err, flow.ErrConflict) {
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
	if err := j.Prepare(t.Context(), testScope, ops); err != nil {
		t.Fatal(err)
	}
	sum, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone})
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
	if err := j.Prepare(t.Context(), testScope, ops); err != nil {
		t.Fatal(err)
	}
	sum, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone})
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
	if err := j.Prepare(t.Context(), testScope, ops); err != nil {
		t.Fatal(err)
	}
	sum, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Uncertain != 2 {
		t.Fatalf("uncertain = %d, want 2", sum.Uncertain)
	}
	// A recovery well after the deadline must not resend an unsafe operation.
	clk.Advance(time.Hour)
	rsum, err := j.Recover(t.Context(), testScope, testNode, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatal(err)
	}
	if rsum.Pending != 0 || p.totalSends() != 2 {
		t.Fatalf("recovery %+v sent again: sends=%d", rsum, p.totalSends())
	}
}

func TestRecoverDispatching(t *testing.T) {
	db := newFakeDB()
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	const timeout, grace = 30 * time.Second, 5 * time.Second
	j := newJournal(t, db, p, clock.NewFake(t0), Config{AttemptTimeout: timeout, Grace: grace})
	ops := makeOps(3)
	deadline := t0.Add(timeout)
	db.seed(ops, deadline, t0)
	contract := Contract{Idempotency: IdempotencyProviderKey, KeyTTL: time.Hour}

	// Before the deadline plus grace, nothing is touched: the worker may be alive.
	db.dbClock.Set(deadline.Add(grace - time.Second))
	sum, err := j.Recover(t.Context(), testScope, testNode, contract)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Pending != 3 || p.totalSends() != 0 {
		t.Fatalf("early recovery acted: %+v sends=%d", sum, p.totalSends())
	}

	// After it, a key still inside its TTL is resent with the same key.
	db.dbClock.Set(deadline.Add(grace + time.Second))
	sum, err = j.Recover(t.Context(), testScope, testNode, contract)
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
	db.seed(ops, dispatched.Add(timeout), dispatched)
	clk.Set(t0)

	sum, err := j.Recover(t.Context(), testScope, testNode, Contract{Idempotency: IdempotencyProviderKey, KeyTTL: time.Hour})
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
	_, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone})
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
	if err := j.Prepare(t.Context(), testScope, ops); err != nil {
		t.Fatal(err)
	}
	// A timeout of one second leaves no room before the guard: nothing is sent.
	sum, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatal(err)
	}
	if p.totalSends() != 0 || sum.Pending != 1 {
		t.Fatalf("sends=%d summary=%+v", p.totalSends(), sum)
	}
}

// hangingProvider answers only when its context ends, as a stalled peer does.
type hangingProvider struct {
	mu          sync.Mutex
	hasDeadline bool
	remaining   time.Duration
}

func (p *hangingProvider) Send(ctx context.Context, _ *Attempt) (Reply, error) {
	p.mu.Lock()
	d, ok := ctx.Deadline()
	p.hasDeadline = ok
	if ok {
		p.remaining = time.Until(d)
	}
	p.mu.Unlock()
	select {
	case <-ctx.Done():
		return Reply{}, ctx.Err()
	case <-time.After(20 * time.Second): // a broken bound ends here, so the test fails instead of hanging
		return Reply{Outcome: StateSucceeded}, nil
	}
}

func (p *hangingProvider) Lookup(context.Context, *Attempt) (Reply, error) {
	return Reply{}, errors.New("no lookup in this test")
}

// BL-113: an item that waited for a send slot past the guard is not sent.
func TestSendThatWaitedPastTheGuardIsNotSent(t *testing.T) {
	db := newFakeDB()
	clk := clock.NewFake(t0)
	const timeout = 10 * time.Second
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		// The first send takes until the guard (the attempt timeout minus the send guard).
		clk.Advance(timeout - sendGuard)
		return Reply{Outcome: StateSucceeded}, nil
	})
	j := newJournal(t, db, p, clk, Config{AttemptTimeout: timeout, Concurrency: 1})
	ops := makeOps(3)
	if err := j.Prepare(t.Context(), testScope, ops); err != nil {
		t.Fatal(err)
	}
	sum, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone})
	if err != nil {
		t.Fatal(err)
	}
	if p.totalSends() != 1 || sum.Succeeded != 1 || sum.Pending != 2 {
		t.Fatalf("sends=%d summary=%+v, want one send and two items left in DISPATCHING", p.totalSends(), sum)
	}
	for _, op := range ops[1:] {
		if got := db.state(op.ID); got != StateDispatching {
			t.Fatalf("item %d is %q, want DISPATCHING", op.ItemIndex, got)
		}
	}
}

// BL-113: a send that hangs is cut at the attempt timeout, measured from the mark.
func TestHangingSendIsCutAtTheAttemptTimeout(t *testing.T) {
	db := newFakeDB()
	const timeout = 1500 * time.Millisecond
	p := &hangingProvider{}
	j := newJournal(t, db, p, clock.NewFake(t0), Config{AttemptTimeout: timeout})
	ops := makeOps(1)
	if err := j.Prepare(t.Context(), testScope, ops); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	sum, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Uncertain != 1 || db.state(ops[0].ID) != StateUncertain {
		t.Fatalf("summary=%+v state=%q, want the cut send UNCERTAIN", sum, db.state(ops[0].ID))
	}
	if elapsed > 10*time.Second {
		t.Fatalf("the send ran %v, past the attempt timeout %v", elapsed, timeout)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.hasDeadline || p.remaining > timeout || p.remaining < timeout-sendGuard {
		t.Fatalf("send context: deadline=%v remaining=%v, want a deadline of about %v", p.hasDeadline, p.remaining, timeout)
	}
}

// BL-113: the dispatch deadline is the database clock plus the attempt timeout, not the application clock.
func TestDispatchDeadlineComesFromTheDatabaseClock(t *testing.T) {
	db := newFakeDB()
	db.dbClock.Set(t0.Add(time.Hour))
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	j := newJournal(t, db, p, clock.NewFake(t0), Config{AttemptTimeout: 30 * time.Second})
	ops := makeOps(2)
	if err := j.Prepare(t.Context(), testScope, ops); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone}); err != nil {
		t.Fatal(err)
	}
	want := t0.Add(time.Hour + 30*time.Second)
	for _, op := range ops {
		if got := db.deadline(op.ID); !got.Equal(want) {
			t.Fatalf("item %d deadline = %v, want %v (database clock + attempt timeout)", op.ItemIndex, got, want)
		}
	}
}

// BL-113: recovery decides with the database clock: an application clock far past the deadline changes nothing.
func TestRecoveryReadsTheDatabaseClock(t *testing.T) {
	db := newFakeDB()
	clk := clock.NewFake(t0.Add(3 * time.Hour))
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	const timeout, grace = 30 * time.Second, 5 * time.Second
	j := newJournal(t, db, p, clk, Config{AttemptTimeout: timeout, Grace: grace})
	ops := makeOps(2)
	deadline := t0.Add(timeout)
	db.seed(ops, deadline, t0)
	contract := Contract{Idempotency: IdempotencyProviderKey, KeyTTL: time.Hour}

	db.dbClock.Set(deadline.Add(grace - time.Second))
	sum, err := j.Recover(t.Context(), testScope, testNode, contract)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Pending != 2 || p.totalSends() != 0 {
		t.Fatalf("recovery followed the application clock: %+v sends=%d", sum, p.totalSends())
	}

	db.dbClock.Set(deadline.Add(grace + time.Second))
	sum, err = j.Recover(t.Context(), testScope, testNode, contract)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Succeeded != 2 {
		t.Fatalf("recovery after the database deadline = %+v", sum)
	}
}

// BL-114: recovery pages by the full key, so rows that share an item index (one
// per retry epoch) are all settled, also across page boundaries.
func TestRecoverPagesThroughRetryEpochsSharingAnItem(t *testing.T) {
	db := newFakeDB()
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	j := newJournal(t, db, p, clock.NewFake(t0), Config{AttemptTimeout: 30 * time.Second, Grace: 5 * time.Second})
	j.page = 2
	var ops []store.ExternalOperation
	for _, row := range []struct{ item, epoch int64 }{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {2, 0}} {
		ops = append(ops, store.ExternalOperation{
			ID:                store.ExternalOperationID(id.Derive("op", strconv.FormatInt(row.item, 10), strconv.FormatInt(row.epoch, 10))),
			OrgID:             testOrg,
			ExecutionID:       store.ExecutionID(id.Derive("exec")),
			NodeExecutionID:   testNode,
			ItemIndex:         row.item,
			RetryEpoch:        row.epoch,
			CorrelationKey:    CorrelationKey("exec", "t1", "", "tok", row.item, row.epoch),
			ContractReference: "orcon.http@1",
			CreatedAt:         t0,
		})
	}
	db.seed(ops, t0.Add(30*time.Second), t0)
	db.dbClock.Set(t0.Add(40 * time.Second))
	sum, err := j.Recover(t.Context(), testScope, testNode, Contract{Idempotency: IdempotencyProviderKey, KeyTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Succeeded != len(ops) || p.totalSends() != len(ops) {
		t.Fatalf("recovery = %+v sends=%d, want all %d rows settled", sum, p.totalSends(), len(ops))
	}
	for _, op := range ops {
		if got := db.state(op.ID); got != StateSucceeded {
			t.Fatalf("item %d epoch %d is %q", op.ItemIndex, op.RetryEpoch, got)
		}
	}
}

// BL-115: a window that is not entirely PREPARED is rolled back as a whole: no
// item of it stays DISPATCHING, and nothing is sent.
func TestPartialMarkIsRolledBack(t *testing.T) {
	db := newFakeDB()
	p := newFakeProvider(t, db, func(int, *Attempt) (Reply, error) {
		return Reply{Outcome: StateSucceeded}, nil
	})
	j := newJournal(t, db, p, clock.NewFake(t0), Config{})
	ops := makeOps(3)
	// The third operation is never prepared, so the mark of the window moves two of three.
	if err := j.Prepare(t.Context(), testScope, ops[:2]); err != nil {
		t.Fatal(err)
	}
	_, err := j.Dispatch(t.Context(), testScope, ops, Contract{Idempotency: IdempotencyNone})
	if !errors.Is(err, ErrNotPrepared) {
		t.Fatalf("err = %v, want ErrNotPrepared", err)
	}
	for _, op := range ops[:2] {
		if got := db.state(op.ID); got != StatePrepared {
			t.Fatalf("item %d is %q after a refused mark, want PREPARED", op.ItemIndex, got)
		}
	}
	if p.totalSends() != 0 {
		t.Fatalf("sends = %d after a refused mark", p.totalSends())
	}
}

// scopeFor issues the scope of a test organization; tests may build scopes, production code may not.
func scopeFor(org identity.OrgID) orgscope.Org {
	o, err := issue.New(id.UUID(org))
	if err != nil {
		panic(err)
	}
	return o
}
