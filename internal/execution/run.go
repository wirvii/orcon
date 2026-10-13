// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/wirvii/orcon/internal/flow"
)

// errInternal marks a broken invariant of the interpreter itself.
var errInternal = errors.New("execution: internal error")

func internalf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errInternal, fmt.Sprintf(format, args...))
}

// nodeKey identifies a node in a scope path: the unit trackers are kept per.
type nodeKey struct {
	node  int
	scope ScopePath
}

// edgeKey identifies an edge in the scope its target lives in.
type edgeKey struct {
	edge  int
	scope ScopePath
}

// forkKey identifies one branch of one split node execution.
type forkKey struct {
	split  NodeExecutionID
	branch int
}

// token is the unit of work (R-REC-01).
type token struct {
	id      TokenID
	state   TokenState
	node    int
	inEdge  int // plan edge the token traveled, -1 for the root and join continuations
	scope   ScopePath
	lineage []ForkRef
	payload PayloadRef
	// passed marks a join continuation: the join gate is behind it.
	passed bool
	// tracked is set while the token counts toward loop and branch counters.
	tracked bool
	// merged marks the token that starts the next iteration of a loop whose
	// returns were merged: it is not collected again.
	merged bool
	// roundHeld is set until the token reaches a terminal state; it keeps the
	// rounds of its split instances open.
	roundHeld bool
}

// nodeState is the completion tracker of a node in a scope (ADR-0010
// section 2): the node completes its outgoing edges once every incoming edge
// is complete and every token delivered to it has been processed.
type nodeState struct {
	inflight  int
	signaled  []bool
	completed bool
}

func (ns *nodeState) allSignaled() bool {
	for _, s := range ns.signaled {
		if !s {
			return false
		}
	}
	return true
}

// loopInstance tracks every instance of a loop control in a scope: each token
// that arrives on the entry edge runs its own instance (named in the scope
// path), and the loop's exit edges close once all of them have finished.
type loopInstance struct {
	// live counts the active tokens inside the instance, every iteration.
	live int
	// armed is set once the entry is complete: no more tokens will enter.
	armed  bool
	closed bool
}

// loopReturn is a token that came back to a loop control by a back edge.
type loopReturn struct {
	edge    int
	token   TokenID
	payload PayloadRef
	lineage []ForkRef
}

// loopReturns are the returns of one iteration of a loop instance.
type loopReturns struct {
	loop    int
	arrived []loopReturn
	started bool
}

// seqGroup holds the sibling tokens of one fork in SEQUENTIAL branch mode.
type seqGroup struct {
	siblings []*token
	keys     []forkKey
	// cursor is the index of the last sibling released.
	cursor int
	dirty  bool
}

// signal is a pending completion signal: edge e is complete in scope.
type signal struct {
	edgeKey
}

// nodeExec is a node execution.
type nodeExec struct {
	id    NodeExecutionID
	token TokenID
	node  int
	scope ScopePath
	state NodeExecutionState
}

// emission is a live token an activation sends along an edge.
type emission struct {
	edge    int
	payload PayloadRef
}

