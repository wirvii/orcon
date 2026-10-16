// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
)

// errLostToken means the token the step was working is no longer leased to this
// worker at this epoch (another holder, a cancellation, a finished execution).
var errLostToken = errors.New("durable: the token is no longer leased to this worker")

// step is everything a commit needs to know about the step it commits.
type step struct {
	exec *store.Execution
	plan *execution.Plan
	// fence names the lease of the token the worker holds; the zero Fence on
	// the start of an execution.
	fence store.Fence
	// baseRevision is the revision of the engine state the step was computed on.
	baseRevision int64
	work         execution.WorkOptions
	known        map[execution.NodeExecutionID]execution.NodeEnv
	delta        execution.Delta
	result       execution.WorkResult
}

// committed is what a commit did.
type committed struct {
	delta    execution.Delta
	result   execution.WorkResult
	replayed bool
	// held is the lease the worker keeps after the commit; the zero Fence if none.
	held store.Fence
	// runnable counts the tokens made available to claim.
	runnable int
}

// commit writes a step in one transaction. It locks the execution, checks the
// fence of the token, and, when another commit moved the execution on since the
// step was computed, recomputes the step on the newer state with the activations
// the worker already made, instead of overwriting the newer state (ADR-0013
// sections 4 and 6, bounded recomputation).
func (e *Engine) commit(ctx context.Context, s *step) (committed, error) {
	org := s.exec.OrgID
	var out committed
	err := e.inTx(ctx, org, func(ctx context.Context, tx store.Tx) error {
		out = committed{}
		now := e.cfg.Clock.Now()
		state, err := tx.LockExecution(ctx, org, s.exec.ID)
		if err != nil {
			return fmt.Errorf("lock execution %s: %w", s.exec.ID, err)
		}
		if s.fence.Epoch != 0 {
			if err := tx.AssertFence(ctx, s.fence, now); err != nil {
				return err
			}
		}
		delta, result := s.delta, s.result
		if state.Revision != s.baseRevision {
			if delta, result, err = e.recompute(ctx, tx, s); err != nil {
				return err
			}
			out.replayed = true
		}
		runnable, err := e.apply(ctx, tx, &applyInput{
			exec: s.exec, plan: s.plan, fence: s.fence, delta: &delta, revision: state.Revision, now: now, held: delta.Held,
		})
		if err != nil {
			return err
		}
		if e.cfg.Faults != nil {
			if err := e.cfg.Faults.Hit(ctx, PointInCommit, s.fence.Token); err != nil {
				return err
			}
		}
		out.delta, out.result, out.runnable = delta, result, runnable
		if delta.Held != "" {
			out.held = s.fence
			if string(delta.Held) != s.fence.Token.String() {
				held, err := tokenID(delta.Held)
				if err != nil {
					return err
				}
				out.held = store.Fence{OrgID: org, Token: held, Epoch: 1, Owner: e.cfg.Worker}
			}
		}
		return nil
	})
	return out, err
}

// recompute applies the step again on the state the transaction sees, with the
// activations of the first computation. It runs inside the transaction because
// the state it read must be the state it commits on; it calls no port, because
// the oracle answers for every node that needs one and the session stops at the
// first node it cannot answer for.
func (e *Engine) recompute(ctx context.Context, tx store.Tx, s *step) (execution.Delta, execution.WorkResult, error) {
	org := s.exec.OrgID
	snap, err := tx.LoadSnapshot(ctx, org, s.exec.ID)
	if err != nil {
		return execution.Delta{}, execution.WorkResult{}, fmt.Errorf("load snapshot of %s: %w", s.exec.ID, err)
	}
	held := false
	for i := range snap.Tokens {
		t := &snap.Tokens[i]
		if t.ID == s.fence.Token && t.Status == "LEASED" && t.LeaseOwner == s.fence.Owner && t.LeaseEpoch == s.fence.Epoch {
			held = true
		}
	}
	if !held {
		return execution.Delta{}, execution.WorkResult{}, store.ErrFenced
	}
	esnap, err := snapshotOf(s.exec.ID, &snap)
	if err != nil {
		return execution.Delta{}, execution.WorkResult{}, err
	}
	known := make(map[execution.NodeExecutionID]execution.NodeEnv, len(s.known)+len(s.delta.Envs))
	for k, v := range s.known {
		known[k] = v
	}
	oracle := make(map[execution.NodeExecutionID]*execution.Activation, len(s.delta.Activations))
	for _, a := range s.delta.Activations {
		oracle[a.NodeExecution] = a
	}
	sess, err := e.interp.OpenSession(s.plan, esnap, execution.SessionOptions{Org: s.exec.OrgID, Project: s.exec.ProjectID, Known: known, Oracle: oracle})
	if err != nil {
		return execution.Delta{}, execution.WorkResult{}, fmt.Errorf("reopen execution %s: %w", s.exec.ID, err)
	}
	work := s.work
	work.Pause = nil // the recomputation has no time bound: it must reach the point the first computation did
	result, err := sess.Work(withTx(withOrg(ctx, org), tx), execution.TokenID(s.fence.Token.String()), work)
	if err != nil {
		return execution.Delta{}, execution.WorkResult{}, fmt.Errorf("recompute step of %s: %w", s.exec.ID, err)
	}
	delta, err := sess.Delta()
	return delta, result, err
}

