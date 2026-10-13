// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
)

// nodeKind is how the interpreter runs a node. The kind comes from the node
// type for the nodes the format itself defines (orcon.loop, orcon.if,
// orcon.switch) and from the BPMN element class otherwise.
type nodeKind uint8

const (
	kindStart nodeKind = iota + 1
	kindEnd
	kindActivity
	kindParallel
	kindExclusive
	kindInclusive
	kindRouting
	kindLoop
)

// planNode is a node of a compiled flow with the edges around it resolved.
type planNode struct {
	node *flow.Node
	kind nodeKind
	// in lists the forward incoming edges in document order. The position of
	// an edge in this list is its slot at the node.
	in []int
	// out lists every outgoing edge, back edges included, in document order.
	out []int
	// join is set for a node with a join block.
	join *joinSpec
	// routing is the item-routing mode of kindRouting nodes.
	routing flow.Routing
	// defaultEdge is the outgoing edge marked default, or -1.
	defaultEdge int
	// maxIterations is the limit of a loop control.
	maxIterations int
	// closing lists the exit edges that complete when this loop's instance
	// closes: the edges whose outermost exited loop is this one.
	closing []int
	// dominators marks, for a join, the nodes that dominate it.
	dominators []bool
	// splitFor is, for a FIRST join, the split node whose branches the join
	// discriminates: the nearest dominating node from which the join is
	// reachable through at least two of its outgoing edges. It is -1 when there
	// is none, and then the join cancels nothing.
	splitFor int
	// nesting is the number of loops whose body contains the node.
	nesting int
}

type joinSpec struct {
	mode flow.JoinMode
}

// planEdge is an edge of a compiled flow.
type planEdge struct {
	edge *flow.Edge
	from int
	to   int
	back bool
	// slot is the position among the forward incoming edges of the target, or
	// -1 for a back edge.
	slot int
	// outPos is the position among the outgoing edges of the source.
	outPos int
	// exits lists the loop control nodes whose bodies the edge leaves,
	// innermost first. A token crossing the edge pops one scope frame per entry.
	exits []int
}

// Plan is a flow document compiled for the interpreter: nodes and edges
// resolved to indices, loop bodies, the dominators discriminator joins need,
// and the check that the flow uses only features this interpreter runs. A Plan
// is immutable and safe to share between executions; it can be cached by the
// content hash of the revision it was compiled from (ADR-0007 section 7).
type Plan struct {
	flowID     flow.ID
	nodes      []planNode
	edges      []planEdge
	index      map[flow.NodeID]int
	starts     []int
	sequential bool

	usesConditions bool
	usesRouting    bool
	usesActivities bool
}

// FlowID returns the id of the compiled flow.
func (p *Plan) FlowID() flow.ID { return p.flowID }

// Compile validates a flow document and compiles it. It fails with an error
// wrapping [ErrInvalidFlow] when static validation reports errors, and with an
// [*UnsupportedError] when the flow uses a feature a later cut provides.
func Compile(doc *flow.Document) (*Plan, error) {
	if doc == nil {
		return nil, fmt.Errorf("%w: no document", ErrInvalidFlow)
	}
	if err := flow.Validate(doc).Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFlow, err)
	}
	c := &compiler{
		plan: &Plan{
			flowID:     doc.ID,
			index:      make(map[flow.NodeID]int, len(doc.Nodes)),
			sequential: doc.Settings.BranchMode == flow.BranchSequential,
		},
	}
	c.settings(doc)
	nodes := slices.Clone(doc.Nodes)
	edges := slices.Clone(doc.Edges)
	c.nodes(nodes)
	c.edges(edges)
	c.loops()
	if len(c.issues) > 0 {
		return nil, &UnsupportedError{Issues: c.issues}
	}
	c.dominators()
	return c.plan, nil
}

type compiler struct {
	plan   *Plan
	issues []UnsupportedIssue
	// body[l] marks the nodes in the body of loop control l (nil for others).
	body [][]bool
}

