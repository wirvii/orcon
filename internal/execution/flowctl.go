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
	arrival := joinArrival{Slot: e.slot, Token: t.id, Payload: t.payload, Lineage: slices.Clone(t.lineage)}
	out := js.arrive(arrival)
	if js.letFinish {
		r.joinRound(key, n, js, arrival)
	}
	if err := r.setToken(t, TokenConsumed); err != nil {
		return err
	}
	if js.letFinish {
		return r.advanceRounds(ctx, key, n, js)
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
		r.event(Event{Type: EventJoinFailed, Node: n.node.ID, Scope: key.scope, Detail: string(js.mode), Round: 1})
		return r.fail(&Failure{
			Code: CodeJoinRequirementNotMet, Origin: FailureEngine, Node: n.node.ID, Scope: key.scope,
			Message: fmt.Sprintf("an incoming edge of the %s join completed with no live token", js.mode),
		}, nil)
	case joinEmpty:
		r.nodeOf(key).canceled = js.anyCanceled()
		r.checkComplete(key)
	case joinWaiting:
		if js.letFinish {
			// An edge closing can complete a join whose rounds all reset.
			r.checkComplete(key)
		}
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
	if r.abandoned(js) {
		// Every live arrival descends from a split whose branches were
		// canceled: this join is inside the abandoned region.
		js.fired, js.empty = false, true
		r.nodeOf(key).canceled = true
		r.checkComplete(key)
		return nil
	}
	var canceled []TokenID
	if js.mode == flow.JoinFirst && trigger != nil && n.splitFor >= 0 {
		if split, ok := correspondingSplit(trigger.lineage, r.plan.nodes[n.splitFor].node.ID); ok {
			var err error
			canceled, err = r.cancelBranches(ctx, split, key.node)
			if err != nil {
				return err
			}
		}
	}
	cont := r.spawn(contID, TokenRunnable, key.node, -1, key.scope, continuationLineage(js.combined(), r.splitNode(n)), ref)
	cont.passed = true
	r.nodeOf(key).inflight++
	r.queue = append(r.queue, cont)
	r.event(Event{Type: EventJoinFired, Token: contID, Node: n.node.ID, Scope: key.scope, Detail: string(js.mode), Round: 1, Tokens: canceled})
	return nil
}

// correspondingSplit finds, in the fork lineage of the first live token, the
// last entry of the split node the plan computed for the join (the nearest
// split that dominates it): that entry names the split instance to cancel.
func correspondingSplit(lineage []ForkRef, split flow.NodeID) (NodeExecutionID, bool) {
	for i := len(lineage) - 1; i >= 0; i-- {
		if lineage[i].SplitNode == split {
			return lineage[i].SplitNodeExecution, true
		}
	}
	return "", false
}

// abandoned reports whether every live arrival of a join descends from a split
// whose branches a discriminator canceled.
func (r *run) abandoned(js *joinState) bool {
	if len(js.arrivals) == 0 || len(r.canceledSplits) == 0 {
		return false
	}
	for _, a := range js.arrivals {
		if !slices.ContainsFunc(a.Lineage, func(f ForkRef) bool { return r.canceledSplits[f.SplitNodeExecution] }) {
			return false
		}
	}
	return true
}

// cancelBranches cancels the tokens descending from a split execution that
// have not reached the join: the discriminator semantics of ADR-0010
// section 3. Tokens already at the join are absorbed as later arrivals.
//
// Every canceled token closes, with no live token, the edges it still owed
// (ADR-0010 section 2: one signal per edge and scope), so a branch that also
// feeds a join outside the block does not leave it waiting. The signals are
// marked canceled: a join receiving one never fails because of it.
func (r *run) cancelBranches(ctx context.Context, split NodeExecutionID, join int) ([]TokenID, error) {
	r.canceledSplits[split] = true
	var canceled []TokenID
	var keys []nodeKey
	for _, c := range r.order {
		if !c.state.isActive() || c.node == join || !hasSplit(c.lineage, split) {
			continue
		}
		key := r.retract(c)
		for _, k := range r.instances(c.scope) {
			r.loopOf(k).canceled = true // its exit edges close as canceled
		}
		if err := r.setToken(c, TokenCanceled); err != nil {
			return nil, err
		}
		canceled = append(canceled, c.id)
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		n := &r.plan.nodes[key.node]
		r.nodeOf(key).canceled = true
		if n.join != nil {
			if js := r.joins[key]; js != nil {
				if err := r.applyJoin(ctx, key, n, js, js.decide(), nil); err != nil {
					return nil, err
				}
			}
			continue
		}
		r.checkComplete(key)
	}
	return canceled, nil
}

