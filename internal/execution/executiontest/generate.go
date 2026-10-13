// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// This file generates structured flows from a byte string, together with the
// result a reference model computes for them. The reference model evaluates
// the generated program directly (a tree of sequences, splits, joins and
// loops) and knows nothing of tokens, scopes or completion signals, so a
// disagreement with the interpreter points at one of them.
//
// A structured flow cannot deadlock by construction: every split is closed by
// a join or a merge, every branch emits at least one item, and every loop is a
// do-while whose counter is reset on entry, so every item that enters leaves
// after the same number of iterations. The interpreter must therefore end each
// generated flow in SUCCEEDED, and (without discriminators) with the batch the
// reference model predicts.

// GenOptions tunes [Generate].
type GenOptions struct {
	// Discriminators allows FIRST joins. Which branch arrives first depends on
	// scheduling, so a program with one has no reference result.
	Discriminators bool
	// MaxNodes bounds the size of the program; zero means 40.
	MaxNodes int
}

// Program is a generated flow and what to run it with.
type Program struct {
	Flow  *Builder
	Input execution.Batch
	// Want lists the end paths ("end:a>b>c") the reference model predicts, in
	// order. It is meaningful only when HasReference is true.
	Want         []string
	HasReference bool

	routes  []routeSpec
	loops   []loopSpec
	verdict Conditions
}

// Deps returns fresh ports for one run of the program.
func (p Program) Deps() execution.Deps {
	behaviors := map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){}
	routers := map[flow.NodeID]RouterFunc{}
	for _, l := range p.loops {
		behaviors[flow.NodeID("z"+l.id)] = SetField("lc"+l.id, 0)
		behaviors[flow.NodeID("i"+l.id)] = Increment("lc" + l.id)
		routers[flow.NodeID("c"+l.id)] = WhileLess("lc"+l.id, int64(l.limit))
	}
	for _, r := range p.routes {
		routers[flow.NodeID("r"+r.id)] = routeByField("r"+r.id, r.ports)
	}
	router := RouterFunc(func(n *flow.Node, it execution.Item) []string { return routers[n.ID](n, it) })
	return execution.Deps{Runner: &TraceRunner{Behaviors: behaviors}, Router: router, Conditions: p.verdict}
}

// routeByField sends an item to the ports p0..p(ports-1) its field lists; a
// listed port that the node does not have is ignored.
func routeByField(field string, ports int) RouterFunc {
	return func(n *flow.Node, it execution.Item) []string {
		v, _ := it.JSON.Get(field)
		var out []string
		for _, s := range v.Items() {
			str, _ := s.Str()
			if idx, err := strconv.Atoi(strings.TrimPrefix(str, "p")); err == nil && idx < ports {
				out = append(out, str)
			}
		}
		if n.Routing != flow.RoutingInclusive && len(out) > 1 {
			out = out[:1] // an EXCLUSIVE node answers one port at most
		}
		return out
	}
}

type routeSpec struct {
	id    string
	ports int
}

type loopSpec struct {
	id    string
	limit int
}

// byteSource reads choices from the generator input; it yields zeros once the
// input is exhausted, so every input byte string generates some program.
type byteSource struct {
	data []byte
	pos  int
}

func (s *byteSource) intn(n int) int {
	if n <= 1 {
		return 0
	}
	if s.pos >= len(s.data) {
		s.pos++
		return 0
	}
	v := int(s.data[s.pos])
	s.pos++
	return v % n
}

func (s *byteSource) flag() bool { return s.intn(2) == 1 }

// genNode is a node of the program tree: one of the types below.
type genNode = any

// Program tree.
type (
	taskN struct{ id string }
	seqN  struct{ items []genNode }
	parN  struct {
		id       string
		branches []genNode
		mode     flow.JoinMode
	}
	xorN struct {
		id       string
		branches []genNode
		verdicts []bool // one per branch but the last, which is the default
	}
	orN struct {
		id       string
		branches []genNode
		verdicts []bool
	}
	routeN struct {
		id       string
		routing  flow.Routing
		branches []genNode // ports p0..p(n-2); the last branch is the default
	}
	loopN struct {
		id    string
		body  genNode
		limit int
	}
)

type generator struct {
	src      *byteSource
	opts     GenOptions
	nodes    int
	counter  int
	verdict  Conditions
	routes   []routeSpec
	loops    []loopSpec
	hasFirst bool
}

func (g *generator) id() string {
	g.counter++
	return strconv.Itoa(g.counter)
}

// maxItems bounds the number of items a generated program may produce. Splits
// that copy every item to every branch, inside loops, grow the batch
// exponentially, so a program over the bound is regenerated smaller.
const maxItems = 48

