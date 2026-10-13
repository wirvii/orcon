// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"slices"
	"strings"
)

// CodeElement marks a node whose edges do not fit its BPMN element class.
const CodeElement = "ELEMENT"

// maxListedNodes bounds the node names quoted in one message.
const maxListedNodes = 5

// control runs the control-flow rules of ADR-0010 section 11: V1 to V7.
func (v *validator) control(g *graph) {
	v.elementShapes(g)
	v.reachability(g)
	v.cycles(g)
	v.loops()
	v.waitTimeouts(g)
	v.splitsAndJoins(g)
	v.joinOptions(g)
	v.boundaries(g)
}

// CodeV14 is rule V14, the options of a join: remaining applies to FIRST only,
// LET_FINISH needs a corresponding split, and the data modes MATCH_FIELD and
// MATCH_POSITION (and matchField) are rejected until ADR-0010 section 3 defines
// them. The rule is semantic, so the values stay in the JSON Schema.
const CodeV14 = "V14"

// CorrespondingSplits returns, for every join of a valid document, the split
// node whose branches the join brings together: the nearest node that
// dominates the join and reaches it through at least two of its outgoing edges
// (ADR-0010 section 3). A FIRST join discriminates the branches of that split;
// any join drops it from the fork lineage of its continuation. A join without
// such a split is absent from the result.
// Publication validation and the engine compiler both use this one function,
// so the editor, V14 and the engine agree on the split.
func CorrespondingSplits(d *Document) map[NodeID]NodeID {
	g := newGraph(d)
	out := make(map[NodeID]NodeID)
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if n.Join == nil {
			continue
		}
		if s, ok := g.correspondingSplit(i); ok {
			out[n.ID] = d.Nodes[s].ID
		}
	}
	return out
}

// correspondingSplit walks the immediate dominators of join j, nearest first.
func (g *graph) correspondingSplit(j int) (int, bool) {
	idom := g.dominators()
	root := len(g.succ)
	for x := idom[j]; x >= 0 && x != root; x = idom[x] {
		branches := 0
		for _, ei := range g.outEdges[x] {
			to := g.index[g.doc.Edges[ei].To]
			if to == j || g.reaches(to, j) {
				branches++
			}
		}
		if branches >= 2 {
			return x, true
		}
	}
	return -1, false
}

// joinOptions implements V14.
func (v *validator) joinOptions(g *graph) {
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		j := n.Join
		if j == nil {
			continue
		}
		if j.Data == JoinDataMatchField || j.Data == JoinDataMatchPosition {
			v.r.errorf(CodeV14, ptr("nodes", i, "join", "data"),
				"join data mode %s of node %q is not defined yet; only %s is supported", j.Data, n.ID, JoinDataAppend)
		}
		if j.MatchField != "" {
			v.r.errorf(CodeV14, ptr("nodes", i, "join", "matchField"),
				"matchField of node %q is not supported yet; only data %s is", n.ID, JoinDataAppend)
		}
		if j.Remaining == "" {
			continue
		}
		if j.Mode != JoinFirst {
			v.r.errorf(CodeV14, ptr("nodes", i, "join", "remaining"),
				"remaining applies to FIRST joins only, but node %q has mode %s", n.ID, j.Mode)
			continue
		}
		if j.Remaining == RemainingLetFinish {
			if _, ok := g.correspondingSplit(i); !ok {
				v.r.errorf(CodeV14, ptr("nodes", i, "join", "remaining"),
					"let-finish needs a split that dominates the join: no node dominates %q and reaches it through two outgoing edges", n.ID)
			}
		}
	}
}