func (c *compiler) unsupported(node flow.NodeID, edge flow.EdgeID, feature, cut string) {
	c.issues = append(c.issues, UnsupportedIssue{Node: node, Edge: edge, Feature: feature, Cut: cut})
}

// Cuts that own the features the interpreter rejects.
const (
	cutErrors      = "C7"
	cutWaits       = "C8"
	cutSubflows    = "C9"
	cutVariables   = "C10"
	cutUnspecified = "none: not specified by an accepted document"
)

func (c *compiler) settings(doc *flow.Document) {
	if doc.Settings.Timeouts != nil {
		c.unsupported("", "", "settings.timeouts", cutErrors)
	}
	if doc.Settings.ErrorFlow != "" {
		c.unsupported("", "", "settings.errorFlow", cutErrors)
	}
}

func typeName(nodeType string) string {
	name, _, _ := strings.Cut(nodeType, "@")
	return name
}

const (
	typeLoop   = "orcon.loop"
	typeIf     = "orcon.if"
	typeSwitch = "orcon.switch"
)

func (c *compiler) nodes(nodes []flow.Node) {
	p := c.plan
	p.nodes = make([]planNode, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		p.index[n.ID] = i
		pn := &p.nodes[i]
		pn.node = n
		pn.defaultEdge = -1
		pn.splitFor = -1
		pn.kind = c.kindOf(n)
		c.features(n, pn)
		switch pn.kind {
		case kindStart:
			p.starts = append(p.starts, i)
		case kindActivity:
			p.usesActivities = true
		case kindRouting:
			p.usesRouting = true
			pn.routing = n.Routing
			if pn.routing == "" {
				pn.routing = flow.RoutingExclusive
			}
		case kindLoop:
			if n.Loop != nil {
				pn.maxIterations = n.Loop.MaxIterations
			}
		case kindEnd, kindParallel, kindExclusive, kindInclusive:
		}
	}
}

// kindOf classifies a node, recording an issue for an element class the
// interpreter does not run.
func (c *compiler) kindOf(n *flow.Node) nodeKind {
	switch typeName(n.Type) {
	case typeLoop:
		return kindLoop
	case typeIf, typeSwitch:
		return kindRouting
	}
	switch n.Element {
	case flow.ElementStartEvent:
		return kindStart
	case flow.ElementEndEvent:
		return kindEnd
	case flow.ElementTask, flow.ElementServiceTask, flow.ElementScriptTask, flow.ElementSendTask:
		return kindActivity
	case flow.ElementParallelGateway:
		return kindParallel
	case flow.ElementExclusiveGateway:
		return kindExclusive
	case flow.ElementInclusiveGateway:
		return kindInclusive
	case flow.ElementUserTask, flow.ElementReceiveTask, flow.ElementEventBasedGateway,
		flow.ElementIntermediateCatchEvent, flow.ElementIntermediateThrowEvent:
		c.unsupported(n.ID, "", "element "+string(n.Element), cutWaits)
	case flow.ElementBoundaryEvent:
		c.unsupported(n.ID, "", "element "+string(n.Element), cutWaits)
	case flow.ElementCallActivity:
		c.unsupported(n.ID, "", "element "+string(n.Element), cutSubflows)
	case flow.ElementComplexGateway:
		c.unsupported(n.ID, "", "element "+string(n.Element), cutUnspecified)
	}
	return 0
}

