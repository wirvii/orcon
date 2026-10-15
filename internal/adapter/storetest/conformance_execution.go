// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The execution cases run against the ports of internal/execution/store. The
// database decides whether a lease is expired or a timer is due on PostgreSQL
// (orcon-db TY-5), so these cases use the real clock for what the database
// compares and a negative lease for "already expired".

// execFixture is a fixture with a flow version, an execution and its tools.
type execFixture struct {
	*fixture
	flow flow.Record
	ver  flow.VersionRecord
	// scopeA and scopeB are the scopes of the two organizations, which the store
	// methods take (ADR-0046).
	scopeA, scopeB orgscope.Org
}

func newExecFixture(t *testing.T, newStore Factory) *execFixture {
	t.Helper()
	f := newFixture(t, newStore)
	rec := f.flow(f.orgA, f.projA, "executions")
	return &execFixture{fixture: f, flow: rec, ver: f.firstVersion(rec), scopeA: scopeOf(f.orgA), scopeB: scopeOf(f.orgB)}
}

func (e *execFixture) tx(org identity.OrgID, fn func(ctx context.Context, tx store.Tx) error) error {
	return e.s.InTx(e.ctx(), scopeOf(org), fn)
}

func mustTx(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// execution creates an execution of the fixture's flow version in org A.
func (e *execFixture) execution() *store.Execution {
	e.t.Helper()
	return e.executionIn(e.orgA, e.projA, e.flow, e.ver)
}

func (e *execFixture) executionIn(org identity.OrgID, project identity.ProjectID, rec flow.Record, ver flow.VersionRecord) *store.Execution {
	e.t.Helper()
	ex := store.Execution{
		ID: next[store.ExecutionID](e.ids), OrgID: org, ProjectID: project, FlowID: rec.ID, FlowVersionID: ver.ID,
		Status: "QUEUED", Mode: "MANUAL", RunnableCount: 1, CreatedAt: base, UpdatedAt: base,
	}
	mustTx(e.t, e.tx(org, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &ex) }), "insert execution")
	return &ex
}

// token inserts a RUNNABLE token of the execution.
func (e *execFixture) token(ex *store.Execution, node string, priority int64, available time.Time) store.Token {
	e.t.Helper()
	tok := store.Token{
		ID: next[store.TokenID](e.ids), OrgID: ex.OrgID, ExecutionID: ex.ID, Status: "RUNNABLE", NodeID: node, ScopePath: "",
		ForkLineage: []byte(`[]`), FairnessKey: "flow/" + ex.FlowID.String(), EffectivePriority: priority,
		AvailableAt: available, CreatedAt: base, UpdatedAt: base,
	}
	mustTx(e.t, e.tx(ex.OrgID, func(ctx context.Context, tx store.Tx) error { return tx.InsertToken(ctx, &tok) }), "insert token")
	return tok
}

func worker(e *execFixture) execution.WorkerID { return next[execution.WorkerID](e.ids) }

// ExecutionCases lists the cases of the execution tables; Conformance runs them.
func executionCases() []struct {
	name string
	run  func(*testing.T, Factory)
} {
	return []struct {
		name string
		run  func(*testing.T, Factory)
	}{
		{"ExecutionsAndEvents", executionsCase},
		{"TransactionRollsBack", rollbackCase},
		{"ClaimIsExclusive", claimCase},
		{"ClaimOrderAndFilters", claimOrderCase},
		{"FencingStopsALostLease", fencingCase},
		{"RenewAndReap", reapCase},
		{"JoinArrivalIsAtomic", joinCase},
		{"TimersFireOnce", timersCase},
		{"JournalBatch", journalCase},
		{"OutboxLeases", outboxCase},
		{"WaitSecretLookup", waitCase},
		{"AttentionInbox", attentionCase},
		{"ExecutionIsolation", executionIsolationCase},
		{"EventsAreOrderedUnderConcurrency", eventsCase},
		{"ConcurrentStepsOnOneExecution", stepsCase},
		{"OneJoinInstancePerSplitInstance", splitJoinCase},
	}
}

func executionsCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	var created []store.Execution
	for range 5 {
		ex := store.Execution{
			ID: next[store.ExecutionID](e.ids), OrgID: e.orgA, ProjectID: e.projA, FlowID: e.flow.ID, FlowVersionID: e.ver.ID,
			Status: "QUEUED", Mode: "MANUAL", RunnableCount: 1, CreatedAt: base.Add(time.Duration(len(created)) * time.Second), UpdatedAt: base,
		}
		created = append(created, ex)
		mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &ex) }), "insert")
	}
	got, err := e.s.GetExecution(e.ctx(), e.scopeA, created[0].ID)
	if err != nil || got.Status != "QUEUED" || got.RootExecutionID != created[0].ID || got.FlowVersionID != e.ver.ID || !got.FlowSnapshotID.IsZero() ||
		got.RunnableCount != 1 || !got.ParentExecutionID.IsZero() {
		t.Fatalf("execution = %+v, %v", got, err)
	}
	// Newest first, in keyset pages, for each filter.
	page := func(filter store.ExecutionFilter) []store.Execution {
		var all []store.Execution
		var cursor store.Cursor
		for {
			rows, err := e.s.ListExecutions(e.ctx(), e.scopeA, filter, cursor, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				return all
			}
			if len(rows) > 2 {
				t.Fatalf("page of %d, limit 2", len(rows))
			}
			all = append(all, rows...)
			last := rows[len(rows)-1]
			cursor = store.Cursor{CreatedAt: last.CreatedAt, ID: [16]byte(last.ID)}
		}
	}
	for name, filter := range map[string]store.ExecutionFilter{
		"project": {ProjectID: e.projA}, "flow": {FlowID: e.flow.ID}, "status": {Status: "QUEUED"},
	} {
		rows := page(filter)
		if len(rows) != 5 {
			t.Fatalf("%s filter listed %d executions, want 5", name, len(rows))
		}
		for i := range rows {
			r := &rows[i]
			if r.ID != created[len(created)-1-i].ID {
				t.Fatalf("%s filter: row %d is %s, want %s (newest first)", name, i, r.ID, created[len(created)-1-i].ID)
			}
		}
	}
	if rows, err := e.s.ListExecutions(e.ctx(), e.scopeA, store.ExecutionFilter{Status: "RUNNING"}, store.Cursor{}, 10); err != nil || len(rows) != 0 {
		t.Fatalf("running executions = %d, %v", len(rows), err)
	}
	_, err = e.s.ListExecutions(e.ctx(), e.scopeA, store.ExecutionFilter{}, store.Cursor{}, 10)
	wantErr(t, "a list without a filter", err, flow.ErrInvalidRecord)
	_, err = e.s.ListExecutions(e.ctx(), e.scopeA, store.ExecutionFilter{Status: "QUEUED", FlowID: e.flow.ID}, store.Cursor{}, 10)
	wantErr(t, "a list with two filters", err, flow.ErrInvalidRecord)
	_, err = e.s.GetExecution(e.ctx(), e.scopeA, next[store.ExecutionID](e.ids))
	wantErr(t, "an unknown execution", err, flow.ErrNotFound)

	// Exactly one of version and snapshot.
	bad := created[0]
	bad.ID, bad.FlowSnapshotID = next[store.ExecutionID](e.ids), next[flow.FlowSnapshotID](e.ids)
	wantErr(t, "a version and a snapshot", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &bad) }), flow.ErrInvalidRecord)
	bad.ID, bad.FlowVersionID, bad.FlowSnapshotID = next[store.ExecutionID](e.ids), flow.FlowVersionID{}, flow.FlowSnapshotID{}
	wantErr(t, "neither a version nor a snapshot", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &bad) }), flow.ErrInvalidRecord)
	bad.ID, bad.Status, bad.FlowVersionID = next[store.ExecutionID](e.ids), "LOST", e.ver.ID
	wantErr(t, "an unknown status", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &bad) }), flow.ErrInvalidRecord)

	// A child execution keeps its root.
	child := created[0]
	child.ID, child.ParentExecutionID, child.RootExecutionID, child.CallDepth = next[store.ExecutionID](e.ids), created[0].ID, created[0].ID, 1
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &child) }), "insert child")
	if got, err = e.s.GetExecution(e.ctx(), e.scopeA, child.ID); err != nil || got.ParentExecutionID != created[0].ID || got.RootExecutionID != created[0].ID || got.CallDepth != 1 {
		t.Fatalf("child = %+v, %v", got, err)
	}

	// Events: numbered 1.. in the order appended, read back in pages.
	ex := created[0]
	for i := range 5 {
		var seq int64
		mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
			var err error
			seq, err = tx.AppendEvent(ctx, store.Event{ID: next[store.EventID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, OccurredAt: base, Type: fmt.Sprintf("test.event_%d", i),
				SchemaVersion: 1, Reference: []byte(`{}`), Data: []byte(`{"i":1}`), CreatedAt: base})
			return err
		}), "append event")
		if seq != int64(i+1) {
			t.Fatalf("event %d got sequence number %d", i, seq)
		}
	}
	events, err := e.s.ListExecutionEvents(e.ctx(), e.scopeA, ex.ID, 2, 10)
	if err != nil || len(events) != 3 || events[0].SequenceNumber != 3 || events[2].Type != "test.event_4" || string(events[0].Data) != `{"i":1}` || events[0].RootExecutionID != ex.ID {
		t.Fatalf("events after 2 = %+v, %v", events, err)
	}
	// Another execution starts at 1 again.
	other := created[1]
	var seq int64
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		var err error
		seq, err = tx.AppendEvent(ctx, store.Event{ID: next[store.EventID](e.ids), OrgID: e.orgA, ExecutionID: other.ID, OccurredAt: base, Type: "test.first", SchemaVersion: 1, Reference: []byte(`{}`), CreatedAt: base})
		return err
	}), "append to the second execution")
	if seq != 1 {
		t.Fatalf("first event of another execution got %d", seq)
	}
}

func rollbackCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := store.Execution{ID: next[store.ExecutionID](e.ids), OrgID: e.orgA, ProjectID: e.projA, FlowID: e.flow.ID, FlowVersionID: e.ver.ID,
		Status: "QUEUED", Mode: "MANUAL", CreatedAt: base, UpdatedAt: base}
	boom := errors.New("boom")
	err := e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if err := tx.InsertExecution(ctx, &ex); err != nil {
			return err
		}
		if _, err := tx.AppendEvent(ctx, store.Event{ID: next[store.EventID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, OccurredAt: base, Type: "x", SchemaVersion: 1, Reference: []byte(`{}`), CreatedAt: base}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error %v, want boom", err)
	}
	_, err = e.s.GetExecution(e.ctx(), e.scopeA, ex.ID)
	wantErr(t, "the rolled back execution", err, flow.ErrNotFound)
	// A failed statement inside the transaction also leaves nothing of it.
	err = e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if err := tx.InsertExecution(ctx, &ex); err != nil {
			return err
		}
		dup := ex
		return tx.InsertExecution(ctx, &dup)
	})
	wantErr(t, "a duplicate identity", err, flow.ErrConflict)
	_, err = e.s.GetExecution(e.ctx(), e.scopeA, ex.ID)
	wantErr(t, "the execution of a failed transaction", err, flow.ErrNotFound)
	// A record of another organization is refused before it is written.
	other := ex
	other.OrgID = e.orgB
	wantErr(t, "a record of another organization", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &other) }), flow.ErrInvalidRecord)
}

func realNow() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

func claimCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	recB := e.flow2(e.orgB, e.projB, "other")
	verB := e.firstVersionOf(recB)
	exA, exB := e.execution(), e.executionIn(e.orgB, e.projB, recB, verB)
	const perOrg = 40
	past := realNow().Add(-time.Hour)
	want := map[store.TokenID]identity.OrgID{}
	for i := range perOrg {
		a := e.token(exA, fmt.Sprintf("a%d", i), 0, past)
		b := e.token(exB, fmt.Sprintf("b%d", i), 0, past)
		want[a.ID], want[b.ID] = e.orgA, e.orgB
	}
	const workers = 8
	var mu sync.Mutex
	claimed := map[store.TokenID]int{}
	owners := map[store.TokenID]execution.WorkerID{}
	var wg sync.WaitGroup
	for range workers {
		w := worker(e)
		wg.Go(func() {
			for {
				rows, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, Limit: 7, LeaseTTL: time.Hour, Now: realNow()})
				if err != nil {
					t.Errorf("claim: %v", err)
					return
				}
				if len(rows) == 0 {
					return
				}
				mu.Lock()
				for i := range rows {
					r := &rows[i]
					claimed[r.TokenID]++
					owners[r.TokenID] = w
					if r.LeaseEpoch != 1 || r.OrgID != want[r.TokenID] {
						t.Errorf("claimed token %s: epoch %d org %s", r.TokenID, r.LeaseEpoch, r.OrgID)
					}
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if len(claimed) != 2*perOrg {
		t.Fatalf("%d distinct tokens claimed, want %d", len(claimed), 2*perOrg)
	}
	for tok, n := range claimed {
		if n != 1 {
			t.Fatalf("token %s was claimed %d times", tok, n)
		}
	}
	// The claim wrote the lease, across organizations, through the engine plane.
	sample := e.mustToken(e.orgA, firstKey(want, e.orgA))
	if sample.Status != "LEASED" || sample.LeaseOwner != owners[sample.ID] || sample.LeaseEpoch != 1 || sample.ClaimCount != 1 || sample.LeaseExpiresAt.Before(realNow()) {
		t.Fatalf("claimed token = %+v", sample)
	}
	rows, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: worker(e), Limit: 10, LeaseTTL: time.Hour, Now: realNow()})
	if err != nil || len(rows) != 0 {
		t.Fatalf("a claim after the queue emptied returned %d tokens, %v", len(rows), err)
	}
}

