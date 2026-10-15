// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"fmt"
	"slices"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// A Session is the durable face of the interpreter. The in-memory interpreter
// keeps a whole run in one process; a durable worker holds a lease on one
// token and only ever sees the rows of the execution, so a Session rebuilds the
// state of the run from a [Snapshot] (the rows that are not terminal and the
// tracker document), lets the worker run the token through the same transition
// tables, join rules and routing the in-memory interpreter uses, and hands back
// what changed as a [Delta] for one transaction to commit (ADR-0013 sections 1
// and 6). Nothing in a Session touches storage: the durable driver
// (internal/execution/durable) reads the Snapshot and writes the Delta.

// TokenRow is a token as the store keeps it. Terminal tokens are not part of a
// [Snapshot], but they appear in a [Delta] when the session ended them.
type TokenRow struct {
	ID    TokenID
	State TokenState
	Node  flow.NodeID
	// InEdge is the edge the token traveled, empty for the root token and a
	// join continuation.
	InEdge  flow.EdgeID
	Scope   ScopePath
	Lineage []ForkRef
	Payload PayloadRef
	// Passed marks a join continuation: the join gate is behind it.
	Passed bool
	// Merged marks the token that starts the next iteration of a loop whose
	// returns were merged.
	Merged bool
}

func (a TokenRow) equal(b TokenRow) bool {
	return a.ID == b.ID && a.State == b.State && a.Node == b.Node && a.InEdge == b.InEdge && a.Scope == b.Scope &&
		a.Payload == b.Payload && a.Passed == b.Passed && a.Merged == b.Merged && slices.Equal(a.Lineage, b.Lineage)
}

// Snapshot is what the store knows of an execution when a worker opens a
// session on it.
type Snapshot struct {
	Execution ID
	Status    State
	// Trackers is the document written by an earlier [Delta]; nil for an
	// execution no session has worked on.
	Trackers []byte
	// Tokens lists every token that is not terminal, in any order.
	Tokens []TokenRow
}

// SessionOptions carry what earlier work recorded about the execution.
type SessionOptions struct {
	// Org is the organization of the execution. A restored session names it in
	// the requests its activations carry (the dispatcher's rate buckets and
	// credentials are per organization).
	Org identity.OrgID
	// Known holds the environments (activation instant and seed) recorded for
	// node executions that were activated before; an activation uses its record
	// instead of making a new one.
	Known map[NodeExecutionID]NodeEnv
	// Oracle holds the activations to replay instead of calling the ports of
	// the interpreter. A session with an oracle stops, without error, at the
	// first node whose activation it does not hold.
	Oracle map[NodeExecutionID]*Activation
}

// Session works on one execution for one worker. It is not safe for
// concurrent use.
type Session struct {
	r            *run
	base         map[TokenID]TokenRow
	baseTrackers []byte
	baseState    State
	// heldID is the token still leased to the worker when Work ended.
	heldID TokenID
}

// NewSession starts an execution: the session holds the root token as
// RUNNABLE, the execution input and the trackers of the empty run. Its
// [Delta] is the first commit of the execution.
func (in *Interpreter) NewSession(ctx context.Context, plan *Plan, req Request) (*Session, error) {
	if plan == nil {
		return nil, fmt.Errorf("%w: no plan", ErrInvalidRequest)
	}
	if req.Execution == "" {
		return nil, fmt.Errorf("%w: no execution id", ErrInvalidRequest)
	}
	start, err := resolveStart(plan, req.Start)
	if err != nil {
		return nil, err
	}
	if err := in.checkPorts(plan); err != nil {
		return nil, err
	}
	s := in.newSession(plan, req.Execution, SessionOptions{})
	r := s.r
	ref := inputRef(req.Execution)
	root := rootTokenID(req.Execution)
	if err := r.put(ctx, PayloadWrite{Ref: ref, Kind: PayloadInput, Token: root, Batch: req.Input}); err != nil {
		return nil, fmt.Errorf("store execution input: %w", err)
	}
	rootTok := r.spawn(root, TokenRunnable, start, -1, "", nil, ref)
	r.nodeOf(nodeKey{start, ""}).inflight++
	r.enqueue(rootTok)
	r.completeOtherStarts(start)
	return s, nil
}

