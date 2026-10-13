// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/wirvii/orcon/internal/flow"
)

// route sends the live tokens of an activation along their edges (ADR-0010
// sections 2 and 5). Zero live edges end the token; one moves it; several fork
// it into child tokens, the first of which stays with this worker (R-REC-01).
// The tracker named by arrival belongs to the node that was activated: its
// pending count drops once the tokens are delivered, and the node completes its
// outgoing edges when that makes it complete.
func (r *run) route(t *token, n *planNode, neID NodeExecutionID, execScope ScopePath,
	arrival nodeKey, countArrival bool, ems []emission,
) (*token, error) {
	ems = r.siblingOrder(ems)
	dests := make([]ScopePath, len(ems))
	for i, em := range ems {
		e := &r.plan.edges[em.edge]
		dest, ok := execScope.PopN(len(e.exits))
		if !ok {
			return nil, internalf("edge %q leaves %d loops from scope %q", e.edge.ID, len(e.exits), execScope)
		}
		dests[i] = dest
		r.deliver(em.edge, dest)
	}
	if countArrival {
		r.nodeOf(arrival).inflight--
	}
	var kept *token
	var err error
	switch len(ems) {
	case 0:
		r.event(Event{Type: EventTokenConsumed, Token: t.id, Node: n.node.ID, Scope: execScope, Detail: "NO_OUTPUT"})
		err = r.setToken(t, TokenConsumed)
	case 1:
		r.advance(t, ems[0], dests[0])
		kept = t
	default:
		kept, err = r.fork(t, n, neID, execScope, ems, dests)
	}
	if err != nil {
		return nil, err
	}
	if countArrival {
		r.checkComplete(arrival)
	}
	return kept, nil
}

// siblingOrder orders the live edges of a split. In SEQUENTIAL branch mode
// siblings run in edge order (ADR-0010 section 5); edges without an order
// come last, in document order. In PARALLEL mode the document order stands.
func (r *run) siblingOrder(ems []emission) []emission {
	if !r.plan.sequential || len(ems) < 2 {
		return ems
	}
	rank := func(em emission) int {
		if o := r.plan.edges[em.edge].edge.Order; o > 0 {
			return o
		}
		return math.MaxInt
	}
	ordered := slices.Clone(ems)
	slices.SortStableFunc(ordered, func(a, b emission) int {
		ra, rb := rank(a), rank(b)
		switch {
		case ra < rb:
			return -1
		case ra > rb:
			return 1
		default:
			return 0
		}
	})
	return ordered
}

// deliver records that a live token travels an edge toward its target.
func (r *run) deliver(edge int, dest ScopePath) {
	e := &r.plan.edges[edge]
	r.edgeLive[edgeKey{edge, dest}]++
	if e.back {
		// A back edge is not part of the completion accounting; the loop
		// control counts the returning token through the loop instance.
		return
	}
	key := nodeKey{e.to, dest}
	if r.plan.nodes[e.to].join != nil {
		r.joinOf(key).send(e.slot)
		return
	}
	r.nodeOf(key).inflight++
}

// advance moves a token along one edge.
func (r *run) advance(t *token, em emission, dest ScopePath) {
	e := &r.plan.edges[em.edge]
	t.node = e.to
	t.inEdge = em.edge
	t.payload = em.payload
	t.passed = false
	r.moveScope(t, dest)
}

// fork replaces a token by one child per live edge. The parent becomes
// CONSUMED only after the children exist, so the counters of the branches it
// belongs to never dip to zero in between.
func (r *run) fork(t *token, n *planNode, neID NodeExecutionID, execScope ScopePath, ems []emission, dests []ScopePath) (*token, error) {
	children := make([]*token, len(ems))
	keys := make([]forkKey, len(ems))
	ids := make([]TokenID, len(ems))
	for i, em := range ems {
		e := &r.plan.edges[em.edge]
		lineage := append(slices.Clone(t.lineage), ForkRef{SplitNodeExecution: neID, SplitNode: n.node.ID, Branch: e.outPos})
		state := TokenRunnable
		switch {
		case i == 0:
			state = TokenLeased
		case r.plan.sequential:
			state = TokenDeferred
		}
		id := forkTokenID(t.id, n.node.ID, execScope, e.outPos)
		children[i] = r.spawn(id, state, e.to, em.edge, dests[i], lineage, em.payload)
		keys[i] = forkKey{neID, e.outPos}
		ids[i] = id
	}
	r.event(Event{Type: EventTokenForked, Token: t.id, NodeExecution: neID, Node: n.node.ID, Scope: execScope, Tokens: ids})
	if err := r.setToken(t, TokenConsumed); err != nil {
		return nil, err
	}
	for _, c := range children[1:] {
		if c.state == TokenDeferred {
			r.event(Event{Type: EventTokenDeferred, Token: c.id, Node: n.node.ID, Scope: execScope})
			continue
		}
		r.queue = append(r.queue, c)
	}
	if r.plan.sequential {
		r.groups[neID] = &seqGroup{siblings: children, keys: keys}
	}
	return children[0], nil
}