// run is the state of one execution.
type run struct {
	in   *Interpreter
	plan *Plan
	req  Request

	payloads Payloads
	state    State
	failure  *Failure

	tokens map[TokenID]*token
	order  []*token
	queue  []*token

	nodes    map[nodeKey]*nodeState
	joins    map[nodeKey]*joinState
	loops    map[nodeKey]*loopInstance
	edgeLive map[edgeKey]int
	forkLive map[forkKey]int
	groups   map[NodeExecutionID]*seqGroup
	// canceledSplits holds the split executions whose branches a discriminator
	// canceled; a join whose continuation would descend from one is canceled.
	canceledSplits map[NodeExecutionID]bool
	// splitLive counts the active tokens that descend from each split instance.
	// It is the one descendant accounting behind SEQUENTIAL branch order and the
	// reset of LET_FINISH rounds.
	splitLive map[NodeExecutionID]int
	// roundJoins lists, per split instance, the LET_FINISH joins that opened a
	// round for it, to look at when a descendant ends.
	roundJoins map[NodeExecutionID][]nodeKey
	dirtyJoins []nodeKey
	// iterations holds the loop iteration scopes already started.
	iterations map[ScopePath]bool
	// scopeLive counts the active tokens in each scope path, nested scopes
	// included: an iteration is over when no token is left in its scope.
	scopeLive map[ScopePath]int
	// returns collects, per loop iteration scope, the tokens that came back by
	// the back edges of a loop that merges its returns.
	returns    map[ScopePath]*loopReturns
	dirtyIters []ScopePath
	nodeExecs  map[NodeExecutionID]*nodeExec
	execOrder  []*nodeExec

	signals    []signal
	dirtyLoops []nodeKey
	dirtyGroup []*seqGroup

	events  []Event
	flushed int
	ends    []EndArrival
}

func newRun(in *Interpreter, plan *Plan, req Request) *run {
	payloads := in.deps.Payloads
	if payloads == nil {
		payloads = NewMemoryPayloads()
	}
	return &run{
		in:       in,
		plan:     plan,
		req:      req,
		payloads: payloads,
		state:    ExecQueued,
		tokens:   make(map[TokenID]*token),
		nodes:    make(map[nodeKey]*nodeState),
		joins:    make(map[nodeKey]*joinState),
		loops:    make(map[nodeKey]*loopInstance),
		edgeLive: make(map[edgeKey]int),
		forkLive: make(map[forkKey]int),
		groups:   make(map[NodeExecutionID]*seqGroup),

		canceledSplits: make(map[NodeExecutionID]bool),
		splitLive:      make(map[NodeExecutionID]int),
		roundJoins:     make(map[NodeExecutionID][]nodeKey),
		iterations:     make(map[ScopePath]bool),
		scopeLive:      make(map[ScopePath]int),
		returns:        make(map[ScopePath]*loopReturns),
		nodeExecs:      make(map[NodeExecutionID]*nodeExec),
	}
}

// execute runs the execution to its end.
func (r *run) execute(ctx context.Context, start int) error {
	ref := inputRef(r.req.Execution)
	if err := r.payloads.Put(ctx, ref, r.req.Input); err != nil {
		return fmt.Errorf("store execution input: %w", err)
	}
	root := r.spawn(rootTokenID(r.req.Execution), TokenRunnable, start, -1, "", nil, ref)
	r.nodeOf(nodeKey{start, ""}).inflight++
	r.queue = append(r.queue, root)
	r.completeOtherStarts(start)
	for r.failure == nil {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("execution %s: %w", r.req.Execution, err)
		}
		t, ok, err := r.claim()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if err := r.work(ctx, t); err != nil {
			return err
		}
		if err := r.flush(ctx); err != nil {
			return err
		}
	}
	if err := r.flush(ctx); err != nil {
		return err
	}
	if r.failure != nil {
		return nil
	}
	return r.succeed(ctx)
}

// completeOtherStarts completes the outgoing edges of the start events that
// were not selected with no live token, so joins downstream see them as
// not-taken paths.
func (r *run) completeOtherStarts(start int) {
	for _, s := range r.plan.starts {
		if s != start {
			r.checkComplete(nodeKey{s, ""})
		}
	}
}

func (r *run) succeed(ctx context.Context) error {
	for _, t := range r.order {
		if t.state.isActive() {
			return internalf("execution ended with token %s still %s", t.id, t.state)
		}
	}
	if err := r.setExec(ExecSucceeded); err != nil {
		return err
	}
	r.event(Event{Type: EventExecutionSucceeded})
	return r.flush(ctx)
}