func hasSplit(lineage []ForkRef, split NodeExecutionID) bool {
	return slices.ContainsFunc(lineage, func(f ForkRef) bool { return f.SplitNodeExecution == split })
}

// retract undoes the delivery bookkeeping of a token canceled before it was
// processed at its node, and returns the tracker of that node.
func (r *run) retract(t *token) nodeKey {
	key := nodeKey{t.node, t.scope}
	n := &r.plan.nodes[t.node]
	switch {
	case t.passed:
		r.nodeOf(key).inflight--
	case t.inEdge < 0 || r.plan.edges[t.inEdge].back:
	case n.join != nil:
		edge := &r.joinOf(key).edges[r.plan.edges[t.inEdge].slot]
		edge.Sent--
		edge.Canceled = true
	default:
		r.nodeOf(key).inflight--
	}
	return key
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
		if js == nil || !js.done() {
			return
		}
	} else if !ns.allSignaled() {
		return
	}
	ns.completed = true
	if n.kind == kindLoop {
		r.armLoop(key, ns.canceled)
		return
	}
	r.completeOut(n, key.scope, ns.canceled)
}

// completeOut queues the completion signal of every outgoing edge of a node
// that completed in a scope. Back edges are excluded from completion
// accounting; exit edges of a loop complete when the loop's instance closes.
func (r *run) completeOut(n *planNode, scope ScopePath, canceled bool) {
	for _, ei := range n.out {
		e := &r.plan.edges[ei]
		if e.back || len(e.exits) > 0 {
			continue
		}
		r.signals = append(r.signals, signal{edgeKey: edgeKey{ei, scope}, canceled: canceled})
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
		case len(r.dirtyJoins) > 0:
			key := r.dirtyJoins[0]
			r.dirtyJoins = r.dirtyJoins[1:]
			if js := r.joins[key]; js != nil {
				if err := r.advanceRounds(ctx, key, &r.plan.nodes[key.node], js); err != nil {
					return err
				}
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
		Live: r.edgeLive[sg.edgeKey] > 0,
	})
	key := nodeKey{e.to, sg.scope}
	n := &r.plan.nodes[e.to]
	if n.join != nil {
		js := r.joinOf(key)
		return r.applyJoin(ctx, key, n, js, js.signal(e.slot, sg.canceled), nil)
	}
	ns := r.nodeOf(key)
	ns.signaled[e.slot] = true
	ns.canceled = ns.canceled || sg.canceled
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

// stepLoop activates a loop control (ADR-0010 section 6, amendment 9). Every
// token arriving by the entry edge starts its own loop instance at iteration 0,
// named by that token in the scope path, with its own iteration budget; a token
// returning by a back edge starts the next iteration of its instance, and
// exceeding maxIterations raises LoopLimitExceeded. The iteration frame is
// appended to the token's scope path, and the control's own node execution
// belongs to that iteration.
func (r *run) stepLoop(t *token, n *planNode) (*token, error) {
	back := t.inEdge >= 0 && r.plan.edges[t.inEdge].back
	outer, entry, iteration := t.scope, t.id, 0
	if back {
		var frame ScopeFrame
		var ok bool
		outer, frame, ok = t.scope.Pop()
		if !ok || frame.Kind != ScopeLoop || frame.Node != n.node.ID {
			return nil, internalf("token %s returned to loop %q from scope %q", t.id, n.node.ID, t.scope)
		}
		entry, iteration = frame.Entry, frame.Index+1
	}
	if outer.Depth() != n.nesting {
		return nil, internalf("token %s reached loop %q in scope %q, which has %d frames; the loop lies in %d loops",
			t.id, n.node.ID, outer, outer.Depth(), n.nesting)
	}
	key := nodeKey{t.node, outer}
	if iteration >= n.maxIterations {
		r.event(Event{Type: EventLoopLimitExceeded, Token: t.id, Node: n.node.ID, Scope: outer, Iteration: iteration})
		return nil, r.failLoop(t, n, outer, CodeLoopLimitExceeded,
			fmt.Sprintf("the loop would start iteration %d; maxIterations is %d", iteration, n.maxIterations))
	}
	scope := outer.Push(LoopFrame(n.node.ID, entry, iteration))
	// Two tokens returning in the same iteration of one instance would share a
	// scope: ADR-0010 names an instance by its entry token only, so the
	// situation has no defined meaning yet and stops the run instead of mixing
	// the trackers of the two.
	if r.iterations[scope] {
		return nil, fmt.Errorf("%w: two tokens returned to loop %q in the same iteration (%q) of one loop instance",
			ErrUnsupported, n.node.ID, scope)
	}
	r.iterations[scope] = true
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
	r.completeOut(n, scope, false)
	return kept, nil
}

func (r *run) failLoop(t *token, n *planNode, scope ScopePath, code, message string) error {
	return r.fail(&Failure{Code: code, Origin: FailureEngine, Node: n.node.ID, Scope: scope, Message: message}, t)
}

// armLoop marks that no more tokens will enter a loop instance.
func (r *run) armLoop(key nodeKey, canceled bool) {
	r.loopOf(key).armed = true
	r.loopOf(key).canceled = canceled
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
		r.signals = append(r.signals, signal{edgeKey: edgeKey{ei, key.scope}, canceled: inst.canceled})
	}
}