// arrive applies the arrival of a token at a join (ADR-0010 section 3).
func (r *run) arrive(ctx context.Context, t *token, n *planNode) error {
	e := &r.plan.edges[t.inEdge]
	key := nodeKey{t.node, t.scope}
	js := r.joinOf(key)
	r.event(Event{Type: EventTokenConsumed, Token: t.id, Node: n.node.ID, Edge: e.edge.ID, Scope: t.scope, Detail: "JOIN"})
	out := js.arrive(joinArrival{Slot: e.slot, Token: t.id, Payload: t.payload, Lineage: slices.Clone(t.lineage)})
	if err := r.setToken(t, TokenConsumed); err != nil {
		return err
	}
	return r.applyJoin(ctx, key, n, js, out, t)
}

// applyJoin carries out what a join decided. The trigger is the token whose
// arrival caused the decision, or nil when a completion signal did.
func (r *run) applyJoin(ctx context.Context, key nodeKey, n *planNode, js *joinState, out joinOutcome, trigger *token) error {
	switch out {
	case joinFires:
		return r.fireJoin(ctx, key, n, js, trigger)
	case joinFails:
		r.event(Event{Type: EventJoinFailed, Node: n.node.ID, Scope: key.scope, Detail: string(js.mode)})
		return r.fail(&Failure{
			Code: CodeJoinRequirementNotMet, Node: n.node.ID, Scope: key.scope,
			Message: fmt.Sprintf("an incoming edge of the %s join completed with no live token", js.mode),
		}, nil)
	case joinEmpty:
		r.checkComplete(key)
	case joinWaiting:
	}
	return nil
}

// fireJoin creates the continuation token of a join: it combines the arrived
// batches (APPEND), drops the split the join synchronizes from the fork
// lineage and, for a discriminator, cancels the branches of that split that
// have not reached the join.
func (r *run) fireJoin(ctx context.Context, key nodeKey, n *planNode, js *joinState, trigger *token) error {
	arrivals := js.combined()
	batches := make([]Batch, len(arrivals))
	for i, a := range arrivals {
		b, err := r.load(ctx, a.Payload)
		if err != nil {
			return err
		}
		batches[i] = b
	}
	contID := joinTokenID(r.req.Execution, n.node.ID, key.scope)
	ref := joinedRef(contID)
	if err := r.payloads.Put(ctx, ref, concat(batches)); err != nil {
		return fmt.Errorf("store combined input of join %q: %w", n.node.ID, err)
	}
	dominates := func(id flow.NodeID) bool {
		i, ok := r.plan.index[id]
		return ok && n.dominators != nil && n.dominators[i]
	}
	var canceled []TokenID
	if js.mode == flow.JoinFirst && trigger != nil {
		if i := lastDominating(trigger.lineage, dominates); i >= 0 {
			var err error
			canceled, err = r.cancelBranches(trigger.lineage[i].SplitNodeExecution, key.node)
			if err != nil {
				return err
			}
		}
	}
	cont := r.spawn(contID, TokenRunnable, key.node, -1, key.scope, continuationLineage(js.combined(), dominates), ref)
	cont.passed = true
	r.nodeOf(key).inflight++
	r.queue = append(r.queue, cont)
	r.event(Event{Type: EventJoinFired, Token: contID, Node: n.node.ID, Scope: key.scope, Detail: string(js.mode), Tokens: canceled})
	return nil
}

