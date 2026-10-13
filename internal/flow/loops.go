// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

// several is the upper bound that stands for "more than one live token".
const several = 2

// LoopsReturningSeveral returns the loop controls of a valid document whose
// body can deliver more than one live token to its back edges in a single
// iteration (SPEC-001 amendment 9, section C-1). For those loops the engine
// merges what returns in one iteration before it starts the next; for every
// other loop the first returning token starts the next iteration at once and no
// merge state is kept. The analysis only chooses the cheaper path, so it
// overestimates: a loop it reports may in fact return one token, never the
// other way round.
//
// It walks the body without its back edges, which is acyclic, from one token
// leaving the control, and bounds the activations of each node by 0, 1 or
// several: a node without a join block activates once per token on its incoming
// edges; a join activates once per scope, except a FIRST join that lets the
// other branches finish, which activates once per instance of its split (here:
// several); gateways and item-routing nodes pass their activations on every
// outgoing edge; a nested loop leaves several tokens on its outgoing edges.
func LoopsReturningSeveral(d *Document) map[NodeID]bool {
	index := make(map[NodeID]int, len(d.Nodes))
	for i := range d.Nodes {
		index[d.Nodes[i].ID] = i
	}
	succ := make([][]int, len(d.Nodes)) // forward edge indices
	pred := make([][]int, len(d.Nodes)) // forward predecessor nodes
	for ei, e := range d.Edges {
		if e.Kind == EdgeBack {
			continue
		}
		from, to := index[e.From.Node], index[e.To]
		succ[from] = append(succ[from], ei)
		pred[to] = append(pred[to], from)
	}
	out := make(map[NodeID]bool)
	for i := range d.Nodes {
		if typeName(d.Nodes[i].Type) == typeLoop && loopReturnsSeveral(d, index, succ, pred, i) {
			out[d.Nodes[i].ID] = true
		}
	}
	return out
}

func loopReturnsSeveral(d *Document, index map[NodeID]int, succ [][]int, pred [][]int, l int) bool {
	in := loopBody(d, index, succ, pred, l)
	// Kahn over the forward edges inside the body, starting at the control.
	indeg := make([]int, len(d.Nodes))
	for x := range in {
		if !in[x] {
			continue
		}
		for _, ei := range succ[x] {
			if to := index[d.Edges[ei].To]; in[to] {
				indeg[to]++
			}
		}
	}
	incoming := make([]int, len(d.Nodes))
	queue := []int{l}
	back := 0
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		act := activations(&d.Nodes[x], x == l, incoming[x])
		out := carried(&d.Nodes[x], x == l, act)
		for _, ei := range succ[x] {
			to := index[d.Edges[ei].To]
			if !in[to] {
				continue
			}
			incoming[to] = min(several, incoming[to]+out)
			if indeg[to]--; indeg[to] == 0 {
				queue = append(queue, to)
			}
		}
		for _, e := range d.Edges {
			if e.Kind == EdgeBack && index[e.To] == l && index[e.From.Node] == x {
				back += out
			}
		}
	}
	return back >= several
}

// loopBody marks the control of loop l and the nodes reachable from it through
// forward edges that reach the source of one of its back edges.
func loopBody(d *Document, index map[NodeID]int, succ [][]int, pred [][]int, l int) []bool {
	n := len(d.Nodes)
	fwd := make([]bool, n)
	stack := []int{l}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, ei := range succ[x] {
			if to := index[d.Edges[ei].To]; !fwd[to] {
				fwd[to] = true
				stack = append(stack, to)
			}
		}
	}
	rev := make([]bool, n)
	for _, e := range d.Edges {
		if src := index[e.From.Node]; e.Kind == EdgeBack && index[e.To] == l && src != l && !rev[src] {
			rev[src] = true
			stack = append(stack, src)
		}
	}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, p := range pred[x] {
			if p != l && !rev[p] {
				rev[p] = true
				stack = append(stack, p)
			}
		}
	}
	in := make([]bool, n)
	for x := range in {
		in[x] = x == l || (fwd[x] && rev[x])
	}
	return in
}

// activations bounds how many times a node activates from the tokens on its
// incoming edges.
func activations(n *Node, isControl bool, incoming int) int {
	switch {
	case isControl:
		return 1
	case incoming == 0:
		return 0
	case n.Join != nil && n.Join.Mode == JoinFirst && n.Join.Remaining == RemainingLetFinish:
		return several
	case n.Join != nil:
		return 1
	default:
		return min(several, incoming)
	}
}

// carried bounds the live tokens a node puts on each outgoing edge.
func carried(n *Node, isControl bool, act int) int {
	if act == 0 {
		return 0
	}
	if !isControl && typeName(n.Type) == typeLoop {
		return several
	}
	return act
}
