// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest

import (
	"context"
	"hash/fnv"
	"slices"
	"strconv"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// hashed returns a deterministic pseudo-random number for the parts.
func hashed(parts ...string) uint32 {
	h := fnv.New32a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p)) // hash.Hash never fails
		_, _ = h.Write([]byte{0})
	}
	return h.Sum32()
}

// portsRunner emits the batch on most of the ports its node has an edge on.
type portsRunner struct {
	ports map[flow.NodeID][]string
}

func (r portsRunner) Execute(_ context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	var out execution.NodeOutput
	for _, p := range r.ports[call.Node.ID] {
		// A quarter of the ports stay empty, decided by the node and port alone.
		if hashed(string(call.Node.ID), p)%4 != 0 {
			out.Ports = append(out.Ports, execution.PortBatch{Port: p, Batch: call.Input})
		}
	}
	return out, nil
}

// ChaosDeps returns ports that give an arbitrary flow some behavior with no
// reference result: conditions, item routing and node output are
// pseudo-random functions of what they see, so a run is reproducible. A flow
// run with them may fail, for example when an exclusive gateway finds no true
// condition, but never for a reason of the interpreter itself.
func ChaosDeps(doc *flow.Document) execution.Deps {
	ports := make(map[flow.NodeID][]string)
	for _, e := range doc.Edges {
		if !slices.Contains(ports[e.From.Node], e.From.Port) {
			ports[e.From.Node] = append(ports[e.From.Node], e.From.Port)
		}
	}
	return execution.Deps{
		Runner: portsRunner{ports: ports},
		Conditions: ConditionFunc(func(expr string, env execution.EvalEnv) (bool, error) {
			return hashed(expr, string(env.Edge), string(env.Scope))%2 == 0, nil
		}),
		Router: RouterFunc(func(n *flow.Node, it execution.Item) []string {
			if len(ports[n.ID]) == 0 {
				return []string{"none"} // a port with no edge: the items stop here
			}
			var out []string
			for _, p := range ports[n.ID] {
				raw, _ := jsonvalue.Canonical(it.JSON)
				if hashed(string(n.ID), p, string(raw))%2 == 0 {
					out = append(out, p)
				}
			}
			if n.Routing != flow.RoutingInclusive && len(out) > 1 {
				out = out[:1] // an EXCLUSIVE node answers one port at most
			}
			return out
		}),
	}
}

// RandomGraph builds an unstructured acyclic flow from the bytes: nodes in a
// topological order, each reachable from the start through at least one edge
// from an earlier node, with extra edges that make merges, splits and joins of
// every mode. Nothing guarantees that it runs to SUCCEEDED; it is meant for
// checking that the interpreter behaves on flows no author would draw.
func RandomGraph(data []byte) *Builder {
	src := &byteSource{data: data}
	b := NewFlow("random")
	if src.flag() {
		b.Sequential()
	}
	b.Start("n0")
	count := 2 + src.intn(12)
	kinds := make([]string, count+1)
	kinds[0] = "start"
	for i := 1; i <= count; i++ {
		id := "n" + strconv.Itoa(i)
		if i == count {
			b.End(id)
			kinds[i] = "end"
			continue
		}
		kinds[i] = addRandomNode(b, src, id)
	}
	incoming := make([]int, count+1)
	outgoing := make([]int, count+1)
	for i := 1; i <= count; i++ {
		// One edge from an earlier node that can have outgoing edges, so the
		// node is reachable, then up to two more for merges and joins.
		extra := src.intn(3)
		for k := 0; k <= extra; k++ {
			from := src.intn(i)
			for from > 0 && kinds[from] == "end" {
				from--
			}
			if k > 0 && edgeExists(b, from, i) {
				continue
			}
			addRandomEdge(b, src, kinds, from, i, outgoing)
			incoming[i]++
			outgoing[from]++
		}
	}
	// A node that is not an end event gets a way out, towards the last node.
	for i := range count {
		if outgoing[i] == 0 && kinds[i] != "end" && !edgeExists(b, i, count) {
			addRandomEdge(b, src, kinds, i, count, outgoing)
			incoming[count]++
			outgoing[i]++
		}
	}
	for i := 1; i <= count; i++ {
		if incoming[i] >= 2 {
			setRandomJoin(b, src, "n"+strconv.Itoa(i))
		}
	}
	// A FIRST join with a corresponding split may let the other branches finish.
	doc := b.Doc()
	splits := flow.CorrespondingSplits(&doc)
	for i := 1; i <= count; i++ {
		id := "n" + strconv.Itoa(i)
		if j := b.mustNode(id).Join; j == nil || j.Mode != flow.JoinFirst {
			continue
		}
		if _, ok := splits[flow.NodeID(id)]; ok && src.flag() {
			b.Remaining(id, flow.RemainingLetFinish)
		}
	}
	// Sometimes the whole graph is the body of a loop: its last node becomes the
	// test that sends items back to the loop control, so a body full of
	// uncontrolled merges, splits and joins returns several tokens in one
	// iteration.
	if src.flag() {
		wrapInLoop(b, src, "n"+strconv.Itoa(count))
	}
	return b
}