// OpenSession rebuilds the run of an execution from its rows.
func (in *Interpreter) OpenSession(plan *Plan, snap Snapshot, opt SessionOptions) (*Session, error) {
	if plan == nil {
		return nil, fmt.Errorf("%w: no plan", ErrInvalidRequest)
	}
	if snap.Execution == "" {
		return nil, fmt.Errorf("%w: no execution id", ErrInvalidRequest)
	}
	if err := in.checkPorts(plan); err != nil {
		return nil, err
	}
	s := in.newSession(plan, snap.Execution, opt)
	r := s.r
	r.state = snap.Status
	s.baseState = snap.Status
	var order []TokenID
	if len(snap.Trackers) > 0 {
		var err error
		if order, err = r.decodeTrackers(snap.Trackers); err != nil {
			return nil, err
		}
		s.baseTrackers = slices.Clone(snap.Trackers)
	}
	rows, err := orderRows(snap.Tokens, order)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		row := rows[i]
		t, err := r.tokenOf(row)
		if err != nil {
			return nil, err
		}
		s.base[row.ID] = cloneRow(row)
		r.tokens[t.id] = t
		r.order = append(r.order, t)
		if t.state.isActive() {
			r.track(t)
		}
		r.holdRounds(t)
	}
	r.bindGroups()
	return s, nil
}

func (in *Interpreter) newSession(plan *Plan, exec ID, opt SessionOptions) *Session {
	pl := newSessionPayloads(in.deps.Payloads)
	r := newRun(in, plan, Request{Org: opt.Org, Execution: exec})
	r.payloads, r.owned = pl, pl
	r.rec = newRecorder(plan, in.deps.Clock, opt)
	return &Session{r: r, base: make(map[TokenID]TokenRow), baseState: ExecQueued}
}

func cloneRow(row TokenRow) TokenRow {
	row.Lineage = slices.Clone(row.Lineage)
	return row
}

