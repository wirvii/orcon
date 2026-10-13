// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

// graph is the control-flow graph of a document, built once validation has
// established that ids are unique and every edge endpoint exists.
//
// Besides the forward (SEQUENCE) edges, it holds two implicit links that
// carry control: a boundary event follows the activity it is attached to, and
// a compensation handler follows the node it compensates. Back edges are kept
// apart because they only close loops; every analysis of "before" and
// "after" ignores them (ADR-0010 section 2: back edges are excluded from
// completion accounting).
type graph struct {
	doc   *Document
	index map[NodeID]int
	// succ and pred hold forward adjacency including the implicit links.
	succ, pred [][]int
	// outEdges and inEdges hold indices into doc.Edges, forward edges only.
	outEdges, inEdges [][]int
	starts            []int
	// nonInterrupting holds, per host node, its non-interrupting boundary events.
	nonInterrupting [][]int

	reachMemo  map[int][]bool
	blockedMem map[int][]bool
	idom       []int
}

func newGraph(d *Document) *graph {
	n := len(d.Nodes)
	g := &graph{
		doc:             d,
		index:           make(map[NodeID]int, n),
		succ:            make([][]int, n),
		pred:            make([][]int, n),
		outEdges:        make([][]int, n),
		inEdges:         make([][]int, n),
		nonInterrupting: make([][]int, n),
		reachMemo:       map[int][]bool{},
		blockedMem:      map[int][]bool{},
	}
	for i := range d.Nodes {
		node := &d.Nodes[i]
		g.index[node.ID] = i
		if node.Element == ElementStartEvent {
			g.starts = append(g.starts, i)
		}
	}
	for ei, e := range d.Edges {
		if e.Kind == EdgeBack {
			continue
		}
		from, to := g.index[e.From.Node], g.index[e.To]
		g.link(from, to)
		g.outEdges[from] = append(g.outEdges[from], ei)
		g.inEdges[to] = append(g.inEdges[to], ei)
	}
	for i := range d.Nodes {
		node := &d.Nodes[i]
		if node.Element == ElementBoundaryEvent {
			if from, ok := g.index[node.AttachedTo]; ok {
				g.link(from, i)
				if node.CancelActivity != nil && !*node.CancelActivity {
					g.nonInterrupting[from] = append(g.nonInterrupting[from], i)
				}
			}
		}
		if node.Compensation != nil {
			if to, ok := g.index[node.Compensation.Handler]; ok {
				g.link(i, to)
			}
		}
	}
	return g
}

func (g *graph) link(from, to int) {
	g.succ[from] = append(g.succ[from], to)
	g.pred[to] = append(g.pred[to], from)
}

// reachableFromStarts marks every node a token starting at a start event can reach.
func (g *graph) reachableFromStarts() []bool {
	seen := make([]bool, len(g.succ))
	stack := append([]int(nil), g.starts...)
	for _, s := range g.starts {
		seen[s] = true
	}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, y := range g.succ[x] {
			if !seen[y] {
				seen[y] = true
				stack = append(stack, y)
			}
		}
	}
	return seen
}

// reaches reports whether a path of one or more forward steps leads from a to b.
func (g *graph) reaches(a, b int) bool {
	set, ok := g.reachMemo[a]
	if !ok {
		set = make([]bool, len(g.succ))
		stack := []int{}
		for _, y := range g.succ[a] {
			if !set[y] {
				set[y] = true
				stack = append(stack, y)
			}
		}
		for len(stack) > 0 {
			x := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, y := range g.succ[x] {
				if !set[y] {
					set[y] = true
					stack = append(stack, y)
				}
			}
		}
		g.reachMemo[a] = set
	}
	return set[b]
}

// dominators computes the immediate dominator of every node reachable from a
// start event, with a virtual root joining all start events (Cooper, Harvey
// and Kennedy, "A Simple, Fast Dominance Algorithm"). The virtual root has
// index len(nodes); unreachable nodes get -1.
func (g *graph) dominators() []int {
	if g.idom != nil {
		return g.idom
	}
	n := len(g.succ)
	root := n
	order := g.reversePostorder(root)
	rpo := make([]int, n+1) // position of each node in the order
	for i := range rpo {
		rpo[i] = -1
	}
	for pos, x := range order {
		rpo[x] = pos
	}
	idom := make([]int, n+1)
	for i := range idom {
		idom[i] = -1
	}
	idom[root] = root
	for changed := true; changed; {
		changed = false
		for _, x := range order {
			if x == root {
				continue
			}
			next := -1
			for _, p := range g.predecessorsWithRoot(x) {
				if idom[p] == -1 {
					continue
				}
				if next == -1 {
					next = p
				} else {
					next = intersect(idom, rpo, p, next)
				}
			}
			if next != idom[x] {
				idom[x] = next
				changed = true
			}
		}
	}
	g.idom = idom
	return idom
}

// predecessorsWithRoot returns the forward predecessors of x, plus the
// virtual root when x is a start event.
func (g *graph) predecessorsWithRoot(x int) []int {
	preds := g.pred[x]
	if g.doc.Nodes[x].Element == ElementStartEvent {
		return append(append([]int(nil), preds...), len(g.succ))
	}
	return preds
}

func intersect(idom, rpo []int, a, b int) int {
	for a != b {
		for rpo[a] > rpo[b] {
			a = idom[a]
		}
		for rpo[b] > rpo[a] {
			b = idom[b]
		}
	}
	return a
}

// reversePostorder returns the nodes reachable from the virtual root in
// reverse postorder of a depth-first search.
func (g *graph) reversePostorder(root int) []int {
	n := len(g.succ)
	visited := make([]bool, n+1)
	var post []int
	type frame struct {
		node int
		next int
	}
	succOf := func(x int) []int {
		if x == root {
			return g.starts
		}
		return g.succ[x]
	}
	stack := []frame{{node: root}}
	visited[root] = true
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		children := succOf(top.node)
		if top.next < len(children) {
			c := children[top.next]
			top.next++
			if !visited[c] {
				visited[c] = true
				stack = append(stack, frame{node: c})
			}
			continue
		}
		post = append(post, top.node)
		stack = stack[:len(stack)-1]
	}
	for i, j := 0, len(post)-1; i < j; i, j = i+1, j-1 {
		post[i], post[j] = post[j], post[i]
	}
	return post
}

// dominates reports whether a dominates b: every path from a start event to b
// passes through a. A node dominates itself. Both must be reachable.
func (g *graph) dominates(a, b int) bool {
	idom := g.dominators()
	if idom[b] == -1 || idom[a] == -1 {
		return false
	}
	root := len(g.succ)
	for x := b; ; x = idom[x] {
		if x == a {
			return true
		}
		if x == root {
			return false
		}
	}
}

// reachNoJoin returns the nodes a token released at x can reach without
// passing a join: x itself is included, join nodes are neither included nor
// expanded, because a join fires once however many tokens arrive at it.
func (g *graph) reachNoJoin(x int) []bool {
	if set, ok := g.blockedMem[x]; ok {
		return set
	}
	set := make([]bool, len(g.succ))
	if g.doc.Nodes[x].Join == nil {
		set[x] = true
		stack := []int{x}
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, y := range g.succ[cur] {
				if !set[y] && g.doc.Nodes[y].Join == nil {
					set[y] = true
					stack = append(stack, y)
				}
			}
		}
	}
	g.blockedMem[x] = set
	return set
}