type applyInput struct {
	exec     *store.Execution
	plan     *execution.Plan
	fence    store.Fence
	delta    *execution.Delta
	revision int64
	now      time.Time
	held     execution.TokenID
}

// apply writes a delta: the new tokens, the advance of the held token and the
// other tokens it moved, the node executions, the batches, the environments
// recorded, the events, the outbox rows and, last, the engine state at the
// revision the transaction holds. It returns how many tokens became claimable.
func (e *Engine) apply(ctx context.Context, tx store.Tx, a *applyInput) (int, error) {
	d, org, ex, now := a.delta, a.exec.OrgID, a.exec, a.now
	order := make(map[execution.TokenID]int64, len(d.Runnable))
	for i, tid := range d.Runnable {
		order[tid] = int64(i + 1)
	}
	runnable := 0
	for i := range d.Tokens {
		row := &d.Tokens[i]
		tok, err := e.newToken(ex, row, order[row.ID], a.held, now)
		if err != nil {
			return 0, err
		}
		if err := tx.InsertToken(ctx, &tok); err != nil {
			return 0, fmt.Errorf("insert token %s: %w", row.ID, err)
		}
		if row.State == execution.TokenRunnable {
			runnable++
		}
	}
	for i := range d.Changed {
		c := &d.Changed[i]
		woke, err := e.change(ctx, tx, a, c, order[c.Row.ID])
		if err != nil {
			return 0, err
		}
		if woke {
			runnable++
		}
	}
	if err := e.writeRecords(ctx, tx, a); err != nil {
		return 0, err
	}
	if err := tx.RefreshCounters(ctx, org, ex.ID, now); err != nil {
		return 0, err
	}
	update := store.EngineStateUpdate{
		OrgID: org, Execution: ex.ID, ExpectedRevision: a.revision, Status: string(d.State), Now: now,
	}
	if d.TrackersChanged {
		update.Document = d.Trackers
	}
	if d.StateFrom == execution.ExecQueued && d.State != execution.ExecQueued {
		update.StartedAt = now
	}
	if d.State == execution.ExecSucceeded || d.State == execution.ExecFailed || d.State == execution.ExecCanceled {
		update.FinishedAt = now
	}
	if err := tx.SaveEngineState(ctx, update); err != nil {
		return 0, err
	}
	return runnable, nil
}

func (e *Engine) newToken(ex *store.Execution, row *execution.TokenRow, queueOrder int64, held execution.TokenID, now time.Time) (store.Token, error) {
	tid, err := tokenID(row.ID)
	if err != nil {
		return store.Token{}, err
	}
	doc, err := execution.EncodeTokenDocument(row)
	if err != nil {
		return store.Token{}, err
	}
	tok := store.Token{
		ID: tid, OrgID: ex.OrgID, ExecutionID: ex.ID, Status: string(row.State), NodeID: string(row.Node), EdgeID: string(row.InEdge),
		ScopePath: string(row.Scope), ForkLineage: doc, PayloadReference: string(row.Payload),
		FairnessKey: fairnessKey(ex), EffectivePriority: ex.Priority, QueueOrder: queueOrder, CreatedAt: now, UpdatedAt: now,
	}
	if row.State == execution.TokenLeased {
		if row.ID != held {
			return store.Token{}, fmt.Errorf("%w: token %s was created leased and is not the branch the worker keeps", execution.ErrInvalidRequest, row.ID)
		}
		tok.LeaseOwner, tok.LeaseEpoch, tok.LeaseTTL = e.cfg.Worker, 1, e.cfg.LeaseTTL
	}
	if row.State != execution.TokenRunnable {
		tok.QueueOrder = 0
	}
	return tok, nil
}