// orderRows puts the rows in the creation order the trackers recorded; rows the
// trackers do not know (tokens another path created) follow, by id.
func orderRows(rows []TokenRow, order []TokenID) ([]TokenRow, error) {
	pos := make(map[TokenID]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	out := slices.Clone(rows)
	for i := range out {
		if row := &out[i]; row.State.IsTerminal() {
			return nil, fmt.Errorf("%w: token %s is %s; a snapshot holds live tokens only", ErrInvalidRequest, row.ID, row.State)
		}
	}
	slices.SortStableFunc(out, func(a, b TokenRow) int {
		pa, oka := pos[a.ID]
		pb, okb := pos[b.ID]
		switch {
		case oka && okb:
			return pa - pb
		case oka:
			return -1
		case okb:
			return 1
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	return out, nil
}

// tokenOf builds the interpreter's token from a row.
func (r *run) tokenOf(row TokenRow) (*token, error) {
	node, ok := r.plan.index[row.Node]
	if !ok {
		return nil, fmt.Errorf("%w: token %s is at node %q, which the flow does not have", ErrInvalidRequest, row.ID, row.Node)
	}
	edge := -1
	if row.InEdge != "" {
		if edge, ok = r.plan.edgeIndex[row.InEdge]; !ok {
			return nil, fmt.Errorf("%w: token %s traveled edge %q, which the flow does not have", ErrInvalidRequest, row.ID, row.InEdge)
		}
	}
	return &token{
		id: row.ID, state: row.State, node: node, inEdge: edge, scope: row.Scope, lineage: slices.Clone(row.Lineage),
		payload: row.Payload, passed: row.Passed, merged: row.Merged,
	}, nil
}

func (r *run) rowOf(t *token) TokenRow {
	row := TokenRow{
		ID: t.id, State: t.state, Node: r.nodeID(t.node), Scope: t.scope, Lineage: slices.Clone(t.lineage),
		Payload: t.payload, Passed: t.passed, Merged: t.merged,
	}
	if t.inEdge >= 0 {
		row.InEdge = r.plan.edges[t.inEdge].edge.ID
	}
	return row
}

// bindGroups connects the sequential groups the trackers recorded to the tokens
// of the session. A sibling that already ended is not loaded and appears as a
// finished stub: the group only releases deferred siblings, and a finished
// sibling needs no release.
func (r *run) bindGroups() {
	for _, pg := range r.pendingGroups {
		g := &seqGroup{keys: pg.keys, cursor: pg.cursor}
		for _, id := range pg.siblings {
			t := r.tokens[id]
			if t == nil {
				t = &token{id: id, state: TokenCompleted}
			}
			g.siblings = append(g.siblings, t)
		}
		r.groups[pg.id] = g
	}
	r.pendingGroups = nil
}

// WorkOptions bound one segment of work.
type WorkOptions struct {
	// Budget bounds the node activations of the segment (ADR-0013 section 6
	// asks for a confirmation point at the latest every 64 activations). Zero
	// means no bound.
	Budget int
	// StopAtFork ends the segment at the first fork, so that the siblings become
	// visible to other workers as soon as possible (ADR-0013 section 1).
	StopAtFork bool
	// Release gives the lease back at the end of the segment instead of
	// continuing: the token that would go on returns to RUNNABLE (graceful
	// shutdown, ADR-0013 section 9).
	Release bool
	// Pause is asked before every step; when it answers true the segment ends
	// there, with the token still held (the time bound of a confirmation point,
	// or a lease that can no longer be trusted).
	Pause func() bool
	// Checkpoint opens the checkpoint transaction of each activation, for the
	// nodes that act on external systems (see [NodeCall.Checkpoint]). Nil when no
	// durable engine runs the segment.
	Checkpoint ActivationCheckpoint
}

// WorkResult says how a segment ended.
type WorkResult struct {
	// Held is the token still leased to the worker, positioned at the node it
	// goes on with; empty when the token ended or was released.
	Held TokenID
	// Steps counts the tokens' steps taken.
	Steps int
	// Forked reports that the segment ended at a fork.
	Forked bool
	// Awaiting reports that a replay stopped at a node whose activation it did
	// not hold.
	Awaiting bool
	// Done reports that the execution reached SUCCEEDED or FAILED.
	Done bool
}

// Work runs the leased token node by node until it ends, forks (with
// [WorkOptions.StopAtFork]), reaches the budget or fails the execution, then
// settles whatever the end of its steps made possible and decides whether the
// execution is over.
func (s *Session) Work(ctx context.Context, id TokenID, opt WorkOptions) (WorkResult, error) {
	r := s.r
	r.checkpoint = opt.Checkpoint
	t := r.tokens[id]
	if t == nil || t.state != TokenLeased {
		return WorkResult{}, fmt.Errorf("%w: token %s is not leased in this session", ErrInvalidRequest, id)
	}
	if r.failure != nil || r.state == ExecFailed {
		return WorkResult{}, fmt.Errorf("%w: execution %s already failed", ErrInvalidRequest, r.req.Execution)
	}
	if err := r.begin(); err != nil {
		return WorkResult{}, err
	}
	var res WorkResult
	kept, err := r.segment(ctx, t, opt, &res)
	if err != nil {
		return res, err
	}
	if err := r.drain(ctx); err != nil {
		return res, err
	}
	if kept != nil && r.failure == nil && opt.Release {
		if err := r.setToken(kept, TokenRunnable); err != nil {
			return res, err
		}
		kept = nil
	}
	if kept != nil && kept.state == TokenLeased {
		res.Held = kept.id
		s.heldID = kept.id
	}
	if err := r.settle(); err != nil {
		return res, err
	}
	res.Done = r.state == ExecSucceeded || r.state == ExecFailed
	return res, nil
}

// segment runs the token node by node until the segment ends, and returns the
// token the worker still holds, if any.
func (r *run) segment(ctx context.Context, t *token, opt WorkOptions, res *WorkResult) (*token, error) {
	forks := r.forks
	kept := t
	for kept != nil && r.failure == nil {
		if err := ctx.Err(); err != nil {
			return kept, fmt.Errorf("execution %s: %w", r.req.Execution, err)
		}
		if r.segmentOver(opt, res, kept) {
			break
		}
		next, err := r.step(ctx, kept)
		if err != nil {
			return kept, err
		}
		res.Steps++
		if err := r.drain(ctx); err != nil {
			return kept, err
		}
		kept = next
		if opt.StopAtFork && r.forks != forks && kept != nil {
			res.Forked = true
			break
		}
	}
	return kept, nil
}

// segmentOver reports whether the segment ends before the next step: the budget
// of activations is spent, the caller asked to pause, or a replay reached a node
// whose activation it does not hold.
func (r *run) segmentOver(opt WorkOptions, res *WorkResult, kept *token) bool {
	switch {
	case opt.Budget > 0 && res.Steps >= opt.Budget:
		return true
	case opt.Pause != nil && res.Steps > 0 && opt.Pause():
		return true
	case r.rec.oracle != nil && !r.activationReady(kept):
		res.Awaiting = true
		return true
	default:
		return false
	}
}

// begin moves a queued, waiting or suspended execution to RUNNING, the first
// token of a worker being the trigger (ADR-0009 section 1).
func (r *run) begin() error {
	switch r.state {
	case ExecQueued:
		if err := r.setExec(ExecRunning); err != nil {
			return err
		}
		r.event(Event{Type: EventExecutionStarted})
	case ExecWaiting, ExecSuspended:
		if err := r.setExec(ExecRunning); err != nil {
			return err
		}
		r.event(Event{Type: EventExecutionResumed})
	case ExecRunning:
	case ExecCanceling, ExecCompensating, ExecSucceeded, ExecFailed, ExecCanceled:
		return fmt.Errorf("%w: execution %s is %s", ErrInvalidRequest, r.req.Execution, r.state)
	}
	return nil
}

// activationReady reports whether the next step of a token can run in a
// replay: the node it is at either needs no port or was activated before.
func (r *run) activationReady(t *token) bool {
	n := &r.plan.nodes[t.node]
	if !n.needsPorts() || (n.join != nil && !t.passed) || n.kind == kindLoop {
		return true
	}
	return r.rec.ready(nodeExecutionID(r.req.Execution, t.id, n.node.ID, t.scope))
}

// settle ends the execution when no live token is left.
func (r *run) settle() error {
	if r.failure != nil || r.state != ExecRunning {
		return nil
	}
	for _, t := range r.order {
		if !t.state.IsTerminal() {
			return nil
		}
	}
	if err := r.setExec(ExecSucceeded); err != nil {
		return err
	}
	r.event(Event{Type: EventExecutionSucceeded})
	return nil
}

// TokenChange is a token the session moved: its new row and the state the
// store has to find it in.
type TokenChange struct {
	Row  TokenRow
	From TokenState
}

// Delta is what a session changed, to be committed in one transaction.
type Delta struct {
	Execution ID
	// State is the state of the execution after the work, StateFrom before.
	State     State
	StateFrom State
	// Failure explains a FAILED execution.
	Failure *Failure
	// Tokens are the tokens the session created, in creation order. A token
	// that also ended in the session appears with its final state.
	Tokens []TokenRow
	// Changed are the existing tokens whose row differs.
	Changed []TokenChange
	// NodeExecutions are the node executions of the session, in creation order,
	// with their final state.
	NodeExecutions []NodeExecutionRecord
	// Events are the events of the session in order. Their Sequence is local.
	Events []Event
	// Payloads are the batches to store before the commit refers to them.
	Payloads []PayloadWrite
	// Trackers is the document of the completion trackers; TrackersChanged says
	// whether it differs from the one the session opened with.
	Trackers        []byte
	TrackersChanged bool
	// Ends are the tokens that reached an end event in the session.
	Ends []EndArrival
	// Envs are the activation environments created in the session.
	Envs []EnvRecord
	// Activations are the activations that ran or replayed in the session, to
	// replay them on a newer state of the execution.
	Activations []*Activation
	// Runnable lists, in order, the tokens the session made available to claim
	// (some may have ended since): the order of the claims, so that one worker
	// running an execution alone sees the order of the in-memory interpreter.
	Runnable []TokenID
	// Held is the token still leased to the worker.
	Held TokenID
	// Live counts the tokens that are not terminal after the work.
	Live int
}

// Delta returns what the session changed since it was opened.
func (s *Session) Delta() (Delta, error) {
	r := s.r
	d := Delta{
		Execution: r.req.Execution, State: r.state, StateFrom: s.baseState, Failure: r.failure,
		Events: slices.Clone(r.events), Ends: slices.Clone(r.ends), Payloads: slices.Clone(r.owned.since(0)),
		Envs: slices.Clone(r.rec.created), Activations: slices.Clone(r.rec.acts),
	}
	order := make([]TokenID, 0, len(r.order))
	for _, t := range r.order {
		row := r.rowOf(t)
		before, existed := s.base[t.id]
		switch {
		case !existed:
			d.Tokens = append(d.Tokens, row)
		case !row.equal(before):
			d.Changed = append(d.Changed, TokenChange{Row: row, From: before.State})
		}
		if !t.state.IsTerminal() {
			order = append(order, t.id)
			d.Live++
		}
	}
	d.Runnable = slices.Clone(r.rec.queued)
	d.Held = s.heldID
	for _, ne := range r.execOrder {
		d.NodeExecutions = append(d.NodeExecutions, NodeExecutionRecord{
			ID: ne.id, Token: ne.token, Node: r.nodeID(ne.node), Scope: ne.scope, State: ne.state,
		})
	}
	trackers, err := r.encodeTrackers(order)
	if err != nil {
		return Delta{}, err
	}
	d.Trackers = trackers
	d.TrackersChanged = !slices.Equal(trackers, s.baseTrackers)
	return d, nil
}