// features records the behavior blocks of a node that a later cut provides.
func (c *compiler) features(n *flow.Node, pn *planNode) {
	if n.MultiInstance != nil {
		c.unsupported(n.ID, "", "multiInstance", cutSubflows)
	}
	if n.Wait != nil {
		c.unsupported(n.ID, "", "wait", cutWaits)
	}
	if n.Call != nil {
		c.unsupported(n.ID, "", "call", cutSubflows)
	}
	if n.Compensation != nil {
		c.unsupported(n.ID, "", "compensation", cutErrors)
	}
	if n.Retry != nil {
		c.unsupported(n.ID, "", "retry", cutErrors)
	}
	if n.Timeouts != nil {
		c.unsupported(n.ID, "", "timeouts", cutErrors)
	}
	if n.OnError != "" && n.OnError != flow.OnErrorStopExecution {
		c.unsupported(n.ID, "", "onError "+string(n.OnError), cutErrors)
	}
	if n.Disabled != nil && *n.Disabled {
		c.unsupported(n.ID, "", "disabled", cutUnspecified)
	}
	if n.Loop != nil && len(n.Loop.Carry) > 0 {
		c.unsupported(n.ID, "", "loop.carry", cutVariables)
	}
	if n.AttachedTo != "" || n.EventDefinition != nil {
		c.unsupported(n.ID, "", "boundary event definition", cutWaits)
	}
	if n.Join != nil {
		c.join(n, pn)
	}
}

// join reads the join block of a node. Validation guarantees the mode and
// data values are in their closed sets, that the data mode is runnable and that
// the node has at least two incoming edges, so a start event or a loop control
// (one entry edge) never carries one.
func (c *compiler) join(n *flow.Node, pn *planNode) {
	// Validation already rejected the data modes the engine cannot run
	// (flow.JoinDataRunnable), so APPEND is the only one left.
	pn.join = &joinSpec{mode: n.Join.Mode}
}

func (c *compiler) edges(edges []flow.Edge) {
	p := c.plan
	p.edges = make([]planEdge, len(edges))
	for i := range edges {
		e := &edges[i]
		from, to := p.index[e.From.Node], p.index[e.To]
		pe := &p.edges[i]
		*pe = planEdge{edge: e, from: from, to: to, back: e.Kind == flow.EdgeBack, slot: -1}
		src := &p.nodes[from]
		pe.outPos = len(src.out)
		src.out = append(src.out, i)
		if e.Default != nil && *e.Default && src.defaultEdge < 0 {
			src.defaultEdge = i
		}
		if !pe.back {
			dst := &p.nodes[to]
			pe.slot = len(dst.in)
			dst.in = append(dst.in, i)
		}
		if e.Condition != "" {
			p.usesConditions = true
			if src.kind != kindExclusive && src.kind != kindInclusive {
				c.unsupported("", e.ID, "a condition on an edge that does not leave an exclusive or inclusive gateway", cutUnspecified)
			}
		}
	}
}

// loops resolves the body of every loop control, rejects the shapes the
// scope accounting cannot handle, and computes the scope frames every edge
// pops.
func (c *compiler) loops() {
	p := c.plan
	n := len(p.nodes)
	succ := make([][]int, n)
	pred := make([][]int, n)
	for _, e := range p.edges {
		if e.back {
			continue
		}
		succ[e.from] = append(succ[e.from], e.to)
		pred[e.to] = append(pred[e.to], e.from)
	}
	c.body = make([][]bool, n)
	var loopNodes []int
	for i := range p.nodes {
		if p.nodes[i].kind != kindLoop {
			continue
		}
		loopNodes = append(loopNodes, i)
		c.loopBody(i, succ, pred)
	}
	if len(loopNodes) == 0 {
		return
	}
	c.nesting(loopNodes)
	c.exits(loopNodes)
}