// joinRound files an arrival at a LET_FINISH join under the round of the split
// instance in the arriving token's fork lineage (a token that carries none is
// a round of its own), and registers the join to be looked at when a
// descendant of that instance ends.
func (r *run) joinRound(key nodeKey, n *planNode, js *joinState, a joinArrival) {
	instance := NodeExecutionID("solo:" + string(a.Token))
	if n.splitFor >= 0 {
		if split, ok := correspondingSplit(a.Lineage, r.plan.nodes[n.splitFor].node.ID); ok {
			instance = split
		}
	}
	rd, created := js.round(instance)
	rd.arrivals = append(rd.arrivals, a)
	if created {
		r.roundJoins[instance] = append(r.roundJoins[instance], key)
	}
}

// advanceRounds settles the rounds of a LET_FINISH join. A round whose split
// instance has no descendant left that can reach the join resets; the first
// round that has arrivals, is not fired and follows only reset rounds fires on
// its first arrival, in the same step that reset the one before it (the
// blocking discriminator, WCP 28). A fired round that has not reset blocks the
// rounds after it.
func (r *run) advanceRounds(ctx context.Context, key nodeKey, n *planNode, js *joinState) error {
	for changed := true; changed; {
		changed = false
		blocked := false
		for _, rd := range js.rounds {
			switch {
			case rd.reset:
			case rd.fired:
				if r.splitLive[rd.split] > 0 {
					blocked = true
					continue
				}
				rd.reset = true
				changed = true
				r.event(Event{
					Type: EventJoinReset, Node: n.node.ID, Scope: key.scope, Round: rd.number,
					LiveArrivals: len(rd.arrivals), Arrivals: len(rd.arrivals),
				})
			case !blocked:
				if err := r.fireRound(ctx, key, n, rd); err != nil {
					return err
				}
				blocked, changed = true, true
			}
		}
	}
	r.checkComplete(key)
	return nil
}

// fireRound creates the continuation of a round from its first arrival. The
// other branches keep running; later arrivals of the round are consumed and
// recorded without a continuation.
func (r *run) fireRound(ctx context.Context, key nodeKey, n *planNode, rd *joinRound) error {
	first := rd.arrivals[0]
	batch, err := r.load(ctx, first.Payload)
	if err != nil {
		return err
	}
	contID := roundTokenID(r.req.Execution, n.node.ID, key.scope, rd.split)
	ref := joinedRef(contID)
	if err := r.payloads.Put(ctx, ref, batch); err != nil {
		return fmt.Errorf("store input of join %q round %d: %w", n.node.ID, rd.number, err)
	}
	cont := r.spawn(contID, TokenRunnable, key.node, -1, key.scope, continuationLineage([]joinArrival{first}, r.splitNode(n)), ref)
	cont.passed = true
	rd.fired = true
	r.nodeOf(key).inflight++
	r.queue = append(r.queue, cont)
	r.event(Event{Type: EventJoinFired, Token: contID, Node: n.node.ID, Scope: key.scope, Detail: string(flow.JoinFirst), Round: rd.number})
	return nil
}

// splitNode returns the corresponding split node of a join, or "" when it has
// none.
func (r *run) splitNode(n *planNode) flow.NodeID {
	if n.splitFor < 0 {
		return ""
	}
	return r.plan.nodes[n.splitFor].node.ID
}
