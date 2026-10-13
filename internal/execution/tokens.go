// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"slices"
)

// spawn creates a token. The initial state must be one a token may be
// created in. The token counts toward the loop and branch counters while it
// is active.
func (r *run) spawn(id TokenID, state TokenState, node, inEdge int, scope ScopePath, lineage []ForkRef, payload PayloadRef) *token {
	t := &token{
		id:      id,
		state:   state,
		node:    node,
		inEdge:  inEdge,
		scope:   scope,
		lineage: slices.Clone(lineage),
		payload: payload,
	}
	r.tokens[id] = t
	r.order = append(r.order, t)
	r.track(t)
	r.holdRounds(t)
	return t
}

// holdRounds counts a token as a live descendant of every split instance in its
// lineage, for the rounds of LET_FINISH joins. Unlike the counters of
// SEQUENTIAL branch order, which treat a WAITING token as done (ADR-0010
// section 5), a round stays open while a descendant is WAITING or SUSPENDED
// (amendment 9, B-2): it holds until the token reaches a terminal state.
func (r *run) holdRounds(t *token) {
	t.roundHeld = true
	for _, f := range t.lineage {
		r.splitLive[f.SplitNodeExecution]++
	}
}

// releaseRounds ends the hold of a token that reached a terminal state and
// queues the joins whose rounds may now reset.
func (r *run) releaseRounds(t *token) {
	if !t.roundHeld {
		return
	}
	t.roundHeld = false
	for _, f := range t.lineage {
		r.splitLive[f.SplitNodeExecution]--
		r.dirtyJoins = append(r.dirtyJoins, r.roundJoins[f.SplitNodeExecution]...)
	}
}

// setToken moves a token along its state table. A token that leaves the
// active states stops counting toward the counters; that may close a loop
// instance or release a deferred sibling, which drain settles.
func (r *run) setToken(t *token, to TokenState) error {
	next, err := t.state.Transition(to)
	if err != nil {
		return err
	}
	t.state = next
	switch {
	case !next.isActive():
		r.untrack(t)
	case !t.tracked:
		r.track(t) // a WAITING token that runs again counts again
	}
	if next.IsTerminal() {
		r.releaseRounds(t)
	}
	return nil
}

// instances lists the loop instances a scope path lies in, innermost first:
// each frame belongs to the instance of its loop control entered in the
// scope that precedes the frame.
func (r *run) instances(scope ScopePath) []nodeKey {
	var keys []nodeKey
	for scope != "" {
		parent, frame, ok := scope.Pop()
		if !ok {
			break
		}
		if frame.Kind == ScopeLoop {
			if idx, known := r.plan.index[frame.Node]; known {
				keys = append(keys, nodeKey{idx, parent})
			}
		}
		scope = parent
	}
	return keys
}

// countScopes adds delta to the live count of every scope path the given one
// contains, itself included. When an iteration scope that has collected returns
// empties, the next iteration is due.
func (r *run) countScopes(scope ScopePath, delta int) {
	for scope != "" {
		r.scopeLive[scope] += delta
		if delta < 0 && r.scopeLive[scope] == 0 {
			if rec := r.returns[scope]; rec != nil && !rec.started {
				r.dirtyIters = append(r.dirtyIters, scope)
			}
		}
		parent, _, ok := scope.Pop()
		if !ok {
			return
		}
		scope = parent
	}
}

func (r *run) track(t *token) {
	if t.tracked {
		return
	}
	t.tracked = true
	r.countScopes(t.scope, 1)
	for _, k := range r.instances(t.scope) {
		r.loopOf(k).live++
	}
	for _, f := range t.lineage {
		r.forkLive[forkKey{f.SplitNodeExecution, f.Branch}]++
	}
}

func (r *run) untrack(t *token) {
	if !t.tracked {
		return
	}
	t.tracked = false
	r.countScopes(t.scope, -1)
	for _, k := range r.instances(t.scope) {
		r.loopOf(k).live--
		r.dirtyLoops = append(r.dirtyLoops, k)
	}
	for _, f := range t.lineage {
		fk := forkKey{f.SplitNodeExecution, f.Branch}
		r.forkLive[fk]--
		if g := r.groups[f.SplitNodeExecution]; g != nil && !g.dirty {
			g.dirty = true
			r.dirtyGroup = append(r.dirtyGroup, g)
		}
	}
}

// moveScope puts a token in another scope and keeps the loop counters right.
func (r *run) moveScope(t *token, scope ScopePath) {
	if t.scope == scope {
		return
	}
	if !t.tracked {
		t.scope = scope
		return
	}
	for _, k := range r.instances(t.scope) {
		r.loopOf(k).live--
		r.dirtyLoops = append(r.dirtyLoops, k)
	}
	old := t.scope
	t.scope = scope
	r.countScopes(scope, 1)
	r.countScopes(old, -1)
	for _, k := range r.instances(scope) {
		r.loopOf(k).live++
	}
}