// cancelBranches cancels the tokens descending from a split execution that
// have not reached the join: the discriminator semantics of ADR-0010
// section 3. Tokens already at the join are absorbed as later arrivals.
// Cancellation produces no completion signals: the branches it stops are
// abandoned, not routed around.
func (r *run) cancelBranches(split NodeExecutionID, join int) ([]TokenID, error) {
	var canceled []TokenID
	for _, c := range r.order {
		if !c.state.isActive() || c.node == join || !hasSplit(c.lineage, split) {
			continue
		}
		r.retract(c)
		if err := r.setToken(c, TokenCanceled); err != nil {
			return nil, err
		}
		canceled = append(canceled, c.id)
	}
	return canceled, nil
}

func hasSplit(lineage []ForkRef, split NodeExecutionID) bool {
	return slices.ContainsFunc(lineage, func(f ForkRef) bool { return f.SplitNodeExecution == split })
}

// retract undoes the delivery bookkeeping of a token canceled before it was
// processed at its node.
func (r *run) retract(t *token) {
	key := nodeKey{t.node, t.scope}
	n := &r.plan.nodes[t.node]
	switch {
	case t.passed:
		r.nodeOf(key).inflight--
	case t.inEdge < 0 || r.plan.edges[t.inEdge].back:
	case n.join != nil:
		r.joinOf(key).retract(r.plan.edges[t.inEdge].slot)
	default:
		r.nodeOf(key).inflight--
	}
}

// checkComplete completes the outgoing edges of a node once its input is
// complete and its tokens are processed (ADR-0010 section 2). A node that
// never ran completes them with no live token: dead path elimination.
func (r *run) checkComplete(key nodeKey) {
	n := &r.plan.nodes[key.node]
	ns := r.nodeOf(key)
	if ns.completed || ns.inflight > 0 {
		return
	}
	if n.join != nil {
		js := r.joins[key]
		if js == nil || (!js.fired && !js.empty) {
			return
		}
	} else if !ns.allSignaled() {
		return
	}
	ns.completed = true
	if n.kind == kindLoop {
		r.armLoop(key)
		return
	}
	r.completeOut(n, key.scope)
}

// completeOut queues the completion signal of every outgoing edge of a node
// that completed in a scope. Back edges are excluded from completion
// accounting; exit edges of a loop complete when the loop's instance closes.
func (r *run) completeOut(n *planNode, scope ScopePath) {
	for _, ei := range n.out {
		e := &r.plan.edges[ei]
		if e.back || len(e.exits) > 0 {
			continue
		}
		r.signals = append(r.signals, signal{edge: ei, scope: scope})
	}
}

// drain settles everything the last step made possible: completion signals,
// deferred siblings that may run and loop instances that may close. It stops
// as soon as the execution fails.
func (r *run) drain(ctx context.Context) error {
	for r.failure == nil {
		switch {
		case len(r.signals) > 0:
			sg := r.signals[0]
			r.signals = r.signals[1:]
			if err := r.applySignal(ctx, sg); err != nil {
				return err
			}
		case len(r.dirtyGroup) > 0:
			g := r.dirtyGroup[0]
			r.dirtyGroup = r.dirtyGroup[1:]
			g.dirty = false
			if err := r.releaseSiblings(g); err != nil {
				return err
			}
		case len(r.dirtyLoops) > 0:
			key := r.dirtyLoops[0]
			r.dirtyLoops = r.dirtyLoops[1:]
			r.closeLoop(key)
		default:
			return nil
		}
	}
	return nil
}

// applySignal delivers an edge completion signal to its target.
func (r *run) applySignal(ctx context.Context, sg signal) error {
	e := &r.plan.edges[sg.edge]
	r.event(Event{
		Type: EventEdgeClosed, Edge: e.edge.ID, Scope: sg.scope, Node: r.nodeID(e.to),
		Live: r.edgeLive[edgeKey(sg)] > 0,
	})
	key := nodeKey{e.to, sg.scope}
	n := &r.plan.nodes[e.to]
	if n.join != nil {
		js := r.joinOf(key)
		return r.applyJoin(ctx, key, n, js, js.signal(e.slot), nil)
	}
	r.nodeOf(key).signaled[e.slot] = true
	r.checkComplete(key)
	return nil
}