// loopBody computes the body of loop control l: the nodes reachable from it
// through forward edges that reach the source of one of its back edges.
func (c *compiler) loopBody(l int, succ, pred [][]int) {
	p := c.plan
	ln := &p.nodes[l]
	if len(ln.in) != 1 {
		c.unsupported(ln.node.ID, "", fmt.Sprintf("a loop control with %d incoming forward edges (one entry edge is supported)", len(ln.in)), cutUnspecified)
	}
	for _, ei := range ln.out {
		e := p.edges[ei]
		if e.edge.From.Port != "loop" {
			c.unsupported("", e.edge.ID, fmt.Sprintf("port %q on a loop control (only port \"loop\" is defined by ADR-0010 section 6)", e.edge.From.Port), cutUnspecified)
		}
	}
	fwd := reach([]int{l}, succ, -1)
	var sources []int
	for _, e := range p.edges {
		if e.back && e.to == l {
			sources = append(sources, e.from)
			if !fwd[e.from] {
				c.unsupported("", e.edge.ID, "a back edge from a node its loop control does not reach", cutUnspecified)
			}
		}
	}
	rev := reach(sources, pred, l)
	body := make([]bool, len(p.nodes))
	for x := range body {
		body[x] = x != l && fwd[x] && rev[x]
	}
	c.body[l] = body
	for x, in := range body {
		if !in {
			continue
		}
		for _, ei := range p.nodes[x].in {
			from := p.edges[ei].from
			if from != l && !body[from] {
				c.unsupported(p.nodes[x].node.ID, p.edges[ei].edge.ID,
					fmt.Sprintf("an edge entering the body of loop %q from outside the loop", ln.node.ID), cutUnspecified)
			}
		}
	}
}

// reach returns the nodes reachable from the starts through adj, the starts
// included, without ever expanding or marking the node stop.
func reach(starts []int, adj [][]int, stop int) []bool {
	seen := make([]bool, len(adj))
	stack := make([]int, 0, len(starts))
	for _, s := range starts {
		if s != stop && !seen[s] {
			seen[s] = true
			stack = append(stack, s)
		}
	}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, y := range adj[x] {
			if y != stop && !seen[y] {
				seen[y] = true
				stack = append(stack, y)
			}
		}
	}
	return seen
}

// inside reports whether a node belongs to loop l: its body or the control.
func (c *compiler) inside(l, x int) bool { return x == l || c.body[l][x] }

// nesting counts, per node, the loops whose body contains it, and rejects
// loops whose bodies overlap without one containing the other.
func (c *compiler) nesting(loopNodes []int) {
	p := c.plan
	for x := range p.nodes {
		for _, l := range loopNodes {
			if c.body[l][x] {
				p.nodes[x].nesting++
			}
		}
	}
	for i, a := range loopNodes {
		for _, b := range loopNodes[i+1:] {
			if c.disjointOrNested(a, b) {
				continue
			}
			c.unsupported(p.nodes[a].node.ID, "", fmt.Sprintf("loop bodies overlapping loop %q without nesting", p.nodes[b].node.ID), cutUnspecified)
		}
	}
}

func (c *compiler) disjointOrNested(a, b int) bool {
	common, aInB, bInA := false, true, true
	for x := range c.plan.nodes {
		ina, inb := c.inside(a, x), c.inside(b, x)
		common = common || (ina && inb)
		if ina && !inb {
			aInB = false
		}
		if inb && !ina {
			bInA = false
		}
	}
	return !common || aInB || bInA
}

// exits computes the loops every edge leaves and registers each exit edge with
// the loop whose instance closing completes it: the outermost one it leaves.
func (c *compiler) exits(loopNodes []int) {
	p := c.plan
	size := func(l int) int {
		n := 0
		for x := range p.nodes {
			if c.inside(l, x) {
				n++
			}
		}
		return n
	}
	byInnermost := slices.Clone(loopNodes)
	slices.SortStableFunc(byInnermost, func(a, b int) int { return size(a) - size(b) })
	for i := range p.edges {
		e := &p.edges[i]
		for _, l := range byInnermost {
			if c.inside(l, e.from) && !c.inside(l, e.to) {
				e.exits = append(e.exits, l)
			}
		}
		// A back edge pops frames like any edge but carries no completion
		// signal (ADR-0010 section 2), so only forward exits are closed.
		if len(e.exits) > 0 && !e.back {
			outer := e.exits[len(e.exits)-1]
			p.nodes[outer].closing = append(p.nodes[outer].closing, i)
		}
	}
}