func firstKey(m map[store.TokenID]identity.OrgID, org identity.OrgID) store.TokenID {
	keys := make([]store.TokenID, 0, len(m))
	for k, v := range m {
		if v == org {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i][:], keys[j][:]) < 0 })
	return keys[0]
}

func (e *execFixture) mustToken(org identity.OrgID, tok store.TokenID) store.Token {
	e.t.Helper()
	got, err := e.s.GetToken(e.ctx(), scopeOf(org), tok)
	if err != nil {
		e.t.Fatalf("get token: %v", err)
	}
	return got
}

// flow2 creates a flow in another organization.
func (e *execFixture) flow2(org identity.OrgID, project identity.ProjectID, name string) flow.Record {
	e.t.Helper()
	return e.fixture.flow(org, project, name)
}

func (e *execFixture) firstVersionOf(rec flow.Record) flow.VersionRecord {
	e.t.Helper()
	return e.firstVersion(rec)
}

func claimOrderCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	now := realNow()
	low := e.token(ex, "low", 5, now.Add(-time.Hour))
	high := e.token(ex, "high", 1, now.Add(-time.Hour))
	older := e.token(ex, "older", 5, now.Add(-2*time.Hour))
	e.token(ex, "later", 0, now.Add(time.Hour)) // not available yet
	other := e.token(ex, "other-key", 0, now.Add(-time.Hour))
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return nil }), "noop")
	// A different fairness key: claimed only when asked for.
	otherKey := other
	otherKey.ID, otherKey.FairnessKey, otherKey.NodeID, otherKey.EffectivePriority = next[store.TokenID](e.ids), "flow/other", "keyed", 9
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertToken(ctx, &otherKey) }), "insert keyed token")

	got, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: worker(e), FairnessKeys: []string{"flow/other"}, Limit: 10, LeaseTTL: time.Hour, Now: now})
	if err != nil || len(got) != 1 || got[0].TokenID != otherKey.ID || got[0].FairnessKey != "flow/other" {
		t.Fatalf("claim by fairness key = %+v, %v", got, err)
	}
	// The claim chooses by priority, then availability, then identity; the order of the
	// returned slice is not defined, so each step compares the set it took. The future token stays out.
	ids := func(rows []store.Claimed) []string {
		out := make([]string, 0, len(rows))
		for i := range rows {
			out = append(out, rows[i].TokenID.String())
		}
		sort.Strings(out)
		return out
	}
	wantSet := func(toks ...store.Token) []string {
		out := make([]string, 0, len(toks))
		for i := range toks {
			out = append(out, toks[i].ID.String())
		}
		sort.Strings(out)
		return out
	}
	got, err = e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: worker(e), Limit: 2, LeaseTTL: time.Hour, Now: now})
	if err != nil || fmt.Sprint(ids(got)) != fmt.Sprint(wantSet(other, high)) {
		t.Fatalf("the first two claimed = %v, %v; want the tokens of priority 0 and 1", ids(got), err)
	}
	first := got
	got, err = e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: worker(e), Limit: 1, LeaseTTL: time.Hour, Now: now})
	if err != nil || fmt.Sprint(ids(got)) != fmt.Sprint(wantSet(older)) {
		t.Fatalf("the third claimed = %v, %v; want the older of the priority 5 tokens", ids(got), err)
	}
	got2, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: worker(e), Limit: 10, LeaseTTL: time.Hour, Now: now})
	if err != nil || fmt.Sprint(ids(got2)) != fmt.Sprint(wantSet(low)) {
		t.Fatalf("the rest claimed = %v, %v; want only the token available now (the future one stays out)", ids(got2), err)
	}
	got = first
	sort.Slice(got, func(i, j int) bool { return got[i].TokenID.String() < got[j].TokenID.String() })
	// The root fields travel with the claim.
	for i := range got {
		c := &got[i]
		if c.TokenID == other.ID && (c.NodeID != "other-key" || c.EdgeID != "" || string(c.ForkLineage) != `[]` || c.ExecutionID != ex.ID) {
			t.Fatalf("claimed fields = %+v", c)
		}
	}
}

func fencingCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "n", 0, realNow().Add(-time.Hour))
	w := worker(e)
	claim, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, Limit: 1, LeaseTTL: time.Hour, Now: realNow()})
	if err != nil || len(claim) != 1 {
		t.Fatalf("claim = %+v, %v", claim, err)
	}
	fence := store.Fence{OrgID: e.orgA, Token: tok.ID, Epoch: claim[0].LeaseEpoch, Owner: w}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.AssertFence(ctx, fence, base) }), "assert the held lease")

	for name, bad := range map[string]store.Fence{
		"a stale epoch": {OrgID: e.orgA, Token: tok.ID, Epoch: fence.Epoch + 1, Owner: w},
		"another owner": {OrgID: e.orgA, Token: tok.ID, Epoch: fence.Epoch, Owner: worker(e)},
		"another token": {OrgID: e.orgA, Token: next[store.TokenID](e.ids), Epoch: fence.Epoch, Owner: w},
	} {
		err := e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.AssertFence(ctx, bad, base) })
		wantErr(t, name, err, store.ErrFenced)
	}
	// A fenced step that fails takes everything else of its transaction with it. The claim started the execution, so
	// its event is already there.
	eventsBefore, _ := e.s.ListExecutionEvents(e.ctx(), e.scopeA, ex.ID, 0, 10)
	err = e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if _, err := tx.AppendEvent(ctx, store.Event{ID: next[store.EventID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, OccurredAt: base, Type: "lost", SchemaVersion: 1, Reference: []byte(`{}`), CreatedAt: base}); err != nil {
			return err
		}
		return tx.AssertFence(ctx, store.Fence{OrgID: e.orgA, Token: tok.ID, Epoch: fence.Epoch + 7, Owner: w}, base)
	})
	wantErr(t, "a step with a lost lease", err, store.ErrFenced)
	if evs, _ := e.s.ListExecutionEvents(e.ctx(), e.scopeA, ex.ID, 0, 10); len(evs) != len(eventsBefore) {
		t.Fatalf("a fenced transaction left %d events", len(evs)-len(eventsBefore))
	}
	// Moving the token ends the lease; the same fence then fails.
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.MoveToken(ctx, fence, "WAITING", base) }), "move the token")
	got := e.mustToken(e.orgA, tok.ID)
	if got.Status != "WAITING" || !got.LeaseOwner.IsZero() || !got.LeaseExpiresAt.IsZero() {
		t.Fatalf("moved token = %+v", got)
	}
	err = e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.AssertFence(ctx, fence, base) })
	wantErr(t, "the lease of a moved token", err, store.ErrFenced)
	err = e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.MoveToken(ctx, fence, "COMPLETED", base) })
	wantErr(t, "a second move", err, store.ErrFenced)
	// Only a claim leases.
	err = e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.MoveToken(ctx, fence, "LEASED", base) })
	wantErr(t, "a move to LEASED", err, flow.ErrInvalidRecord)
}

func reapCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	for i := range 3 {
		e.token(ex, fmt.Sprintf("n%d", i), 0, realNow().Add(-time.Hour))
	}
	w1, w2 := worker(e), worker(e)
	now := realNow()
	// A lease that is renewed stays; a lease that already expired is lost and cannot be renewed.
	long, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w1, Limit: 1, LeaseTTL: time.Hour, Now: now})
	if err != nil || len(long) != 1 {
		t.Fatalf("claim = %+v, %v", long, err)
	}
	short, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w1, Limit: 1, LeaseTTL: -time.Second, Now: now})
	if err != nil || len(short) != 1 {
		t.Fatalf("claim = %+v, %v", short, err)
	}
	renewed, err := e.s.RenewLeases(e.ctx(), w1, time.Hour, now)
	if err != nil || len(renewed) != 1 || renewed[0].TokenID != long[0].TokenID || renewed[0].LeaseEpoch != 1 || renewed[0].CancelRequested {
		t.Fatalf("renewed = %+v, %v (only the live lease may renew)", renewed, err)
	}
	if got, err := e.s.RenewLeases(e.ctx(), w2, time.Hour, now); err != nil || len(got) != 0 {
		t.Fatalf("another worker renewed %d leases, %v", len(got), err)
	}
	// The reaper returns the expired lease to RUNNABLE and counts the abandonment.
	reaped, err := e.s.ReleaseExpiredLeases(e.ctx(), 10, 3, now)
	if err != nil || len(reaped) != 1 || reaped[0].TokenID != short[0].TokenID || reaped[0].AbandonedCount != 1 || reaped[0].Poisoned || reaped[0].ExecutionID != ex.ID {
		t.Fatalf("reaped = %+v, %v", reaped, err)
	}
	got := e.mustToken(e.orgA, short[0].TokenID)
	if got.Status != "RUNNABLE" || !got.LeaseOwner.IsZero() || got.AbandonedCount != 1 || got.LeaseEpoch != 1 {
		t.Fatalf("reaped token = %+v", got)
	}
	if again, _ := e.s.ReleaseExpiredLeases(e.ctx(), 10, 3, now); len(again) != 0 {
		t.Fatalf("the reaper took %d more", len(again))
	}
	// The next claim raises the epoch; at the threshold the token is poisoned instead of retried.
	for round := 2; round <= 3; round++ {
		request := store.ClaimRequest{Worker: w2, FairnessKeys: []string{"flow/" + ex.FlowID.String()}, Limit: 10, LeaseTTL: -time.Second, Now: now}
		claim := e.s.ClaimTokens
		if round == 3 { // one abandonment short of the threshold: only the last chance claim takes it
			claim = e.s.ClaimLastChanceToken
		}
		c, err := claim(e.ctx(), request)
		if err != nil {
			t.Fatal(err)
		}
		var mine *store.Claimed
		for i := range c {
			if c[i].TokenID == short[0].TokenID {
				mine = &c[i]
			}
		}
		if mine == nil || mine.LeaseEpoch != int64(round) {
			t.Fatalf("round %d: claim = %+v", round, c)
		}
		r, err := e.s.ReleaseExpiredLeases(e.ctx(), 10, 3, now)
		if err != nil {
			t.Fatal(err)
		}
		var theirs *store.Reaped
		for i := range r {
			if r[i].TokenID == short[0].TokenID {
				theirs = &r[i]
			}
		}
		if theirs == nil || theirs.AbandonedCount != int64(round) || theirs.Poisoned != (round == 3) {
			t.Fatalf("round %d: reaped = %+v", round, r)
		}
		// Return the other expired claims so that only this token is under test.
	}
	got = e.mustToken(e.orgA, short[0].TokenID)
	if got.Status != "SUSPENDED" || got.SuspendReason != "POISON" || got.AbandonedCount != 3 {
		t.Fatalf("poisoned token = %+v", got)
	}
	// Graceful shutdown gives every lease of a worker back at once, and gives it back: no abandonment is counted.
	released, err := e.s.ReleaseWorkerLeases(e.ctx(), w1, 3, now)
	if err != nil || len(released) != 1 || released[0].TokenID != long[0].TokenID || released[0].AbandonedCount != 0 || released[0].Poisoned {
		t.Fatalf("released = %+v, %v", released, err)
	}
	if got := e.mustToken(e.orgA, long[0].TokenID); got.Status != "RUNNABLE" || !got.LeaseOwner.IsZero() || got.AbandonedCount != 0 {
		t.Fatalf("released token = %+v", got)
	}
}

func joinCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	key := store.JoinKey{OrgID: e.orgA, ExecutionID: ex.ID, NodeID: "join", ScopePath: "", RoundNumber: 1}
	const edges = 6
	toks := make([]store.Token, edges)
	for i := range toks {
		toks[i] = e.token(ex, fmt.Sprintf("branch%d", i), 0, realNow().Add(time.Hour))
	}
	continuation := e.token(ex, "after-join", 0, realNow().Add(time.Hour))
	var fired atomic.Int64
	var winner atomic.Pointer[store.TokenID]
	var wg sync.WaitGroup
	for i := range edges {
		wg.Go(func() {
			err := e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
				progress, err := tx.RecordJoinArrival(ctx, store.JoinArrival{
					Key: key, InstanceID: next[store.JoinInstanceID](e.ids), ArrivalID: next[store.JoinArrivalID](e.ids), EdgeIndex: int64(i), TokenID: toks[i].ID,
					IsLive: i%2 == 0, ArrivedAt: base,
				}, base)
				if err != nil {
					return err
				}
				if !progress.Recorded {
					return fmt.Errorf("edge %d was not recorded", i)
				}
				if progress.Arrivals < edges {
					return nil
				}
				won, err := tx.MarkJoinFired(ctx, key, continuation.ID, base)
				if err != nil {
					return err
				}
				if won {
					fired.Add(1)
					id := toks[i].ID
					winner.Store(&id)
				}
				return nil
			})
			if err != nil {
				t.Errorf("edge %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	// The transactions overlap; whoever saw the last arrival fired, and exactly one did.
	if fired.Load() != 1 {
		t.Fatalf("%d callers fired the join, want exactly 1", fired.Load())
	}
	ji, err := e.s.GetJoinInstance(e.ctx(), key)
	if err != nil || ji.Status != "FIRED" || ji.ContinuationTokenID != continuation.ID {
		t.Fatalf("join = %+v, %v", ji, err)
	}
	// A repeated arrival changes nothing and reports itself; the count and the live count stay.
	var progress store.JoinProgress
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		var err error
		progress, err = tx.RecordJoinArrival(ctx, store.JoinArrival{Key: key, InstanceID: next[store.JoinInstanceID](e.ids), ArrivalID: next[store.JoinArrivalID](e.ids), EdgeIndex: 0, TokenID: toks[0].ID, IsLive: true, ArrivedAt: base}, base)
		return err
	}), "repeat an arrival")
	if progress.Recorded || progress.Arrivals != edges || progress.Live != edges/2 {
		t.Fatalf("progress after a repeat = %+v, want %d arrivals and %d live", progress, edges, edges/2)
	}
	// A second round of the same join is its own instance.
	round2 := key
	round2.RoundNumber = 2
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		p, err := tx.RecordJoinArrival(ctx, store.JoinArrival{Key: round2, InstanceID: next[store.JoinInstanceID](e.ids), ArrivalID: next[store.JoinArrivalID](e.ids), EdgeIndex: 0, TokenID: toks[0].ID, IsLive: true, ArrivedAt: base}, base)
		if err != nil || !p.Recorded || p.Arrivals != 1 {
			return fmt.Errorf("round 2 = %+v: %w", p, err)
		}
		won, err := tx.MarkJoinFired(ctx, round2, continuation.ID, base)
		if err != nil || !won {
			return fmt.Errorf("round 2 fired = %v: %w", won, err)
		}
		again, err := tx.MarkJoinFired(ctx, round2, continuation.ID, base)
		if err != nil || again {
			return fmt.Errorf("round 2 fired twice = %v: %w", again, err)
		}
		return nil
	}), "second round")
}

func timersCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "waiting", 0, realNow().Add(time.Hour))
	now := realNow()
	const due, future = 60, 15
	due0 := make([]store.TimerID, 0, due)
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		for i := range due {
			tm := store.Timer{ID: next[store.TimerID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, Kind: "WAIT", DueAt: now.Add(-time.Duration(i+1) * time.Minute), CreatedAt: base}
			due0 = append(due0, tm.ID)
			if err := tx.InsertTimer(ctx, tm); err != nil {
				return err
			}
		}
		for range future {
			if err := tx.InsertTimer(ctx, store.Timer{ID: next[store.TimerID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, Kind: "NODE_TIMEOUT", DueAt: now.Add(time.Hour), CreatedAt: base}); err != nil {
				return err
			}
		}
		return nil
	}), "insert timers")
	var mu sync.Mutex
	fired := map[store.TimerID]int{}
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			for {
				rows, err := e.s.FireDueTimers(e.ctx(), 7, realNow())
				if err != nil {
					t.Errorf("fire: %v", err)
					return
				}
				if len(rows) == 0 {
					return
				}
				mu.Lock()
				for _, r := range rows {
					fired[r.TimerID]++
					if r.OrgID != e.orgA || r.TokenID != tok.ID || r.Kind != "WAIT" {
						t.Errorf("fired timer = %+v", r)
					}
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if len(fired) != due {
		t.Fatalf("%d timers fired, want %d", len(fired), due)
	}
	for id, n := range fired {
		if n != 1 {
			t.Fatalf("timer %s fired %d times", id, n)
		}
	}
	got, err := e.s.GetTimer(e.ctx(), e.scopeA, due0[0])
	if err != nil || got.Status != "FIRED" {
		t.Fatalf("fired timer = %+v, %v", got, err)
	}
	if rows, err := e.s.FireDueTimers(e.ctx(), 100, realNow()); err != nil || len(rows) != 0 {
		t.Fatalf("future timers fired: %d, %v", len(rows), err)
	}
}

func journalCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "effect", 0, realNow().Add(-time.Hour))
	w := worker(e)
	claim, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, Limit: 1, LeaseTTL: time.Hour, Now: realNow()})
	if err != nil || len(claim) != 1 {
		t.Fatalf("claim = %+v, %v", claim, err)
	}
	fence := store.Fence{OrgID: e.orgA, Token: tok.ID, Epoch: claim[0].LeaseEpoch, Owner: w}
	ne := store.NodeExecution{ID: next[store.NodeExecutionID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: "effect", Status: "RUNNING", StartedAt: base, CreatedAt: base, UpdatedAt: base}
	const items = 25
	ops := make([]store.ExternalOperation, items)
	ids := make([]store.ExternalOperationID, items)
	for i := range ops {
		ids[i] = next[store.ExternalOperationID](e.ids)
		ops[i] = store.ExternalOperation{ID: ids[i], OrgID: e.orgA, ExecutionID: ex.ID, NodeExecutionID: ne.ID, ItemIndex: int64(i),
			CorrelationKey: fmt.Sprintf("key-%02d", i), ContractReference: "contract/1", BatchKey: "batch-1", CreatedAt: base}
	}
	// Intent first: the whole batch in one transaction with the node execution.
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if err := tx.AssertFence(ctx, fence, base); err != nil {
			return err
		}
		if err := tx.InsertNodeExecution(ctx, ne); err != nil {
			return err
		}
		return tx.AppendJournal(ctx, ops)
	}), "journal intent")
	listed, err := e.s.ListExternalOperations(e.ctx(), e.scopeA, ne.ID, 0, 100)
	if err != nil || len(listed) != items {
		t.Fatalf("journal has %d entries, %v", len(listed), err)
	}
	for i := range listed {
		o := &listed[i]
		if o.ItemIndex != int64(i) || o.State != "PREPARED" || o.CorrelationKey != fmt.Sprintf("key-%02d", i) || o.BatchKey != "batch-1" || !o.DispatchDeadlineAt.IsZero() {
			t.Fatalf("entry %d = %+v", i, o)
		}
	}
	if rest, err := e.s.ListExternalOperations(e.ctx(), e.scopeA, ne.ID, 20, 100); err != nil || len(rest) != 5 || rest[0].ItemIndex != 20 {
		t.Fatalf("journal from item 20 = %d entries, %v", len(rest), err)
	}
	// A repeated correlation key rejects the whole batch.
	dup := []store.ExternalOperation{
		{ID: next[store.ExternalOperationID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, NodeExecutionID: ne.ID, ItemIndex: 100, CorrelationKey: "fresh", ContractReference: "c", CreatedAt: base},
		{ID: next[store.ExternalOperationID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, NodeExecutionID: ne.ID, ItemIndex: 101, CorrelationKey: "key-00", ContractReference: "c", CreatedAt: base},
	}
	wantErr(t, "a repeated correlation key", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.AppendJournal(ctx, dup) }), flow.ErrConflict)
	if all, _ := e.s.ListExternalOperations(e.ctx(), e.scopeA, ne.ID, 100, 100); len(all) != 0 {
		t.Fatalf("a rejected batch left %d entries", len(all))
	}
	// DISPATCHING is marked under the fence, before any send; it moves only PREPARED entries once.
	deadline := realNow().Add(time.Minute).Truncate(time.Microsecond)
	var moved int64
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if err := tx.AssertFence(ctx, fence, base); err != nil {
			return err
		}
		var err error
		moved, err = tx.MarkDispatching(ctx, ids[:10], deadline, base)
		return err
	}), "mark dispatching")
	if moved != 10 {
		t.Fatalf("%d entries moved to DISPATCHING, want 10", moved)
	}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		var err error
		moved, err = tx.MarkDispatching(ctx, ids[5:15], deadline, base)
		return err
	}), "mark dispatching again")
	if moved != 5 {
		t.Fatalf("%d entries moved on the second call, want 5 (only PREPARED ones)", moved)
	}
	gotOp, err := e.s.GetExternalOperation(e.ctx(), e.scopeA, ids[0])
	if err != nil || gotOp.State != "DISPATCHING" || !gotOp.DispatchDeadlineAt.Equal(deadline) || gotOp.FirstDispatchedAt.IsZero() {
		t.Fatalf("dispatching entry = %+v, %v", gotOp, err)
	}
	// A lost lease stops the dispatch mark.
	stale := fence
	stale.Epoch++
	err = e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if err := tx.AssertFence(ctx, stale, base); err != nil {
			return err
		}
		_, err := tx.MarkDispatching(ctx, ids[20:], deadline, base)
		return err
	})
	wantErr(t, "a dispatch mark with a lost lease", err, store.ErrFenced)
	if g, _ := e.s.GetExternalOperation(e.ctx(), e.scopeA, ids[20]); g.State != "PREPARED" {
		t.Fatalf("entry 20 is %s", g.State)
	}
	// Outcomes: only from the expected state.
	var ok, wrong bool
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		var err error
		if ok, err = tx.ResolveOperation(ctx, ids[0], "DISPATCHING", "SUCCEEDED", "response/1", base); err != nil {
			return err
		}
		wrong, err = tx.ResolveOperation(ctx, ids[20], "DISPATCHING", "SUCCEEDED", "", base)
		return err
	}), "resolve")
	if !ok || wrong {
		t.Fatalf("resolve = %v, %v; want true, false", ok, wrong)
	}
	if g, _ := e.s.GetExternalOperation(e.ctx(), e.scopeA, ids[0]); g.State != "SUCCEEDED" || g.ResponseReference != "response/1" {
		t.Fatalf("resolved entry = %+v", g)
	}
	// An unknown state is rejected by the database.
	wantErr(t, "an unknown state", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		_, err := tx.ResolveOperation(ctx, ids[1], "DISPATCHING", "MAYBE", "", base)
		return err
	}), flow.ErrInvalidRecord)
}

func outboxCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	const rows = 24
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		for i := range rows {
			if err := tx.InsertOutbox(ctx, store.OutboxRow{ID: next[store.OutboxID](e.ids), OrgID: e.orgA, Topic: fmt.Sprintf("topic-%d", i), Payload: []byte(`{}`), CreatedAt: base.Add(time.Duration(i) * time.Millisecond)}); err != nil {
				return err
			}
		}
		return nil
	}), "insert outbox rows")
	var mu sync.Mutex
	claimed := map[store.OutboxID]execution.WorkerID{}
	var wg sync.WaitGroup
	for range 4 {
		w := worker(e)
		wg.Go(func() {
			for {
				got, err := e.s.ClaimOutbox(e.ctx(), w, 3, time.Hour, realNow())
				if err != nil {
					t.Errorf("claim outbox: %v", err)
					return
				}
				if len(got) == 0 {
					return
				}
				mu.Lock()
				for _, c := range got {
					if _, dup := claimed[c.OutboxID]; dup {
						t.Errorf("outbox row %s claimed twice", c.OutboxID)
					}
					claimed[c.OutboxID] = w
					if c.AttemptCount != 1 || c.OrgID != e.orgA {
						t.Errorf("claimed row = %+v", c)
					}
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if len(claimed) != rows {
		t.Fatalf("%d rows claimed, want %d", len(claimed), rows)
	}
	var someone store.OutboxID
	var holder execution.WorkerID
	for id, w := range claimed {
		someone, holder = id, w
		break
	}
	var byOther, byHolder bool
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		var err error
		if byOther, err = tx.MarkOutboxDelivered(ctx, someone, worker(e), base); err != nil {
			return err
		}
		byHolder, err = tx.MarkOutboxDelivered(ctx, someone, holder, base)
		return err
	}), "deliver")
	if byOther || !byHolder {
		t.Fatalf("delivered by another = %v, by the holder = %v", byOther, byHolder)
	}
	// A lease that already expired frees a row again, with the attempt counted; a delivered row never returns.
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		return tx.InsertOutbox(ctx, store.OutboxRow{ID: next[store.OutboxID](e.ids), OrgID: e.orgA, Topic: "retry", Payload: []byte(`{}`), CreatedAt: base.Add(time.Hour)})
	}), "insert a row to retry")
	first, err := e.s.ClaimOutbox(e.ctx(), worker(e), 5, -time.Second, realNow())
	if err != nil || len(first) != 1 || first[0].Topic != "retry" || first[0].AttemptCount != 1 {
		t.Fatalf("first claim = %+v, %v", first, err)
	}
	second, err := e.s.ClaimOutbox(e.ctx(), worker(e), 5, time.Hour, realNow())
	if err != nil || len(second) != 1 || second[0].OutboxID != first[0].OutboxID || second[0].AttemptCount != 2 {
		t.Fatalf("claim after the lease expired = %+v, %v", second, err)
	}
	if third, err := e.s.ClaimOutbox(e.ctx(), worker(e), 5, time.Hour, realNow()); err != nil || len(third) != 0 {
		t.Fatalf("a delivered or leased row was claimed: %d, %v", len(third), err)
	}
}

func waitCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "wait", 0, realNow().Add(time.Hour))
	ne := store.NodeExecution{ID: next[store.NodeExecutionID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: "wait", Status: "WAITING", StartedAt: base, CreatedAt: base, UpdatedAt: base}
	hash := bytes.Repeat([]byte{0xab}, 32)
	signal := store.Wait{ID: next[store.WaitID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeExecutionID: ne.ID, Kind: "SIGNAL", SecretHash: hash, ExpiresAt: base.Add(time.Hour), CreatedAt: base}
	timer := signal
	timer.ID, timer.Kind, timer.SecretHash = next[store.WaitID](e.ids), "TIMER", nil
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if err := tx.InsertNodeExecution(ctx, ne); err != nil {
			return err
		}
		if err := tx.InsertWait(ctx, signal); err != nil {
			return err
		}
		return tx.InsertWait(ctx, timer)
	}), "insert waits")
	// The lookup runs before the organization is known.
	got, err := e.s.ResolveWaitSecret(e.ctx(), signal.ID)
	if err != nil || got.OrgID != e.orgA || got.WaitID != signal.ID || !bytes.Equal(got.SecretHash, hash) || got.Status != "OPEN" || !got.ExpiresAt.Equal(base.Add(time.Hour).Truncate(time.Microsecond)) {
		t.Fatalf("resolved wait = %+v, %v", got, err)
	}
	_, err = e.s.ResolveWaitSecret(e.ctx(), timer.ID)
	wantErr(t, "a wait that is not a signal", err, flow.ErrNotFound)
	_, err = e.s.ResolveWaitSecret(e.ctx(), next[store.WaitID](e.ids))
	wantErr(t, "an unknown wait", err, flow.ErrNotFound)
	// A bad secret hash length is rejected by the database.
	bad := signal
	bad.ID, bad.SecretHash = next[store.WaitID](e.ids), []byte("short")
	wantErr(t, "a secret hash of the wrong length", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertWait(ctx, bad) }), flow.ErrInvalidRecord)
}

func attentionCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	const n = 7
	ids := make([]store.AttentionRequestID, n)
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		for i := range n {
			ids[i] = next[store.AttentionRequestID](e.ids)
			if err := tx.InsertAttentionRequest(ctx, &store.AttentionRequest{
				ID: ids[i], OrgID: e.orgA, ExecutionID: ex.ID, ProjectID: e.projA, FlowID: e.flow.ID, NodeID: "n", Cause: "HUMAN_TASK",
				CauseReference: fmt.Sprintf("task-%d", i), GroupKey: "g", ItemCount: 1, Summary: "needs a person",
				CreatedAt: base.Add(time.Duration(i) * time.Second),
			}); err != nil {
				return err
			}
		}
		return nil
	}), "insert requests")
	var all []store.AttentionRequest
	var cursor store.Cursor
	for {
		page, err := e.s.ListAttentionRequests(e.ctx(), e.scopeA, "OPEN", cursor, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 3 {
			t.Fatalf("page of %d, limit 3", len(page))
		}
		all = append(all, page...)
		last := page[len(page)-1]
		cursor = store.Cursor{CreatedAt: last.CreatedAt, ID: [16]byte(last.ID)}
	}
	if len(all) != n {
		t.Fatalf("inbox lists %d requests, want %d", len(all), n)
	}
	for i := range all {
		r := &all[i]
		if r.ID != ids[i] || r.Status != "OPEN" || r.NotifyClass != "NOTIFY" || r.Cause != "HUMAN_TASK" || r.ExecutionID != ex.ID {
			t.Fatalf("request %d = %+v (oldest first)", i, r)
		}
	}
	if got, err := e.s.ListAttentionRequests(e.ctx(), e.scopeA, "RESOLVED", store.Cursor{}, 10); err != nil || len(got) != 0 {
		t.Fatalf("resolved requests = %d, %v", len(got), err)
	}
	if got, err := e.s.ListAttentionRequests(e.ctx(), e.scopeB, "OPEN", store.Cursor{}, 10); err != nil || len(got) != 0 {
		t.Fatalf("organization B sees %d requests, %v", len(got), err)
	}
	// One request per cause reference, per organization.
	// The class, the status and the level are stored as given, not forced.
	silent := store.AttentionRequest{ID: next[store.AttentionRequestID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, ProjectID: e.projA, FlowID: e.flow.ID, NodeID: "n",
		Cause: "END_USER_WAIT", CauseReference: "wait-1", NotifyClass: "SILENT", Status: "RESOLVED", Level: 2, GroupKey: "g", Summary: "done", CreatedAt: base}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertAttentionRequest(ctx, &silent) }), "insert a silent request")
	resolved, err := e.s.ListAttentionRequests(e.ctx(), e.scopeA, "RESOLVED", store.Cursor{}, 10)
	if err != nil || len(resolved) != 1 || resolved[0].ID != silent.ID || resolved[0].NotifyClass != "SILENT" || resolved[0].Level != 2 || resolved[0].Status != "RESOLVED" {
		t.Fatalf("resolved requests = %+v, %v", resolved, err)
	}
	if open, err := e.s.ListAttentionRequests(e.ctx(), e.scopeA, "OPEN", store.Cursor{}, 100); err != nil || len(open) != n {
		t.Fatalf("a resolved request appeared in the open inbox: %d, %v", len(open), err)
	}
	dup := store.AttentionRequest{ID: next[store.AttentionRequestID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, ProjectID: e.projA, FlowID: e.flow.ID, NodeID: "n",
		Cause: "HUMAN_TASK", CauseReference: "task-0", GroupKey: "g", Summary: "again", CreatedAt: base}
	wantErr(t, "a repeated cause reference", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertAttentionRequest(ctx, &dup) }), flow.ErrConflict)
	dup.ID, dup.CauseReference, dup.Cause = next[store.AttentionRequestID](e.ids), "task-9", "SOMETHING_ELSE"
	wantErr(t, "an unknown cause", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertAttentionRequest(ctx, &dup) }), flow.ErrInvalidRecord)
}

func executionIsolationCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "secret", 0, realNow().Add(time.Hour))
	ctx := e.ctx()
	if _, err := e.s.GetExecution(ctx, e.scopeB, ex.ID); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("execution of another organization: %v", err)
	}
	if _, err := e.s.GetToken(ctx, e.scopeB, tok.ID); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("token of another organization: %v", err)
	}
	if got, err := e.s.ListExecutions(ctx, e.scopeB, store.ExecutionFilter{FlowID: e.flow.ID}, store.Cursor{}, 10); err != nil || len(got) != 0 {
		t.Errorf("executions of another organization: %d, %v", len(got), err)
	}
	if got, err := e.s.ListExecutionEvents(ctx, e.scopeB, ex.ID, 0, 10); err != nil || len(got) != 0 {
		t.Errorf("events of another organization: %d, %v", len(got), err)
	}
	// Organization B cannot write into A's execution: the composite foreign key refuses it.
	foreign := store.Token{ID: next[store.TokenID](e.ids), OrgID: e.orgB, ExecutionID: ex.ID, Status: "RUNNABLE", NodeID: "x", ForkLineage: []byte(`[]`),
		FairnessKey: "k", AvailableAt: base, CreatedAt: base, UpdatedAt: base}
	wantErr(t, "a token of B in an execution of A", e.tx(e.orgB, func(ctx context.Context, tx store.Tx) error { return tx.InsertToken(ctx, &foreign) }), flow.ErrReference)
	// The fence is per organization too.
	err := e.tx(e.orgB, func(ctx context.Context, tx store.Tx) error {
		return tx.AssertFence(ctx, store.Fence{OrgID: e.orgB, Token: tok.ID, Epoch: 0, Owner: worker(e)}, base)
	})
	wantErr(t, "a fence on the token of another organization", err, store.ErrFenced)
}

func eventsCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	const writers, each = 8, 6
	var wg sync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			for i := range each {
				err := e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
					_, err := tx.AppendEvent(ctx, store.Event{ID: next[store.EventID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, OccurredAt: base,
						Type: fmt.Sprintf("test.w%d_%d", w, i), SchemaVersion: 1, Reference: []byte(`{}`), CreatedAt: base})
					return err
				})
				if err != nil {
					t.Errorf("writer %d: %v", w, err)
					return
				}
			}
		})
	}
	wg.Wait()
	events, err := e.s.ListExecutionEvents(e.ctx(), e.scopeA, ex.ID, 0, 100)
	if err != nil || len(events) != writers*each {
		t.Fatalf("%d events, %v; want %d", len(events), err, writers*each)
	}
	perWriter := map[int]int{}
	for i := range events {
		ev := &events[i]
		if ev.SequenceNumber != int64(i+1) {
			t.Fatalf("event %d has sequence number %d: numbers must be contiguous from 1", i, ev.SequenceNumber)
		}
		// Each writer's events keep their own order.
		var w, n int
		if _, err := fmt.Sscanf(ev.Type, "test.w%d_%d", &w, &n); err != nil {
			t.Fatal(err)
		}
		if n != perWriter[w] {
			t.Fatalf("writer %d: event %d came at position %d", w, n, perWriter[w])
		}
		perWriter[w]++
	}
}

// stepsCase runs many engine steps on one execution at once. Each step inserts a
// child row (which takes a shared lock on the execution) and then appends an
// event (which holds the execution until commit): the two locks must not
// deadlock, nothing may fail, and the sequence numbers stay contiguous.
func stepsCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	const workers, steps = 10, 8
	var failed atomic.Int64
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for i := range steps {
				err := e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
					tok := store.Token{ID: next[store.TokenID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, Status: "RUNNABLE", NodeID: fmt.Sprintf("w%d-%d", w, i),
						ForkLineage: []byte(`[]`), FairnessKey: "k", AvailableAt: base, CreatedAt: base, UpdatedAt: base}
					if err := tx.InsertToken(ctx, &tok); err != nil {
						return err
					}
					ne := store.NodeExecution{ID: next[store.NodeExecutionID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: tok.NodeID,
						Status: "RUNNING", StartedAt: base, CreatedAt: base, UpdatedAt: base}
					if err := tx.InsertNodeExecution(ctx, ne); err != nil {
						return err
					}
					_, err := tx.AppendEvent(ctx, store.Event{ID: next[store.EventID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, OccurredAt: base,
						Type: "node.started", SchemaVersion: 1, Reference: []byte(`{}`), CreatedAt: base})
					return err
				})
				if err != nil {
					failed.Add(1)
					t.Errorf("step %d of worker %d: %v", i, w, err)
				}
			}
		})
	}
	wg.Wait()
	if failed.Load() != 0 {
		t.Fatalf("%d of %d steps failed", failed.Load(), workers*steps)
	}
	events, err := e.s.ListExecutionEvents(e.ctx(), e.scopeA, ex.ID, 0, 1000)
	if err != nil || len(events) != workers*steps {
		t.Fatalf("%d events, %v; want %d", len(events), err, workers*steps)
	}
	for i := range events {
		if events[i].SequenceNumber != int64(i+1) {
			t.Fatalf("event %d has sequence number %d: numbers must be contiguous from 1", i, events[i].SequenceNumber)
		}
	}
}

// splitJoinCase: a split instance has one join instance per join node and scope,
// whatever the round (ADR-0015 section 3 as amended by SPEC-001 Amendment 9).
func splitJoinCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "split", 0, realNow().Add(time.Hour))
	split := store.NodeExecution{ID: next[store.NodeExecutionID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: "split",
		Status: "SUCCEEDED", StartedAt: base, CreatedAt: base, UpdatedAt: base}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertNodeExecution(ctx, split) }), "insert the split")
	arrive := func(round int64, splitID store.NodeExecutionID) error {
		return e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
			_, err := tx.RecordJoinArrival(ctx, store.JoinArrival{
				Key:        store.JoinKey{OrgID: e.orgA, ExecutionID: ex.ID, NodeID: "join", ScopePath: "", RoundNumber: round},
				InstanceID: next[store.JoinInstanceID](e.ids), ArrivalID: next[store.JoinArrivalID](e.ids), EdgeIndex: 0, TokenID: tok.ID,
				IsLive: true, SplitNodeExecutionID: splitID, ArrivedAt: base,
			}, base)
			return err
		})
	}
	mustTx(t, arrive(1, split.ID), "round 1 of the split instance")
	wantErr(t, "a second join instance for the same split instance", arrive(2, split.ID), flow.ErrConflict)
	// Rounds without a split instance (ALL, ACTIVATED, FIRST with CANCEL) are not limited by it.
	mustTx(t, arrive(3, store.NodeExecutionID{}), "round 3 without a split instance")
	mustTx(t, arrive(4, store.NodeExecutionID{}), "round 4 without a split instance")
}