// releaseSiblings makes the next deferred sibling of a SEQUENTIAL fork
// runnable once every token descending from the previous one is consumed,
// completed, waiting or terminal (ADR-0010 section 5).
func (r *run) releaseSiblings(g *seqGroup) error {
	for g.cursor+1 < len(g.siblings) && r.forkLive[g.keys[g.cursor]] <= 0 {
		g.cursor++
		next := g.siblings[g.cursor]
		if next.state != TokenDeferred {
			continue
		}
		if err := r.setToken(next, TokenRunnable); err != nil {
			return err
		}
		r.queue = append(r.queue, next)
	}
	return nil
}

// stepLoop activates a loop control (ADR-0010 section 6). A token arriving by
// the entry edge starts iteration 0; a token returning by a back edge starts
// the next iteration, and exceeding maxIterations raises LoopLimitExceeded.
// The iteration frame is appended to the token's scope path, and the
// control's own node execution belongs to that iteration.
func (r *run) stepLoop(t *token, n *planNode) (*token, error) {
	back := t.inEdge >= 0 && r.plan.edges[t.inEdge].back
	outer, iteration := t.scope, 0
	if back {
		var frame ScopeFrame
		var ok bool
		outer, frame, ok = t.scope.Pop()
		if !ok || frame.Kind != ScopeLoop || frame.Node != n.node.ID {
			return nil, internalf("token %s returned to loop %q from scope %q", t.id, n.node.ID, t.scope)
		}
		iteration = frame.Index + 1
	}
	if outer.Depth() != n.nesting {
		return nil, internalf("token %s reached loop %q in scope %q, which has %d frames; the loop lies in %d loops",
			t.id, n.node.ID, outer, outer.Depth(), n.nesting)
	}
	key := nodeKey{t.node, outer}
	inst := r.loopOf(key)
	if !back {
		if inst.entered {
			return nil, r.failLoop(t, n, outer, CodeUnsupported,
				"a second token entered the loop in the same scope; an uncontrolled merge before the loop entry is not supported")
		}
		inst.entered = true
	}
	if iteration >= n.maxIterations {
		r.event(Event{Type: EventLoopLimitExceeded, Token: t.id, Node: n.node.ID, Scope: outer, Iteration: iteration})
		return nil, r.failLoop(t, n, outer, CodeLoopLimitExceeded,
			fmt.Sprintf("the loop would start iteration %d; maxIterations is %d", iteration, n.maxIterations))
	}
	scope := outer.Push(LoopFrame(n.node.ID, iteration))
	r.moveScope(t, scope)
	r.event(Event{Type: EventLoopIterationStarted, Token: t.id, Node: n.node.ID, Scope: scope, Iteration: iteration})
	ne, err := r.beginNode(t, n, scope)
	if err != nil {
		return nil, err
	}
	if err := r.finishNode(ne, NodeSucceeded, t); err != nil {
		return nil, err
	}
	ems := r.emitEdges(n.out, t.payload)
	kept, err := r.route(t, n, ne.id, scope, key, !back, ems)
	if err != nil {
		return nil, err
	}
	// The control activates once per iteration, so its edges into the body
	// complete in the iteration's scope as soon as the token is delivered.
	r.completeOut(n, scope)
	return kept, nil
}

func (r *run) failLoop(t *token, n *planNode, scope ScopePath, code, message string) error {
	return r.fail(&Failure{Code: code, Node: n.node.ID, Scope: scope, Message: message}, t)
}

// armLoop marks that no more tokens will enter a loop instance.
func (r *run) armLoop(key nodeKey) {
	r.loopOf(key).armed = true
	r.dirtyLoops = append(r.dirtyLoops, key)
}

// closeLoop completes the exit edges of a loop once its entry is complete and
// no token is left inside any iteration: the edges leaving the loop carry
// their completion signal in the scope outside the loop (ADR-0010 section 2:
// exactly one signal per edge and scope, after its live tokens).
func (r *run) closeLoop(key nodeKey) {
	inst := r.loops[key]
	if inst == nil || inst.closed || !inst.armed || inst.live > 0 {
		return
	}
	inst.closed = true
	for _, ei := range r.plan.nodes[key.node].closing {
		r.signals = append(r.signals, signal{edge: ei, scope: key.scope})
	}
}