// dominators computes, for every join, the set of nodes that dominate it. The
// split whose branches converge at a join is found among them: the join
// continuation drops that split from its fork lineage, and a discriminator
// cancels the other branches of that split instance (ADR-0010 section 3).
func (c *compiler) dominators() {
	p := c.plan
	needed := false
	for i := range p.nodes {
		needed = needed || p.nodes[i].join != nil
	}
	if !needed {
		return
	}
	idom := immediateDominators(p)
	root := len(p.nodes)
	for i := range p.nodes {
		if p.nodes[i].join == nil {
			continue
		}
		dom := make([]bool, len(p.nodes))
		for x := idom[i]; x >= 0 && x != root; x = idom[x] {
			dom[x] = true
		}
		p.nodes[i].dominators = dom
		if p.nodes[i].join.mode == flow.JoinFirst {
			p.nodes[i].splitFor = nearestSplit(p, idom, i)
		}
	}
}

// immediateDominators computes the immediate dominator of every node reachable
// from a start event over the forward edges, with a virtual root joining all
// start events (Cooper, Harvey and Kennedy, "A Simple, Fast Dominance
// Algorithm"). The virtual root has index len(nodes); unreachable nodes get -1.
func immediateDominators(p *Plan) []int {
	n := len(p.nodes)
	root := n
	succ := make([][]int, n+1)
	pred := make([][]int, n+1)
	for _, e := range p.edges {
		if e.back {
			continue
		}
		succ[e.from] = append(succ[e.from], e.to)
		pred[e.to] = append(pred[e.to], e.from)
	}
	for _, s := range p.starts {
		succ[root] = append(succ[root], s)
		pred[s] = append(pred[s], root)
	}
	order := reversePostorder(succ, root)
	position := make([]int, n+1)
	for i := range position {
		position[i] = -1
	}
	for i, x := range order {
		position[x] = i
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
			for _, q := range pred[x] {
				if idom[q] == -1 {
					continue
				}
				if next == -1 {
					next = q
				} else {
					next = intersect(idom, position, q, next)
				}
			}
			if next != idom[x] {
				idom[x] = next
				changed = true
			}
		}
	}
	return idom[:n+1]
}

func intersect(idom, position []int, a, b int) int {
	for a != b {
		for position[a] > position[b] {
			a = idom[a]
		}
		for position[b] > position[a] {
			b = idom[b]
		}
	}
	return a
}

// reversePostorder lists the nodes reachable from root in reverse postorder,
// walking iteratively so a deep flow cannot exhaust the stack.
func reversePostorder(succ [][]int, root int) []int {
	type frame struct{ node, next int }
	visited := make([]bool, len(succ))
	var post []int
	visited[root] = true
	stack := []frame{{node: root}}
	for len(stack) > 0 {
		f := &stack[len(stack)-1]
		if f.next < len(succ[f.node]) {
			y := succ[f.node][f.next]
			f.next++
			if !visited[y] {
				visited[y] = true
				stack = append(stack, frame{node: y})
			}
			continue
		}
		post = append(post, f.node)
		stack = stack[:len(stack)-1]
	}
	slices.Reverse(post)
	return post
}

// nearestSplit returns the nearest proper dominator of the join from which the
// join is reachable through at least two outgoing forward edges, or -1. The
// walk follows the immediate dominators, so the answer is computed once per
// plan and does not depend on the tokens of a run.
func nearestSplit(p *Plan, idom []int, join int) int {
	succ := make([][]int, len(p.nodes))
	for _, e := range p.edges {
		if !e.back {
			succ[e.from] = append(succ[e.from], e.to)
		}
	}
	root := len(p.nodes)
	for x := idom[join]; x >= 0 && x != root; x = idom[x] {
		branches := 0
		for _, ei := range p.nodes[x].out {
			e := &p.edges[ei]
			if e.back {
				continue
			}
			if e.to == join || reach([]int{e.to}, succ, -1)[join] {
				branches++
			}
		}
		if branches >= 2 {
			return x
		}
	}
	return -1
}