// elementShapes checks the edge counts that define the BPMN element classes.
func (v *validator) elementShapes(g *graph) {
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		base := ptr("nodes", i)
		hasIn := len(g.inEdges[i]) > 0 || v.hasBackEdgeInto(n.ID)
		switch n.Element {
		case ElementStartEvent:
			if hasIn {
				v.r.errorf(CodeElement, base, "start event %q has incoming edges; a flow starts here", n.ID)
			}
		case ElementEndEvent:
			if len(g.outEdges[i]) > 0 {
				v.r.errorf(CodeElement, base, "end event %q has outgoing edges; a flow ends here", n.ID)
			}
		case ElementBoundaryEvent:
			if hasIn {
				v.r.errorf(CodeElement, base, "boundary event %q has incoming edges; it is triggered by the activity it is attached to", n.ID)
			}
		case ElementTask, ElementServiceTask, ElementScriptTask, ElementUserTask, ElementReceiveTask,
			ElementSendTask, ElementCallActivity, ElementExclusiveGateway, ElementInclusiveGateway,
			ElementParallelGateway, ElementEventBasedGateway, ElementComplexGateway,
			ElementIntermediateCatchEvent, ElementIntermediateThrowEvent:
		default:
			// An element outside the closed set is reported by the enum check.
		}
	}
}

func (v *validator) hasBackEdgeInto(id NodeID) bool {
	return slices.ContainsFunc(v.doc.Edges, func(e Edge) bool { return e.Kind == EdgeBack && e.To == id })
}

// reachability implements V1: every node is reachable from a start event.
func (v *validator) reachability(g *graph) {
	if len(v.doc.Nodes) == 0 {
		return
	}
	if len(g.starts) == 0 {
		v.r.errorf(CodeV1, "/nodes", "the flow has no startEvent, so no node can be reached")
		return
	}
	seen := g.reachableFromStarts()
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		if !seen[i] {
			v.r.errorf(CodeV1, ptr("nodes", i), "node %q (%s) cannot be reached from any start event", n.ID, n.Name)
		}
	}
}

// cycles implements V2: a cycle must pass through a loop control, so a cycle
// made of forward edges alone is an error, and every back edge must return to
// an orcon.loop node.
func (v *validator) cycles(g *graph) {
	for _, comp := range stronglyConnected(g.succ) {
		if len(comp) == 1 && !slices.Contains(g.succ[comp[0]], comp[0]) {
			continue
		}
		names := make([]string, 0, len(comp))
		for _, x := range comp {
			names = append(names, string(v.doc.Nodes[x].ID))
		}
		slices.Sort(names)
		v.r.errorf(CodeV2, ptr("nodes", slices.Min(comp)),
			"nodes %s form a cycle of forward edges; a cycle needs an orcon.loop control and a back edge (kind BACK) into it",
			listNames(names))
	}
	for i, e := range v.doc.Edges {
		if e.Kind != EdgeBack {
			continue
		}
		target, ok := g.index[e.To]
		if ok && typeName(v.doc.Nodes[target].Type) != typeLoop {
			v.r.errorf(CodeV2, ptr("edges", i, "kind"), "back edge %q returns to %q, which is not an %s node", e.ID, e.To, typeLoop)
		}
	}
}

func listNames(names []string) string {
	if len(names) <= maxListedNodes {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:maxListedNodes], ", ") + ", ..."
}

// stronglyConnected returns the strongly connected components of a graph
// given by adjacency lists (Tarjan, iterative so deep graphs cannot exhaust
// the stack).
func stronglyConnected(adj [][]int) [][]int {
	n := len(adj)
	const unvisited = -1
	index := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i] = unvisited
	}
	var (
		comps   [][]int
		stack   []int
		counter int
	)
	type frame struct{ node, next int }
	for start := range n {
		if index[start] != unvisited {
			continue
		}
		work := []frame{{node: start}}
		index[start], low[start] = counter, counter
		counter++
		stack = append(stack, start)
		onStack[start] = true
		for len(work) > 0 {
			f := &work[len(work)-1]
			if f.next < len(adj[f.node]) {
				w := adj[f.node][f.next]
				f.next++
				switch {
				case index[w] == unvisited:
					index[w], low[w] = counter, counter
					counter++
					stack = append(stack, w)
					onStack[w] = true
					work = append(work, frame{node: w})
				case onStack[w]:
					low[f.node] = min(low[f.node], index[w])
				}
				continue
			}
			node := f.node
			work = work[:len(work)-1]
			if len(work) > 0 {
				parent := work[len(work)-1].node
				low[parent] = min(low[parent], low[node])
			}
			if low[node] == index[node] {
				var comp []int
				for {
					top := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[top] = false
					comp = append(comp, top)
					if top == node {
						break
					}
				}
				comps = append(comps, comp)
			}
		}
	}
	return comps
}