// claim takes the next runnable token, skipping tokens canceled while queued.
// It reports false when no token is left to run.
func (r *run) claim() (*token, bool, error) {
	for len(r.queue) > 0 {
		t := r.queue[0]
		r.queue = r.queue[1:]
		if t.state != TokenRunnable {
			continue
		}
		if r.state == ExecQueued {
			if err := r.setExec(ExecRunning); err != nil {
				return nil, false, err
			}
			r.event(Event{Type: EventExecutionStarted})
		}
		if err := r.setToken(t, TokenLeased); err != nil {
			return nil, false, err
		}
		return t, true, nil
	}
	return nil, false, nil
}

// work runs a leased token node by node until it ends, then settles the
// signals and releases the tokens its end made possible.
func (r *run) work(ctx context.Context, t *token) error {
	for t != nil && r.failure == nil {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("execution %s: %w", r.req.Execution, err)
		}
		next, err := r.step(ctx, t)
		if err != nil {
			return err
		}
		if err := r.drain(ctx); err != nil {
			return err
		}
		t = next
	}
	return r.drain(ctx)
}

func (r *run) setExec(to State) error {
	next, err := r.state.Transition(to)
	if err != nil {
		return err
	}
	r.state = next
	return nil
}

// event stamps and records an event.
func (r *run) event(e Event) {
	e.Sequence = len(r.events) + 1
	e.Execution = r.req.Execution
	r.events = append(r.events, e)
}

// flush hands the events recorded since the last flush to the sink.
func (r *run) flush(ctx context.Context) error {
	sink := r.in.deps.Events
	if sink == nil {
		r.flushed = len(r.events)
		return nil
	}
	for ; r.flushed < len(r.events); r.flushed++ {
		if err := sink.Emit(ctx, r.events[r.flushed]); err != nil {
			return fmt.Errorf("emit event %d (%s): %w", r.events[r.flushed].Sequence, r.events[r.flushed].Type, err)
		}
	}
	return nil
}

func (r *run) nodeID(i int) flow.NodeID { return r.plan.nodes[i].node.ID }

func (r *run) nodeOf(key nodeKey) *nodeState {
	ns, ok := r.nodes[key]
	if !ok {
		ns = &nodeState{signaled: make([]bool, len(r.plan.nodes[key.node].in))}
		r.nodes[key] = ns
	}
	return ns
}

func (r *run) joinOf(key nodeKey) *joinState {
	js, ok := r.joins[key]
	if !ok {
		n := &r.plan.nodes[key.node]
		js = newJoinState(n.join, len(n.in))
		r.joins[key] = js
	}
	return js
}

func (r *run) loopOf(key nodeKey) *loopInstance {
	li, ok := r.loops[key]
	if !ok {
		li = &loopInstance{}
		r.loops[key] = li
	}
	return li
}

// result assembles the outcome of the run.
func (r *run) result() Result {
	res := Result{
		Execution: r.req.Execution,
		Status:    r.state,
		Failure:   r.failure,
		Events:    slices.Clone(r.events),
		Ends:      slices.Clone(r.ends),
	}
	for _, t := range r.order {
		res.Tokens = append(res.Tokens, TokenRecord{
			ID:      t.id,
			State:   t.state,
			Node:    r.nodeID(t.node),
			Scope:   t.scope,
			Lineage: slices.Clone(t.lineage),
			Payload: t.payload,
		})
	}
	for _, ne := range r.execOrder {
		res.NodeExecutions = append(res.NodeExecutions, NodeExecutionRecord{
			ID: ne.id, Token: ne.token, Node: r.nodeID(ne.node), Scope: ne.scope, State: ne.state,
		})
	}
	return res
}

// load reads the batch a token or an arrival carries.
func (r *run) load(ctx context.Context, ref PayloadRef) (Batch, error) {
	b, err := r.payloads.Get(ctx, ref)
	if err != nil {
		return Batch{}, fmt.Errorf("load payload: %w", err)
	}
	return b, nil
}
