// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"slices"
)

// data runs the data-access rules of ADR-0008 section 4: V8 and V9.
func (v *validator) data(g *graph) {
	v.reads(g)
	v.variableReads()
	v.concurrentWriters(g)
}

// reads implements V8. A node X may read the output of node Y only if Y
// dominates X (every path to X passes through Y), or Y lies on a branch that
// a join upstream of X merged. A read of a node that may have run
// concurrently with X, or that is not upstream at all, is an error.
func (v *validator) reads(g *graph) {
	names := make(map[string]int, len(v.doc.Nodes))
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		names[n.Name] = i
	}
	reachable := g.reachableFromStarts()
	for x := range v.doc.Nodes {
		n := &v.doc.Nodes[x]
		if !reachable[x] {
			continue // V1 already reports it; no ancestor relation exists
		}
		outgoing := make([]Edge, 0, len(g.outEdges[x]))
		for _, e := range v.doc.Edges {
			if e.From.Node == n.ID {
				outgoing = append(outgoing, e)
			}
		}
		seen := map[string]bool{}
		for _, ref := range nodeReads(n, outgoing) {
			if seen[ref.Name] {
				continue
			}
			seen[ref.Name] = true
			v.checkRead(g, x, ref, names)
		}
	}
}

func (v *validator) checkRead(g *graph, x int, ref nodeRef, names map[string]int) {
	path := ptr("nodes", x)
	reader := v.doc.Nodes[x]
	y, ok := names[ref.Name]
	if !ok {
		v.r.errorf(CodeV8, path, "node %q reads $('%s'), but no node has that name", reader.ID, ref.Name)
		return
	}
	switch {
	case y == x:
		v.r.errorf(CodeV8, path, "node %q reads its own output with $('%s'); a node reads its ancestors only", reader.ID, ref.Name)
	case g.dominators()[y] == -1:
		v.r.errorf(CodeV8, path, "node %q reads $('%s'), which no start event can reach", reader.ID, ref.Name)
	case g.dominates(y, x):
		// Every path to the reader passes through the read node.
	default:
		v.readThroughJoin(g, x, y, ref)
	}
}

// readThroughJoin handles a read of a node that does not dominate the reader:
// legal only when a join that dominates the reader merged the read node's
// branch.
func (v *validator) readThroughJoin(g *graph, x, y int, ref nodeRef) {
	path := ptr("nodes", x)
	reader, read := v.doc.Nodes[x], v.doc.Nodes[y]
	for j := range v.doc.Nodes {
		n := &v.doc.Nodes[j]
		if n.Join == nil || !g.dominates(j, x) || !g.reaches(y, j) {
			continue
		}
		if n.Join.Mode != JoinAll && !ref.Guarded {
			v.r.warnf(CodeV8, path,
				"node %q reads $('%s'), which may not have run because join %q is %s; test $('%s').isExecuted first",
				reader.ID, ref.Name, n.ID, n.Join.Mode, ref.Name)
		}
		return
	}
	if g.reaches(y, x) {
		v.r.errorf(CodeV8, path,
			"node %q reads $('%s'), but %q is on one of several branches that reach it without a join; add a join that merges them or read an ancestor common to all paths",
			reader.ID, ref.Name, read.ID)
		return
	}
	v.r.errorf(CodeV8, path,
		"node %q reads $('%s'), but %q is not an ancestor of it (it is on a sibling branch or downstream)",
		reader.ID, ref.Name, read.ID)
}

// concurrentWriters implements V9: a variable without an accumulator may not
// have writers that can run at the same time (ADR-0008 section 4.3).
func (v *validator) concurrentWriters(g *graph) {
	plain := map[string]bool{}
	for _, vr := range v.doc.Variables {
		plain[vr.Name] = vr.Accumulator == ""
	}
	writers := map[string][]int{}
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		for _, w := range v.writers(n) {
			if plain[w] && !slices.Contains(writers[w], i) {
				writers[w] = append(writers[w], i)
			}
		}
	}
	for _, vr := range v.doc.Variables {
		list := writers[vr.Name]
		for a, wa := range list {
			if v.selfConcurrent(g, wa) {
				v.r.errorf(CodeV9, ptr("nodes", wa),
					"node %q writes variable %q and can run concurrently with itself (parallel multi-instance or an uncontrolled merge); give the variable an accumulator or make the writes sequential",
					v.doc.Nodes[wa].ID, vr.Name)
			}
			for _, wb := range list[a+1:] {
				if g.mayRunTogether(wa, wb) {
					v.r.errorf(CodeV9, ptr("nodes", wb),
						"nodes %q and %q both write variable %q and may run at the same time on different branches; give the variable an accumulator or write it from one place",
						v.doc.Nodes[wa].ID, v.doc.Nodes[wb].ID, vr.Name)
				}
			}
		}
	}
}

// selfConcurrent reports whether node w can have two activations at once: a
// parallel multi-instance body with concurrency above one, or a node that two
// concurrent branches reach without a join.
func (v *validator) selfConcurrent(g *graph, w int) bool {
	if mi := v.doc.Nodes[w].MultiInstance; mi != nil && !mi.Sequential {
		if mi.Concurrency == nil || mi.Concurrency.Keyword != "" || mi.Concurrency.N > 1 {
			return true
		}
	}
	return g.mayRunTogether(w, w)
}

// mayRunTogether reports whether activations of nodes a and b can overlap:
// some node releases tokens on two different paths (see [graph.fanOut]), one
// of which reaches a and the other b before any join. When a == b this means
// the node is reached by two concurrent paths, the "uncontrolled merge" case.
func (g *graph) mayRunTogether(a, b int) bool {
	for s := range g.doc.Nodes {
		targets := g.fanOut(s)
		for i, p := range targets {
			for j, q := range targets {
				if p == q || (a == b && j < i) {
					continue
				}
				if g.reachNoJoin(p)[a] && g.reachNoJoin(q)[b] {
					return true
				}
			}
		}
	}
	return false
}

// fanOut lists the distinct nodes that node i can start at the same time. An
// ordinary node starts the targets of all its outgoing edges (item routing
// nodes may fill several ports with different items); XOR and event-based
// gateways choose one path and start none together. A non-interrupting
// boundary event is another branch of its host: the host's continuation and
// the boundary path run side by side, whereas an interrupting boundary event
// replaces the continuation and is not concurrent with it.
func (g *graph) fanOut(i int) []int {
	n := &g.doc.Nodes[i]
	if n.Element == ElementExclusiveGateway || n.Element == ElementEventBasedGateway {
		return nil
	}
	targets := make([]int, 0, len(g.outEdges[i])+len(g.nonInterrupting[i]))
	for _, ei := range g.outEdges[i] {
		targets = append(targets, g.index[g.doc.Edges[ei].To])
	}
	targets = append(targets, g.nonInterrupting[i]...)
	targets = distinct(targets)
	if len(targets) < 2 {
		return nil
	}
	return targets
}

func distinct(xs []int) []int {
	out := slices.Clone(xs)
	slices.Sort(out)
	return slices.Compact(out)
}
