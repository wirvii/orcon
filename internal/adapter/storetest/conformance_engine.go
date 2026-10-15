// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
)

// The cases below are the contract of the ports the durable engine commits
// through (SPEC-001 C4a): the engine state of an execution, the moves of a
// held token, the batches, the recorded environments, the attempts, and the
// recovery that moves tokens when a lease is lost.

func engineCases() []struct {
	name string
	run  func(*testing.T, Factory)
} {
	return []struct {
		name string
		run  func(*testing.T, Factory)
	}{
		{"EngineStateAndSnapshot", engineStateCase},
		{"HeldTokenMoves", tokenMovesCase},
		{"Payloads", payloadsCase},
		{"RecordedEnvironments", environmentsCase},
		{"ClaimStartsTheExecution", claimStartsCase},
		{"AbandonmentCountsAndPoisons", abandonmentCase},
		{"TimerWakesItsTokenAtomically", timerWakeCase},
	}
}

func engineStateCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	live := []store.Token{e.token(ex, "a", 0, base), e.token(ex, "b", 0, base), e.token(ex, "c", 0, base)}

	var state store.EngineState
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		var err error
		state, err = tx.LockExecution(ctx, e.orgA, ex.ID)
		return err
	}), "lock the execution")
	if state.Status != "QUEUED" || state.Revision != 0 || state.Document != nil {
		t.Fatalf("new execution state = %+v", state)
	}
	save := func(update store.EngineStateUpdate) error {
		update.OrgID, update.Execution, update.Now = e.orgA, ex.ID, base
		return e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.SaveEngineState(ctx, update) })
	}
	mustTx(t, save(store.EngineStateUpdate{ExpectedRevision: 0, Status: "RUNNING", Document: []byte(`{"v":1}`), StartedAt: base}), "first commit")
	// A step built on the old revision cannot overwrite the newer state.
	wantErr(t, "a commit at a stale revision", save(store.EngineStateUpdate{ExpectedRevision: 0, Status: "SUCCEEDED", Document: []byte(`{"v":2}`)}), flow.ErrConflict)
	mustTx(t, save(store.EngineStateUpdate{ExpectedRevision: 1, Status: "RUNNING"}), "commit without a document")
	snap, err := e.s.LoadSnapshot(e.ctx(), e.orgA, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Status != "RUNNING" || snap.Revision != 2 || string(snap.Document) != `{"v":1}` || len(snap.Tokens) != 3 {
		t.Fatalf("snapshot = %+v, want RUNNING at revision 2 with the first document and 3 tokens", snap)
	}
	got, err := e.s.GetExecution(e.ctx(), e.scopeA, ex.ID)
	if err != nil || got.Status != "RUNNING" || got.StartedAt.IsZero() {
		t.Fatalf("execution = %+v, %v; want RUNNING with a start instant", got, err)
	}
	// Only the live tokens are read.
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		_, err := tx.CancelToken(ctx, e.orgA, live[0].ID, base)
		return err
	}), "cancel a token")
	if snap, err = e.s.LoadSnapshot(e.ctx(), e.orgA, ex.ID); err != nil || len(snap.Tokens) != 2 {
		t.Fatalf("snapshot after a cancel = %d tokens, %v; want 2", len(snap.Tokens), err)
	}
	// The snapshot pages: more live tokens than a page.
	big := e.execution()
	const many = 1203
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		for i := range many {
			tok := store.Token{
				ID: next[store.TokenID](e.ids), OrgID: e.orgA, ExecutionID: big.ID, Status: "RUNNABLE", NodeID: fmt.Sprintf("n%d", i),
				ForkLineage: []byte(`{}`), FairnessKey: "k", AvailableAt: base, CreatedAt: base, UpdatedAt: base,
			}
			if err := tx.InsertToken(ctx, &tok); err != nil {
				return err
			}
		}
		return nil
	}), "insert many tokens")
	if snap, err = e.s.LoadSnapshot(e.ctx(), e.orgA, big.ID); err != nil || len(snap.Tokens) != many {
		t.Fatalf("snapshot of a big execution = %d tokens, %v; want %d", len(snap.Tokens), err, many)
	}
	seen := map[store.TokenID]bool{}
	for i := range snap.Tokens {
		if seen[snap.Tokens[i].ID] {
			t.Fatalf("token %s appears twice in the snapshot", snap.Tokens[i].ID)
		}
		seen[snap.Tokens[i].ID] = true
	}
	// Another organization cannot read it.
	if _, err := e.s.LoadSnapshot(e.ctx(), e.orgB, ex.ID); err == nil {
		t.Error("an organization read the snapshot of another organization")
	}
	_, err = e.s.LoadSnapshot(e.ctx(), e.orgA, next[store.ExecutionID](e.ids))
	wantErr(t, "an execution that does not exist", err, flow.ErrNotFound)
}

func tokenMovesCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "n", 0, realNow().Add(-time.Hour))
	w := worker(e)
	claim, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, Limit: 1, LeaseTTL: time.Hour, Now: realNow()})
	if err != nil || len(claim) != 1 {
		t.Fatalf("claim = %+v, %v", claim, err)
	}
	f := store.Fence{OrgID: e.orgA, Token: tok.ID, Epoch: claim[0].LeaseEpoch, Owner: w}
	upd := store.TokenUpdate{
		OrgID: e.orgA, Token: tok.ID, Status: "LEASED", NodeID: "next", EdgeID: "e1", ScopePath: "", ForkLineage: []byte(`{"p":true}`),
		PayloadReference: "payload-1",
	}
	// Advancing keeps the lease and moves the token.
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.AdvanceToken(ctx, f, upd, base) }), "advance the held token")
	got := e.mustToken(e.orgA, tok.ID)
	if got.Status != "LEASED" || got.NodeID != "next" || got.EdgeID != "e1" || got.PayloadReference != "payload-1" || got.LeaseOwner != w || got.LeaseEpoch != f.Epoch {
		t.Fatalf("advanced token = %+v", got)
	}
	// A fence that does not hold changes nothing, whatever the write.
	stale := store.Fence{OrgID: e.orgA, Token: tok.ID, Epoch: f.Epoch + 1, Owner: w}
	otherOwner := store.Fence{OrgID: e.orgA, Token: tok.ID, Epoch: f.Epoch, Owner: worker(e)}
	for name, bad := range map[string]store.Fence{"a stale epoch": stale, "another owner": otherOwner} {
		wantErr(t, "advance with "+name, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
			hijack := upd
			hijack.NodeID = "hijacked"
			return tx.AdvanceToken(ctx, bad, hijack, base)
		}), store.ErrFenced)
		wantErr(t, "move with "+name, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
			end := upd
			end.Status = "COMPLETED"
			return tx.MoveHeldToken(ctx, bad, end, base)
		}), store.ErrFenced)
	}
	if got := e.mustToken(e.orgA, tok.ID); got.NodeID != "next" || got.Status != "LEASED" {
		t.Fatalf("a refused write changed the token: %+v", got)
	}
	wantErr(t, "a move to LEASED", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.MoveHeldToken(ctx, f, upd, base) }), flow.ErrInvalidRecord)
	// Moving ends the lease; a RUNNABLE token becomes claimable again.
	upd.Status = "RUNNABLE"
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.MoveHeldToken(ctx, f, upd, base) }), "release the token")
	if got := e.mustToken(e.orgA, tok.ID); got.Status != "RUNNABLE" || !got.LeaseOwner.IsZero() || !got.LeaseExpiresAt.IsZero() {
		t.Fatalf("released token = %+v", got)
	}
	wantErr(t, "a write after the lease ended", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.AdvanceToken(ctx, f, upd, base) }), store.ErrFenced)

	// A deferred sibling is released once.
	deferred := store.Token{
		ID: next[store.TokenID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, Status: "DEFERRED", NodeID: "d", ForkLineage: []byte(`{}`),
		FairnessKey: "flow/" + ex.FlowID.String(), AvailableAt: base, CreatedAt: base, UpdatedAt: base,
	}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertToken(ctx, &deferred) }), "insert a deferred token")
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		return tx.ReleaseDeferredToken(ctx, e.orgA, deferred.ID, 7, base)
	}), "release the deferred token")
	if got := e.mustToken(e.orgA, deferred.ID); got.Status != "RUNNABLE" {
		t.Fatalf("released deferred token = %+v", got)
	}
	wantErr(t, "releasing a token that is not deferred", e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		return tx.ReleaseDeferredToken(ctx, e.orgA, deferred.ID, 8, base)
	}), flow.ErrConflict)

	// A cancel takes a token whoever leases it, and only once.
	var live, again bool
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		var err error
		if live, err = tx.CancelToken(ctx, e.orgA, deferred.ID, base); err != nil {
			return err
		}
		again, err = tx.CancelToken(ctx, e.orgA, deferred.ID, base)
		return err
	}), "cancel")
	if !live || again {
		t.Errorf("cancel of a live token = %v, of a canceled one = %v; want true, false", live, again)
	}
	if got := e.mustToken(e.orgA, deferred.ID); got.Status != "CANCELED" {
		t.Errorf("canceled token = %+v", got)
	}
}

func payloadsCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "n", 0, base)
	ne := store.NodeExecution{
		ID: next[store.NodeExecutionID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: "n", Status: "SUCCEEDED",
		StartedAt: base, FinishedAt: base, CreatedAt: base, UpdatedAt: base,
	}
	output := store.NodeOutput{
		ID: next[store.PayloadID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, NodeExecutionID: ne.ID, Port: "main", ItemCount: 1,
		Encoding: "JSON", Document: []byte(`{"items":[{"json":{"a":1}}]}`), CreatedAt: base,
	}
	input := store.TokenInput{
		ID: next[store.PayloadID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, ItemCount: 2, Encoding: "JSON",
		Document: []byte(`{"items":[{"json":1},{"json":2}]}`), CreatedAt: base,
	}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error { return tx.InsertNodeExecution(ctx, ne) }), "insert the node execution")
	for range 2 { // storing a batch again is a no-op: the ids are deterministic
		mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
			if err := tx.PutNodeOutput(ctx, output); err != nil {
				return err
			}
			return tx.PutTokenInput(ctx, input)
		}), "store the batches")
	}
	for name, p := range map[string]struct {
		id   store.PayloadID
		want string
	}{"a node output": {output.ID, string(output.Document)}, "a token input": {input.ID, string(input.Document)}} {
		doc, err := e.s.GetPayload(e.ctx(), e.orgA, p.id)
		if err != nil || string(doc) != p.want {
			t.Errorf("GetPayload(%s) = %q, %v; want %q", name, doc, err, p.want)
		}
		if _, err := e.s.GetPayload(e.ctx(), e.orgB, p.id); err == nil {
			t.Errorf("another organization read %s", name)
		}
	}
	_, err := e.s.GetPayload(e.ctx(), e.orgA, next[store.PayloadID](e.ids))
	wantErr(t, "a batch that does not exist", err, flow.ErrNotFound)
	// The transaction sees what it wrote before it commits.
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		fresh := store.TokenInput{ID: next[store.PayloadID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, Encoding: "JSON", Document: []byte(`{"items":[]}`), CreatedAt: base}
		if err := tx.PutTokenInput(ctx, fresh); err != nil {
			return err
		}
		doc, err := tx.GetPayload(ctx, e.orgA, fresh.ID)
		if err != nil || string(doc) != `{"items":[]}` {
			return fmt.Errorf("GetPayload inside the transaction = %q, %w", doc, err)
		}
		return nil
	}), "read inside the transaction")
}

func environmentsCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "n", 0, base)
	ne := store.NodeExecution{
		ID: next[store.NodeExecutionID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: "n", Status: "SUCCEEDED",
		StartedAt: base, FinishedAt: base, CreatedAt: base, UpdatedAt: base,
	}
	rec := store.Nondeterminism{
		ID: next[store.NondeterminismID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, NodeExecutionID: ne.ID, Sequence: 1,
		Document: []byte(`{"time":1760000000000000,"seed":"2a"}`), CreatedAt: base,
	}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if err := tx.InsertNodeExecution(ctx, ne); err != nil {
			return err
		}
		if err := tx.RecordNondeterminism(ctx, rec); err != nil {
			return err
		}
		rec.ID = next[store.NondeterminismID](e.ids) // a second record of the same activation is a no-op
		return tx.RecordNondeterminism(ctx, rec)
	}), "record the environment")
	got, err := e.s.ListTokenNondeterminism(e.ctx(), e.orgA, tok.ID, 10)
	if err != nil || len(got) != 1 || got[0].NodeExecutionID != ne.ID || got[0].Sequence != 1 || !bytes.Equal(got[0].Document, rec.Document) {
		t.Fatalf("recorded environments = %+v, %v", got, err)
	}
	if other, err := e.s.ListTokenNondeterminism(e.ctx(), e.orgB, tok.ID, 10); err != nil || len(other) != 0 {
		t.Fatalf("another organization listed %+v, %v", other, err)
	}
}

func claimStartsCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	e.token(ex, "a", 0, realNow().Add(-time.Hour))
	e.token(ex, "b", 0, realNow().Add(-time.Hour))
	w := worker(e)
	if got, err := e.s.GetExecution(e.ctx(), e.scopeA, ex.ID); err != nil || got.Status != "QUEUED" {
		t.Fatalf("new execution = %+v, %v", got, err)
	}
	for range 2 {
		if c, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, Limit: 1, LeaseTTL: time.Hour, Now: realNow()}); err != nil || len(c) != 1 {
			t.Fatalf("claim = %+v, %v", c, err)
		}
	}
	got, err := e.s.GetExecution(e.ctx(), e.scopeA, ex.ID)
	if err != nil || got.Status != "RUNNING" || got.StartedAt.IsZero() {
		t.Fatalf("execution after its first claim = %+v, %v; want RUNNING with a start instant", got, err)
	}
	started := 0
	events, err := e.s.ListExecutionEvents(e.ctx(), e.scopeA, ex.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	for i := range events {
		if events[i].Type == "execution.started" {
			started++
		}
	}
	if started != 1 {
		t.Errorf("%d execution.started events after two claims, want 1", started)
	}
}

func abandonmentCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	tok := e.token(ex, "work", 0, realNow().Add(-time.Hour))
	otherFlow := e.flow2(e.orgA, e.projA, "other")
	other := e.executionIn(e.orgA, e.projA, otherFlow, e.firstVersionOf(otherFlow))
	untouched := e.token(other, "other", 0, realNow().Add(-time.Hour))
	w := worker(e)
	if err := e.s.RegisterWorker(e.ctx(), execution.WorkerRecord{ID: w, Role: execution.WorkerWorker, Host: "host", Version: "1.0.0", StartedAt: base, HeartbeatAt: base}); err != nil {
		t.Fatal(err)
	}
	if c, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, Limit: 10, LeaseTTL: time.Hour, Now: realNow()}); err != nil || len(c) != 2 {
		t.Fatalf("claim = %d tokens, %v; want both", len(c), err)
	}
	ne := store.NodeExecution{
		ID: next[store.NodeExecutionID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: "work", Status: "RUNNING",
		StartedAt: base, CreatedAt: base, UpdatedAt: base,
	}
	attempt := func(n int64, status string) store.TaskAttempt {
		a := store.TaskAttempt{
			ID: id.UUID(next[store.TokenID](e.ids)), OrgID: e.orgA, ExecutionID: ex.ID, NodeExecutionID: ne.ID, AttemptNumber: n, LeaseEpoch: 1,
			Worker: w, Status: status, StartedAt: base,
		}
		if status != "STARTED" {
			a.FinishedAt = base
		}
		return a
	}
	mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
		if err := tx.InsertNodeExecution(ctx, ne); err != nil {
			return err
		}
		if err := tx.InsertAttempt(ctx, attempt(1, "FAILED")); err != nil {
			return err
		}
		return tx.InsertAttempt(ctx, attempt(2, "STARTED"))
	}), "seed the attempts")

	// The leases a previous incarnation of the worker left (BL-051): nobody returned them, so each token counts
	// one abandonment, the started attempt is abandoned, and the execution records it.
	released, err := e.s.ReleaseLostLeases(e.ctx(), w, 3, realNow())
	if err != nil || len(released) != 2 {
		t.Fatalf("released = %+v, %v; want both tokens", released, err)
	}
	for _, r := range released {
		if r.AbandonedCount != 1 || r.Poisoned {
			t.Errorf("released %+v, want one abandonment and no poison", r)
		}
	}
	attempts, err := e.s.ListAttempts(e.ctx(), e.orgA, ne.ID, 10)
	if err != nil || len(attempts) != 2 || attempts[0].Status != "FAILED" || attempts[1].Status != "ABANDONED" || attempts[1].FinishedAt.IsZero() {
		t.Fatalf("attempts after the release = %+v, %v; want FAILED kept and STARTED abandoned", attempts, err)
	}
	if got := e.mustToken(e.orgA, tok.ID); got.Status != "RUNNABLE" || got.AbandonedCount != 1 {
		t.Fatalf("released token = %+v", got)
	}
	countEvents := func(execID store.ExecutionID, typ string) int {
		t.Helper()
		events, err := e.s.ListExecutionEvents(e.ctx(), e.scopeA, execID, 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for i := range events {
			if events[i].Type == typ {
				n++
			}
		}
		return n
	}
	if got := countEvents(ex.ID, "lease.expired"); got != 1 {
		t.Errorf("%d lease.expired events, want 1", got)
	}

	// A threshold of two makes the next abandonment of the token poison: suspended, with its event and attention
	// request, and the execution suspended once nothing else of it can run. The other execution is not touched.
	if c, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, FairnessKeys: []string{"flow/" + ex.FlowID.String()}, Limit: 10, LeaseTTL: time.Hour, Now: realNow()}); err != nil || len(c) != 1 {
		t.Fatalf("claim of the token = %d tokens, %v", len(c), err)
	}
	released, err = e.s.ReleaseLostLeases(e.ctx(), w, 2, realNow())
	if err != nil || len(released) != 1 || released[0].TokenID != tok.ID || !released[0].Poisoned || released[0].AbandonedCount != 2 {
		t.Fatalf("second release = %+v, %v; want the token poisoned at its second abandonment", released, err)
	}
	if got := e.mustToken(e.orgA, tok.ID); got.Status != "SUSPENDED" || got.SuspendReason != "POISON" {
		t.Fatalf("poisoned token = %+v", got)
	}
	if got := countEvents(ex.ID, "token.poisoned"); got != 1 {
		t.Errorf("%d token.poisoned events, want 1", got)
	}
	requests, err := e.s.ListAttentionRequests(e.ctx(), e.scopeA, "OPEN", store.Cursor{}, 10)
	if err != nil || len(requests) != 1 {
		t.Fatalf("attention requests = %+v, %v; want one", requests, err)
	}
	request := requests[0]
	if request.Cause != "POISON_TOKEN" || request.TokenID != tok.ID || request.NodeID != "work" || request.ExecutionID != ex.ID {
		t.Fatalf("attention request = %+v, want a POISON_TOKEN for the token", request)
	}
	if want := store.AttentionRequestID(id.Derive("attention", "POISON_TOKEN", tok.ID.String())); request.ID != want {
		t.Errorf("attention request id = %s, want the deterministic %s (both stores derive it the same way)", request.ID, want)
	}
	if got, err := e.s.GetExecution(e.ctx(), e.scopeA, ex.ID); err != nil || got.Status != "SUSPENDED" {
		t.Errorf("execution = %+v, %v; want SUSPENDED once its only token is suspended", got, err)
	}
	if got := e.mustToken(e.orgA, untouched.ID); got.Status != "RUNNABLE" || got.AbandonedCount != 1 {
		t.Errorf("token of the other execution = %+v, want it as the first release left it", got)
	}
	if got, err := e.s.GetExecution(e.ctx(), e.scopeA, other.ID); err != nil || got.Status == "SUSPENDED" {
		t.Errorf("the other execution = %+v, %v; want it unaffected", got, err)
	}
}

func timerWakeCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	const due = 24
	type waiting struct {
		ex  *store.Execution
		tok store.Token
		tm  store.TimerID
	}
	var all []waiting
	for range due {
		ex := e.execution()
		all = append(all, waiting{ex: ex, tok: e.token(ex, "wait", 0, realNow().Add(-time.Hour)), tm: next[store.TimerID](e.ids)})
	}
	later := e.execution()
	laterTok := e.token(later, "later", 0, realNow().Add(-time.Hour))
	laterTimer := next[store.TimerID](e.ids)
	w := worker(e)
	claimed, err := e.s.ClaimTokens(e.ctx(), store.ClaimRequest{Worker: w, Limit: due + 1, LeaseTTL: time.Hour, Now: realNow()})
	if err != nil || len(claimed) != due+1 {
		t.Fatalf("claim = %d tokens, %v; want %d", len(claimed), err, due+1)
	}
	epochs := map[store.TokenID]int64{}
	for i := range claimed {
		epochs[claimed[i].TokenID] = claimed[i].LeaseEpoch
	}
	// Each token waits for a timer (the wait nodes are cut C8; these are the rows C8 writes): the timer, the token's
	// move to WAITING and the counters commit together.
	wait := func(ex *store.Execution, tok store.Token, tm store.TimerID, dueAt time.Time) {
		t.Helper()
		f := store.Fence{OrgID: e.orgA, Token: tok.ID, Epoch: epochs[tok.ID], Owner: w}
		mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
			timer := store.Timer{ID: tm, OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, Kind: "WAIT", DueAt: dueAt, CreatedAt: base}
			if err := tx.InsertTimer(ctx, timer); err != nil {
				return err
			}
			if err := tx.MoveToken(ctx, f, "WAITING", base); err != nil {
				return err
			}
			return tx.RefreshCounters(ctx, e.orgA, ex.ID, base)
		}), "put the token to wait")
	}
	for i := range all {
		x := &all[i]
		wait(x.ex, x.tok, x.tm, realNow().Add(-time.Second))
	}
	wait(later, laterTok, laterTimer, realNow().Add(time.Hour))

	// Concurrent schedulers fire every due timer once, and each wakes its token in the same commit (BL-052).
	var wg sync.WaitGroup
	var mu sync.Mutex
	fired := map[store.TimerID]int{}
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				got, err := e.s.FireDueTimers(e.ctx(), 5, realNow())
				if err != nil {
					t.Error(err)
					return
				}
				if len(got) == 0 {
					return
				}
				mu.Lock()
				for i := range got {
					fired[got[i].TimerID]++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	for i := range all {
		x := &all[i]
		if fired[x.tm] != 1 {
			t.Errorf("timer %s fired %d times, want once", x.tm, fired[x.tm])
		}
		timer, err := e.s.GetTimer(e.ctx(), e.scopeA, x.tm)
		if err != nil || timer.Status != "FIRED" {
			t.Fatalf("timer = %+v, %v", timer, err)
		}
		if got := e.mustToken(e.orgA, x.tok.ID); got.Status != "RUNNABLE" {
			t.Errorf("token of timer %s is %s after the timer fired, want RUNNABLE: a fired timer must never leave a waiting token", x.tm, got.Status)
		}
		ex, err := e.s.GetExecution(e.ctx(), e.scopeA, x.ex.ID)
		if err != nil || ex.RunnableCount != 1 || ex.WaitingCount != 0 {
			t.Errorf("execution counters = %+v, %v; want one runnable token and none waiting", ex, err)
		}
	}
	if got := e.mustToken(e.orgA, laterTok.ID); got.Status != "WAITING" {
		t.Errorf("token whose timer is not due is %s, want WAITING", got.Status)
	}
	if timer, err := e.s.GetTimer(e.ctx(), e.scopeA, laterTimer); err != nil || timer.Status != "SCHEDULED" {
		t.Errorf("timer that is not due = %+v, %v; want SCHEDULED", timer, err)
	}
}
