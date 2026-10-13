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
	return t
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
	if !next.isActive() {
		r.untrack(t)
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

func (r *run) track(t *token) {
	if t.tracked {
		return
	}
	t.tracked = true
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
	t.scope = scope
	for _, k := range r.instances(scope) {
		r.loopOf(k).live++
	}
}