// fairnessKey groups the tokens of one flow, the unit claims are balanced by
// until the fairness cut refines it (ADR-0014).
func fairnessKey(ex *store.Execution) string { return "flow/" + ex.FlowID.String() }

// change writes the move of a token that existed before the step. It reports
// whether the move made a token claimable.
func (e *Engine) change(ctx context.Context, tx store.Tx, a *applyInput, c *execution.TokenChange, queueOrder int64) (bool, error) {
	org, now := a.exec.OrgID, a.now
	tid, err := tokenID(c.Row.ID)
	if err != nil {
		return false, err
	}
	upd, err := tokenUpdate(org, tid, &c.Row, queueOrder)
	if err != nil {
		return false, err
	}
	upd.ResetAbandoned = tid == a.fence.Token && progressed(a.delta)
	switch {
	case a.fence.Epoch != 0 && tid == a.fence.Token:
		if c.Row.State == execution.TokenLeased {
			return false, tx.AdvanceToken(ctx, a.fence, upd, now)
		}
		// A delayed token is claimable later, so it wakes nobody now.
		return c.Row.State == execution.TokenRunnable && c.Row.AvailableAt.IsZero(), tx.MoveHeldToken(ctx, a.fence, upd, now)
	case c.Row.State == execution.TokenCanceled:
		if _, err := tx.CancelToken(ctx, org, tid, now); err != nil {
			return false, fmt.Errorf("cancel token %s: %w", c.Row.ID, err)
		}
		return false, nil
	case c.From == execution.TokenDeferred && c.Row.State == execution.TokenRunnable:
		return true, tx.ReleaseDeferredToken(ctx, org, tid, queueOrder, now)
	default:
		return false, fmt.Errorf("%w: a step moved token %s from %s to %s, which only its holder may do", execution.ErrInvalidRequest, c.Row.ID, c.From, c.Row.State)
	}
}

func tokenUpdate(org identity.OrgID, tid store.TokenID, row *execution.TokenRow, queueOrder int64) (store.TokenUpdate, error) {
	doc, err := execution.EncodeTokenDocument(row)
	if err != nil {
		return store.TokenUpdate{}, err
	}
	return store.TokenUpdate{
		OrgID: org, Token: tid, Status: string(row.State), NodeID: string(row.Node), EdgeID: string(row.InEdge),
		ScopePath: string(row.Scope), ForkLineage: doc, PayloadReference: string(row.Payload), QueueOrder: queueOrder,
		AvailableAt: row.AvailableAt,
	}, nil
}

// writeRecords writes the node executions, the batches, the environments, the
// events and the outbox rows of a delta.
func (e *Engine) writeRecords(ctx context.Context, tx store.Tx, a *applyInput) error {
	if err := e.writeNodeExecutions(ctx, tx, a); err != nil {
		return err
	}
	for i := range a.delta.Payloads {
		if err := e.writePayload(ctx, tx, a, &a.delta.Payloads[i]); err != nil {
			return err
		}
	}
	if err := e.writeEnvs(ctx, tx, a); err != nil {
		return err
	}
	return e.writeEvents(ctx, tx, a)
}

func (e *Engine) writeNodeExecutions(ctx context.Context, tx store.Tx, a *applyInput) error {
	now := a.now
	for i := range a.delta.NodeExecutions {
		ne := &a.delta.NodeExecutions[i]
		neID, err := nodeExecutionID(ne.ID)
		if err != nil {
			return err
		}
		tid, err := tokenID(ne.Token)
		if err != nil {
			return err
		}
		rec := store.NodeExecution{
			ID: neID, OrgID: a.exec.OrgID, ExecutionID: a.exec.ID, TokenID: tid, NodeID: string(ne.Node), ScopePath: string(ne.Scope),
			Status: string(ne.State), StartedAt: now, CreatedAt: now, UpdatedAt: now,
		}
		if ne.State != execution.NodeRunning && ne.State != execution.NodeSuspended {
			rec.FinishedAt = now
		}
		if err := tx.UpsertNodeExecution(ctx, rec); err != nil {
			return fmt.Errorf("write node execution %s: %w", ne.ID, err)
		}
	}
	return nil
}