// Generate builds a structured flow, its input and its reference result from
// the bytes. The first byte selects the branch mode.
func Generate(data []byte, opts GenOptions) Program {
	if opts.MaxNodes == 0 {
		opts.MaxNodes = 40
	}
	for {
		g := &generator{src: &byteSource{data: data}, opts: opts, verdict: Conditions{}}
		sequential := g.src.flag()
		tree := g.sequence(0)
		if opts.MaxNodes > 2 && 3*fanOut(tree) > maxItems {
			opts.MaxNodes = opts.MaxNodes * 2 / 3
			continue
		}
		return g.program(tree, sequential)
	}
}

// fanOut is an upper bound of the items a node produces per item it receives.
func fanOut(n genNode) float64 {
	switch n := n.(type) {
	case taskN:
		return 1
	case seqN:
		total := 1.0
		for _, item := range n.items {
			total *= fanOut(item)
		}
		return total
	case parN:
		return sumFanOut(n.branches)
	case orN:
		return sumFanOut(n.branches)
	case routeN:
		return sumFanOut(n.branches)
	case xorN:
		most := 0.0
		for _, br := range n.branches {
			most = max(most, fanOut(br))
		}
		return most
	case loopN:
		total := 1.0
		for range n.limit {
			total *= fanOut(n.body)
		}
		return total
	default:
		return 1
	}
}

func sumFanOut(branches []genNode) float64 {
	total := 0.0
	for _, br := range branches {
		total += fanOut(br)
	}
	return total
}

func (g *generator) program(tree genNode, sequential bool) Program {
	b := NewFlow("generated")
	if sequential {
		b.Sequential()
	}
	b.Start("start").End("end")
	em := &emitter{b: b, sequential: sequential, src: g.src}
	entry, ex := em.emit(tree)
	b.Edge("start", entry)
	b.Edge(ex.node, "end", Port(ex.port))

	prog := Program{Flow: b, routes: g.routes, loops: g.loops, verdict: g.verdict, HasReference: !g.hasFirst}
	items := g.inputItems()
	prog.Input = execution.NewBatch(itemsOf(items)...)
	if prog.HasReference {
		for _, r := range evalNode(tree, items, g.loopLimits()) {
			prog.Want = append(prog.Want, "end:"+strings.Join(r.path, ">"))
		}
	}
	return prog
}

func (g *generator) loopLimits() map[string]int {
	out := make(map[string]int, len(g.loops))
	for _, l := range g.loops {
		out[l.id] = l.limit
	}
	return out
}

