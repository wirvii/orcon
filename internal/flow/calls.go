// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"slices"
	"strings"
)

// CallEdge is a call from one flow to another, as ADR-0010 section 7 builds
// the call graph: one edge per static target and one per allowed target of a
// dynamic call.
type CallEdge struct {
	From, To ID
	// Intentional marks a call that may close a cycle on purpose
	// (intentionalRecursion); the runtime depth guard bounds it.
	Intentional bool
}

// CallEdges lists the calls the document can make: static targets and the
// allowedTargets of dynamic calls. Dynamic calls without allowedTargets
// contribute nothing; Validate warns that they cannot be analyzed.
func (d *Document) CallEdges() []CallEdge {
	var edges []CallEdge
	for i := range d.Nodes {
		n := &d.Nodes[i]
		c := n.Call
		if c == nil {
			continue
		}
		intentional := c.IntentionalRecursion != nil
		if c.Target != "" && !strings.HasPrefix(c.Target, "=") {
			edges = append(edges, CallEdge{From: d.ID, To: ID(c.Target), Intentional: intentional})
		}
		for _, t := range c.AllowedTargets {
			edges = append(edges, CallEdge{From: d.ID, To: t, Intentional: intentional})
		}
	}
	return edges
}

// calls runs the single-document part of rule V11. The cross-flow check needs
// the other flows of the project and is [CheckCallGraph].
func (v *validator) calls() {
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		if n.Element != ElementCallActivity {
			continue
		}
		base := ptr("nodes", i)
		c := n.Call
		if c == nil {
			v.r.errorf(CodeV11, base, "call activity %q has no call block", n.ID)
			continue
		}
		dynamic := strings.HasPrefix(c.Target, "=")
		if dynamic && len(c.AllowedTargets) == 0 {
			v.r.warnf(CodeV11, base+"/call", "the target of %q is an expression without allowedTargets, so the call graph cannot be analyzed", n.ID)
		}
		if c.IntentionalRecursion != nil {
			continue
		}
		if ID(c.Target) == v.doc.ID || slices.Contains(c.AllowedTargets, v.doc.ID) {
			v.r.errorf(CodeV11, base+"/call", "call activity %q calls its own flow %q; mark it with intentionalRecursion and a maxDepth or remove the call", n.ID, v.doc.ID)
		}
	}
}

// CheckCallGraph implements the publication guard of ADR-0010 section 7. The
// edges are those of the active published version (and pinned versions) of
// every flow in the project plus the candidate. Strongly connected
// components identify direct and indirect cycles of any length; an edge that
// lies on a cycle and is not marked as intentional recursion is an error that
// lists the full path.
func CheckCallGraph(edges []CallEdge) Report {
	var r collector
	ids := map[ID]int{}
	var names []ID
	id := func(f ID) int {
		i, ok := ids[f]
		if !ok {
			i = len(names)
			ids[f] = i
			names = append(names, f)
		}
		return i
	}
	for _, e := range edges {
		id(e.From)
		id(e.To)
	}
	adj := make([][]int, len(names))
	for _, e := range edges {
		adj[id(e.From)] = append(adj[id(e.From)], id(e.To))
	}
	comp := make([]int, len(names))
	for ci, c := range stronglyConnected(adj) {
		for _, x := range c {
			comp[x] = ci
		}
	}
	reported := map[[2]ID]bool{}
	for _, e := range edges {
		from, to := id(e.From), id(e.To)
		if e.Intentional || comp[from] != comp[to] || reported[[2]ID{e.From, e.To}] {
			continue
		}
		reported[[2]ID{e.From, e.To}] = true
		path := cyclePath(adj, comp, names, from, to)
		r.errorf(CodeV11, "", "flow %q calls %q and that call closes a cycle that is not marked as intentional recursion: %s", e.From, e.To, path)
	}
	return r.report()
}

// cyclePath renders the shortest cycle through the edge from -> to: the edge
// followed by the shortest way back from "to" to "from" inside the component.
func cyclePath(adj [][]int, comp []int, names []ID, from, to int) string {
	prev := map[int]int{to: -1}
	queue := []int{to}
	for len(queue) > 0 && !slices.Contains(queue, from) {
		var next []int
		for _, x := range queue {
			for _, y := range adj[x] {
				if _, seen := prev[y]; !seen && comp[y] == comp[from] {
					prev[y] = x
					next = append(next, y)
				}
			}
		}
		queue = next
	}
	back := []int{}
	for x := from; x != -1; x = prev[x] {
		back = append(back, x)
		if x == to {
			break
		}
	}
	slices.Reverse(back) // to ... from
	parts := []string{string(names[from])}
	for _, x := range back {
		parts = append(parts, string(names[x]))
	}
	return strings.Join(parts, " -> ")
}