func (e *Engine) writeEnvs(ctx context.Context, tx store.Tx, a *applyInput) error {
	for _, rec := range a.delta.Envs {
		neID, err := nodeExecutionID(rec.NodeExecution)
		if err != nil {
			return err
		}
		doc, err := envDocument(rec.Env)
		if err != nil {
			return err
		}
		n := store.Nondeterminism{
			ID: store.NondeterminismID(e.cfg.IDs.New()), OrgID: a.exec.OrgID, ExecutionID: a.exec.ID, NodeExecutionID: neID,
			Sequence: 1, Document: doc, CreatedAt: a.now,
		}
		if err := tx.RecordNondeterminism(ctx, n); err != nil {
			return fmt.Errorf("record environment of %s: %w", rec.NodeExecution, err)
		}
	}
	return nil
}

func (e *Engine) writeEvents(ctx context.Context, tx store.Tx, a *applyInput) error {
	ex, now := a.exec, a.now
	for i := range a.delta.Events {
		ev := &a.delta.Events[i]
		if !persisted(a.plan, ev) {
			continue
		}
		if err := e.appendEvent(ctx, tx, ex, ev, now); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) writePayload(ctx context.Context, tx store.Tx, a *applyInput, w *execution.PayloadWrite) error {
	pid, err := payloadID(w.Ref)
	if err != nil {
		return err
	}
	doc, err := execution.EncodeBatch(w.Batch)
	if err != nil {
		return err
	}
	switch w.Kind {
	case execution.PayloadOutput:
		neID, err := nodeExecutionID(w.NodeExecution)
		if err != nil {
			return err
		}
		return tx.PutNodeOutput(ctx, store.NodeOutput{
			ID: pid, OrgID: a.exec.OrgID, ExecutionID: a.exec.ID, NodeExecutionID: neID, Port: w.Port,
			ItemCount: int64(w.Batch.Len()), Encoding: execution.ItemsEncoding, Document: doc, CreatedAt: a.now,
		})
	case execution.PayloadInput:
		return tx.PutTokenInput(ctx, store.TokenInput{
			ID: pid, OrgID: a.exec.OrgID, ExecutionID: a.exec.ID, ItemCount: int64(w.Batch.Len()),
			Encoding: execution.ItemsEncoding, Document: doc, CreatedAt: a.now,
		})
	default:
		return fmt.Errorf("%w: batch %s has no owner", execution.ErrInvalidRequest, w.Ref)
	}
}

// persisted reports whether an event is stored. Every event is streamed live;
// the completion signal of an edge is stored only when it reaches a join
// (ADR-0009 section 8: the editor learns the others from the stream).
func persisted(plan *execution.Plan, e *execution.Event) bool {
	if e.Type == execution.EventEdgeClosed {
		return plan.IsJoin(e.Node)
	}
	return true
}

// appendEvent stores an event and, when the outbox rule publishes it, its outbox
// row, in the transaction of the state change that produced it. Every event the
// engine appends goes through here; the events the store appends itself go
// through the same rule (see [store.EventRouter]).
func (e *Engine) appendEvent(ctx context.Context, tx store.Tx, ex *store.Execution, ev *execution.Event, now time.Time) error {
	row, err := eventRow(ex.OrgID, ex, ev, now, e.cfg.IDs)
	if err != nil {
		return err
	}
	if _, err := tx.AppendEvent(ctx, row); err != nil {
		return fmt.Errorf("append event %s: %w", ev.Type, err)
	}
	if e.cfg.Outbox == nil {
		return nil
	}
	if topic, payload, ok := e.cfg.Outbox(ex.OrgID, ex.ID, *ev); ok {
		err := tx.InsertOutbox(ctx, store.OutboxRow{ID: store.OutboxRowID(row.ID), OrgID: ex.OrgID, Topic: topic, Payload: payload, CreatedAt: now})
		if err != nil {
			return fmt.Errorf("insert outbox row of %s: %w", ev.Type, err)
		}
	}
	return nil
}

// progressed reports whether the step stored a new result of the token: a node
// execution that reached a terminal state. Such a step puts the abandon count of
// the token back to zero, in the same fenced statement that moves it (SPEC-001
// Enmienda 12 section 12-1): a token that always falls at the same point never
// progresses, and still reaches the threshold.
func progressed(d *execution.Delta) bool {
	for i := range d.NodeExecutions {
		if d.NodeExecutions[i].State != execution.NodeRunning {
			return true
		}
	}
	return false
}