// wrapInLoop makes the graph the body of a loop. The start leads to the loop
// control, which leads to what the start led to, and the last node, an end
// event without outgoing edges, becomes the test that goes back to the loop
// or out.
func wrapInLoop(b *Builder, src *byteSource, last string) {
	b.Loop("loop", 1+src.intn(3)).End("done")
	b.Modify(last, func(n *flow.Node) {
		n.Element = flow.ElementExclusiveGateway
		n.Type = "orcon.if@1"
		n.Routing = flow.RoutingExclusive
	})
	for i := range b.doc.Edges {
		if e := &b.doc.Edges[i]; e.From.Node == "n0" {
			e.From = flow.Endpoint{Node: "loop", Port: "loop"}
		}
	}
	b.Edge("n0", "loop")
	b.Edge(last, "loop", Port("again"), Back())
	b.Edge(last, "done", Port("out"), Default())
}

func edgeExists(b *Builder, from, to int) bool {
	f, t := flow.NodeID("n"+strconv.Itoa(from)), flow.NodeID("n"+strconv.Itoa(to))
	return slices.ContainsFunc(b.doc.Edges, func(e flow.Edge) bool { return e.From.Node == f && e.To == t })
}

func addRandomNode(b *Builder, src *byteSource, id string) string {
	switch src.intn(8) {
	case 0, 1:
		b.Task(id)
		return "task"
	case 2:
		b.Parallel(id)
		return "parallel"
	case 3:
		b.Exclusive(id)
		return "exclusive"
	case 4:
		b.Inclusive(id)
		return "inclusive"
	case 5:
		b.If(id, routingChoice(src))
		return "route"
	case 6:
		b.Switch(id, routingChoice(src))
		return "route"
	default:
		b.End(id)
		return "end"
	}
}

func routingChoice(src *byteSource) flow.Routing {
	if src.flag() {
		return flow.RoutingInclusive
	}
	return flow.RoutingExclusive
}

// addRandomEdge connects node from to node to. Edges out of gateways carry a
// condition (the first one of an exclusive or inclusive gateway may be its
// default); edges out of routing nodes pick one of three ports.
func addRandomEdge(b *Builder, src *byteSource, kinds []string, from, to int, outgoing []int) {
	f, t := "n"+strconv.Itoa(from), "n"+strconv.Itoa(to)
	var opts []EdgeOption
	switch kinds[from] {
	case "exclusive", "inclusive":
		// The first edge is the default; the others carry a condition.
		if outgoing[from] == 0 {
			opts = append(opts, Default())
		} else {
			opts = append(opts, Cond("c"+f+"-"+t))
		}
	case "route":
		opts = append(opts, Port("p"+strconv.Itoa(src.intn(3))))
		if outgoing[from] == 0 {
			opts = append(opts, Default())
		}
	case "start", "task", "parallel", "end":
	}
	if b.doc.Settings.BranchMode == flow.BranchSequential {
		opts = append(opts, Order(src.intn(3)))
	}
	b.Edge(f, t, opts...)
}

func setRandomJoin(b *Builder, src *byteSource, id string) {
	switch src.intn(4) {
	case 0:
		b.Join(id, flow.JoinAll)
	case 1:
		b.Join(id, flow.JoinActivated)
	case 2:
		b.Join(id, flow.JoinFirst)
	default:
	}
}