func itemsOf(recs []rec) []execution.Item {
	out := make([]execution.Item, len(recs))
	for i, r := range recs {
		members := []jsonvalue.Member{{Key: "id", Value: jsonvalue.Int(int64(i))}}
		for _, key := range sortedKeys(r.routes) {
			ports := make([]jsonvalue.Value, len(r.routes[key]))
			for j, p := range r.routes[key] {
				ports[j] = jsonvalue.String(p)
			}
			members = append(members, jsonvalue.Member{Key: key, Value: jsonvalue.Array(ports...)})
		}
		obj, err := jsonvalue.Object(members...)
		if err != nil {
			panic(fmt.Sprintf("executiontest: generated item: %v", err))
		}
		out[i] = execution.Item{JSON: obj}
	}
	return out
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Insertion sort: the maps are tiny and the order must be stable.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

// inputItems makes one to three items, each with a route per route block.
func (g *generator) inputItems() []rec {
	n := 1 + g.src.intn(3)
	items := make([]rec, n)
	for i := range items {
		items[i] = rec{ctr: map[string]int{}, routes: map[string][]string{}}
		for _, r := range g.routes {
			var ports []string
			for p := range 3 {
				if g.src.flag() {
					ports = append(ports, "p"+strconv.Itoa(p))
				}
			}
			items[i].routes["r"+r.id] = ports
		}
	}
	return items
}

func (g *generator) sequence(depth int) any {
	n := 1 + g.src.intn(3)
	items := make([]genNode, n)
	for i := range items {
		items[i] = g.element(depth)
	}
	if n == 1 {
		return items[0]
	}
	return seqN{items: items}
}

func (g *generator) branches(depth, fewest, most int) []genNode {
	n := fewest + g.src.intn(most-fewest+1)
	out := make([]genNode, n)
	for i := range out {
		out[i] = g.sequence(depth + 1)
	}
	return out
}

func (g *generator) bools(n int) []bool {
	out := make([]bool, n)
	for i := range out {
		out[i] = g.src.flag()
	}
	return out
}

// element generates one block. The kinds are weighted toward the cheap ones;
// past the depth or size budget only tasks are generated.
func (g *generator) element(depth int) any {
	g.nodes++
	id := g.id()
	if depth >= 3 || g.nodes > g.opts.MaxNodes {
		return taskN{id: "t" + id}
	}
	choice := g.src.intn(100)
	switch {
	case choice < 30:
		return taskN{id: "t" + id}
	case choice < 45:
		return parN{id: id, branches: g.branches(depth, 2, 3), mode: g.parMode()}
	case choice < 60:
		br := g.branches(depth, 2, 3)
		return xorN{id: id, branches: br, verdicts: g.recordVerdicts("x"+id, g.bools(len(br)-1))}
	case choice < 72:
		br := g.branches(depth, 2, 3)
		return orN{id: id, branches: br, verdicts: g.recordVerdicts("o"+id, g.bools(len(br)-1))}
	case choice < 84:
		routing := flow.RoutingExclusive
		if g.src.flag() {
			routing = flow.RoutingInclusive
		}
		br := g.branches(depth, 3, 4)
		g.routes = append(g.routes, routeSpec{id: id, ports: len(br) - 1})
		return routeN{id: id, routing: routing, branches: br}
	default:
		limit := 1 + g.src.intn(3)
		g.loops = append(g.loops, loopSpec{id: id, limit: limit})
		return loopN{id: id, body: g.sequence(depth + 1), limit: limit}
	}
}

func (g *generator) parMode() flow.JoinMode {
	if g.opts.Discriminators && g.src.intn(3) == 0 {
		g.hasFirst = true
		return flow.JoinFirst
	}
	if g.src.flag() {
		return flow.JoinAll
	}
	return flow.JoinActivated
}

// recordVerdicts registers the condition table entries of a gateway.
func (g *generator) recordVerdicts(prefix string, verdicts []bool) []bool {
	for i, v := range verdicts {
		g.verdict[fmt.Sprintf("%s/%d", prefix, i)] = v
	}
	return verdicts
}

// exit is the place a block hands control on from: a node and a port.
type exit struct{ node, port string }

type emitter struct {
	b          *Builder
	sequential bool
	src        *byteSource
}

// orderOption returns a random sibling order in SEQUENTIAL mode.
func (e *emitter) orderOption(i int) []EdgeOption {
	if !e.sequential {
		return nil
	}
	// A permutation-like order that depends on the input but is always valid.
	return []EdgeOption{Order(1 + (i+e.src.intn(3))%3)}
}

func (e *emitter) emit(n genNode) (string, exit) {
	switch n := n.(type) {
	case taskN:
		e.b.Task(n.id)
		return n.id, exit{n.id, flow.PortMain}
	case seqN:
		return e.emitSeq(n)
	case parN:
		return e.emitPar(n)
	case xorN:
		return e.emitChoice(n.id, "x", n.branches, true)
	case orN:
		return e.emitChoice(n.id, "o", n.branches, false)
	case routeN:
		return e.emitRoute(n)
	case loopN:
		return e.emitLoop(n)
	default:
		panic(fmt.Sprintf("executiontest: unknown program node %T", n))
	}
}

func (e *emitter) emitSeq(n seqN) (string, exit) {
	var entry string
	var prev exit
	for i, item := range n.items {
		in, out := e.emit(item)
		if i == 0 {
			entry = in
		} else {
			e.b.Edge(prev.node, in, Port(prev.port))
		}
		prev = out
	}
	return entry, prev
}

func (e *emitter) emitPar(n parN) (string, exit) {
	split, join := "p"+n.id, "j"+n.id
	e.b.Parallel(split).Parallel(join).Join(join, n.mode)
	exits := make([]exit, len(n.branches))
	for i, br := range n.branches {
		in, out := e.emit(br)
		e.b.Edge(split, in, e.orderOption(i)...)
		exits[i] = out
	}
	for _, out := range exits {
		e.b.Edge(out.node, join, Port(out.port))
	}
	return split, exit{join, flow.PortMain}
}

// emitChoice emits an exclusive (merge by a plain task) or inclusive (merge by
// an ACTIVATED join) choice. The last branch is the default.
func (e *emitter) emitChoice(id, prefix string, branches []genNode, exclusive bool) (string, exit) {
	gate, merge := prefix+id, "m"+id
	if exclusive {
		e.b.Exclusive(gate).Task(merge)
	} else {
		merge = "j" + id
		e.b.Inclusive(gate).Parallel(merge).Join(merge, flow.JoinActivated)
	}
	exits := make([]exit, len(branches))
	for i, br := range branches {
		in, out := e.emit(br)
		opts := e.orderOption(i)
		if i < len(branches)-1 {
			opts = append(opts, Cond(fmt.Sprintf("%s/%d", gate, i)))
		} else {
			opts = append(opts, Default())
		}
		e.b.Edge(gate, in, opts...)
		exits[i] = out
	}
	for _, out := range exits {
		e.b.Edge(out.node, merge, Port(out.port))
	}
	return gate, exit{merge, flow.PortMain}
}

func (e *emitter) emitRoute(n routeN) (string, exit) {
	router, join := "r"+n.id, "j"+n.id
	e.b.If(router, n.routing).Parallel(join).Join(join, flow.JoinActivated)
	exits := make([]exit, len(n.branches))
	for i, br := range n.branches {
		in, out := e.emit(br)
		opts := e.orderOption(i)
		if i < len(n.branches)-1 {
			opts = append(opts, Port("p"+strconv.Itoa(i)))
		} else {
			opts = append(opts, Port("dflt"), Default())
		}
		e.b.Edge(router, in, opts...)
		exits[i] = out
	}
	for _, out := range exits {
		e.b.Edge(out.node, join, Port(out.port))
	}
	return router, exit{join, flow.PortMain}
}

// emitLoop emits reset -> loop -> body -> increment -> test, where the test
// goes back to the loop control or leaves through its "false" port.
func (e *emitter) emitLoop(n loopN) (string, exit) {
	reset, ctl, inc, test := "z"+n.id, "L"+n.id, "i"+n.id, "c"+n.id
	e.b.Task(reset).Loop(ctl, n.limit).Task(inc).If(test, flow.RoutingExclusive)
	in, out := e.emit(n.body)
	e.b.Edge(reset, ctl)
	e.b.Edge(ctl, in, Port("loop"))
	e.b.Edge(out.node, inc, Port(out.port))
	e.b.Edge(inc, test)
	e.b.Edge(test, ctl, Port("true"), Back())
	return reset, exit{test, "false"}
}

// rec is an item of the reference model.
type rec struct {
	path   []string
	ctr    map[string]int
	routes map[string][]string
}

func (r rec) with(step string) rec {
	r.path = append(append([]string(nil), r.path...), step)
	return r
}

func (r rec) counted(key string, value int) rec {
	ctr := make(map[string]int, len(r.ctr)+1)
	for k, v := range r.ctr {
		ctr[k] = v
	}
	ctr[key] = value
	r.ctr = ctr
	return r
}

func mapRecs(in []rec, f func(rec) rec) []rec {
	out := make([]rec, len(in))
	for i, r := range in {
		out[i] = f(r)
	}
	return out
}

func evalNode(n genNode, in []rec, limits map[string]int) []rec {
	switch n := n.(type) {
	case taskN:
		return mapRecs(in, func(r rec) rec { return r.with(n.id) })
	case seqN:
		for _, item := range n.items {
			in = evalNode(item, in, limits)
		}
		return in
	case parN:
		var out []rec
		for _, br := range n.branches {
			out = append(out, evalNode(br, in, limits)...)
		}
		return out
	case xorN:
		pick := len(n.branches) - 1
		for i, v := range n.verdicts {
			if v {
				pick = i
				break
			}
		}
		return mapRecs(evalNode(n.branches[pick], in, limits), func(r rec) rec { return r.with("m" + n.id) })
	case orN:
		var out []rec
		taken := false
		for i, v := range n.verdicts {
			if v {
				taken = true
				out = append(out, evalNode(n.branches[i], in, limits)...)
			}
		}
		if !taken {
			out = evalNode(n.branches[len(n.branches)-1], in, limits)
		}
		return out
	case routeN:
		return evalRoute(n, in, limits)
	case loopN:
		in = mapRecs(in, func(r rec) rec { return r.with("z"+n.id).counted("lc"+n.id, 0) })
		for range n.limit {
			in = evalNode(n.body, in, limits)
			in = mapRecs(in, func(r rec) rec { return r.with("i"+n.id).counted("lc"+n.id, r.ctr["lc"+n.id]+1) })
		}
		return in
	default:
		panic(fmt.Sprintf("executiontest: unknown program node %T", n))
	}
}

func evalRoute(n routeN, in []rec, limits map[string]int) []rec {
	dflt := len(n.branches) - 1
	perBranch := make([][]rec, len(n.branches))
	for _, r := range in {
		var matched []int
		for _, p := range r.routes["r"+n.id] {
			idx, _ := strconv.Atoi(strings.TrimPrefix(p, "p"))
			if idx < dflt {
				matched = append(matched, idx)
			}
		}
		if n.routing == flow.RoutingExclusive && len(matched) > 1 {
			matched = matched[:1]
		}
		if len(matched) == 0 {
			matched = []int{dflt}
		}
		for _, idx := range matched {
			perBranch[idx] = append(perBranch[idx], r)
		}
	}
	var out []rec
	for i, items := range perBranch {
		if len(items) > 0 {
			out = append(out, evalNode(n.branches[i], items, limits)...)
		}
	}
	return out
}