// loops implements V3: a loop control sets maxIterations of at least 1.
func (v *validator) loops() {
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		if typeName(n.Type) != typeLoop {
			continue
		}
		switch {
		case n.Loop == nil:
			v.r.errorf(CodeV3, ptr("nodes", i), "loop control %q has no loop block; maxIterations is mandatory", n.ID)
		case n.Loop.MaxIterations < 1:
			v.r.errorf(CodeV3, ptr("nodes", i, "loop", "maxIterations"), "loop control %q needs maxIterations of at least 1", n.ID)
		}
	}
}

// waitTimeouts implements V4: signal and human waits need a timeout and an
// outgoing edge on the timeout port.
func (v *validator) waitTimeouts(g *graph) {
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		if n.Wait == nil || (n.Wait.Kind != WaitSignal && n.Wait.Kind != WaitHuman) {
			continue
		}
		if n.Wait.Timeout == "" {
			v.r.errorf(CodeV4, ptr("nodes", i, "wait", "timeout"), "%s wait of node %q needs a timeout duration", n.Wait.Kind, n.ID)
		}
		hasPath := slices.ContainsFunc(g.outEdges[i], func(ei int) bool { return v.doc.Edges[ei].From.Port == PortTimeout })
		if !hasPath {
			v.r.errorf(CodeV4, ptr("nodes", i), "%s wait of node %q needs an outgoing edge on port %q for when it times out", n.Wait.Kind, n.ID, PortTimeout)
		}
	}
}

// splitsAndJoins implements V5 (a default path on XOR and OR splits) and V6
// (a join has at least two incoming edges).
func (v *validator) splitsAndJoins(g *graph) {
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		if n.Element == ElementExclusiveGateway || n.Element == ElementInclusiveGateway {
			v.defaultPath(g, i, n)
		}
		if n.Join != nil && len(g.inEdges[i]) < 2 {
			v.r.errorf(CodeV6, ptr("nodes", i, "join"), "node %q is a join but has %d incoming edges; a join needs at least 2", n.ID, len(g.inEdges[i]))
		}
	}
}

func (v *validator) defaultPath(g *graph, i int, n *Node) {
	out := g.outEdges[i]
	if len(out) < 2 {
		return
	}
	defaults := 0
	for _, ei := range out {
		if isTrue(v.doc.Edges[ei].Default) {
			defaults++
		}
	}
	switch {
	case defaults > 1:
		v.r.errorf(CodeV5, ptr("nodes", i), "gateway %q has %d default edges; at most one is allowed", n.ID, defaults)
	case defaults == 0:
		v.r.warnf(CodeV5, ptr("nodes", i), "gateway %q has no default edge and may raise NoPathTaken", n.ID)
	}
}

// boundaries implements V7: boundary events attach to activities only.
func (v *validator) boundaries(g *graph) {
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		if n.Element != ElementBoundaryEvent {
			continue
		}
		host, ok := g.index[n.AttachedTo]
		if ok && !v.doc.Nodes[host].Element.isActivity() {
			v.r.errorf(CodeV7, ptr("nodes", i, "attachedTo"), "boundary event %q is attached to %q, a %s; only activities can have boundary events",
				n.ID, n.AttachedTo, v.doc.Nodes[host].Element)
		}
	}
}
