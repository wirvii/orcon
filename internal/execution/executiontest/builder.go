// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest

import (
	"fmt"
	"strconv"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// FlowID is the id every built flow carries.
const FlowID = "01930000-0000-7000-8000-0000000000aa"

// Builder assembles a flow document node by node. Every method returns the
// builder, so a flow reads as a list. Node ids double as node names.
type Builder struct {
	doc   flow.Document
	edges int
}

// NewFlow starts a flow with the given display name and no nodes.
func NewFlow(name string) *Builder {
	return &Builder{doc: flow.Document{
		Format:   flow.FormatTag,
		ID:       FlowID,
		Name:     name,
		Settings: flow.Settings{},
	}}
}

// Sequential sets the branch mode of the flow to SEQUENTIAL.
func (b *Builder) Sequential() *Builder {
	b.doc.Settings.BranchMode = flow.BranchSequential
	return b
}

func (b *Builder) node(id, nodeType string, element flow.Element) *flow.Node {
	b.doc.Nodes = append(b.doc.Nodes, flow.Node{
		ID:         flow.NodeID(id),
		Name:       id,
		Element:    element,
		Type:       nodeType,
		Parameters: emptyObject(),
	})
	return &b.doc.Nodes[len(b.doc.Nodes)-1]
}

func emptyObject() jsonvalue.Value {
	v, err := jsonvalue.Object()
	if err != nil {
		panic(fmt.Sprintf("executiontest: empty object: %v", err))
	}
	return v
}

// Start adds a start event.
func (b *Builder) Start(id string) *Builder {
	b.node(id, "orcon.start@1", flow.ElementStartEvent)
	return b
}

// End adds an end event.
func (b *Builder) End(id string) *Builder {
	b.node(id, "orcon.end@1", flow.ElementEndEvent)
	return b
}

// Task adds an activity node of the given node type, run by the NodeRunner.
func (b *Builder) Task(id string) *Builder {
	b.node(id, "orcon.task@1", flow.ElementTask)
	return b
}

// Parallel adds a parallel gateway: an AND-split, or with Join an AND-join.
func (b *Builder) Parallel(id string) *Builder {
	b.node(id, "orcon.parallel@1", flow.ElementParallelGateway)
	return b
}

// Exclusive adds an exclusive gateway (XOR).
func (b *Builder) Exclusive(id string) *Builder {
	b.node(id, "orcon.exclusive@1", flow.ElementExclusiveGateway)
	return b
}

// Inclusive adds an inclusive gateway (OR).
func (b *Builder) Inclusive(id string) *Builder {
	b.node(id, "orcon.inclusive@1", flow.ElementInclusiveGateway)
	return b
}

// If adds an orcon.if item-routing node with the given routing mode.
func (b *Builder) If(id string, routing flow.Routing) *Builder {
	n := b.node(id, "orcon.if@1", flow.ElementExclusiveGateway)
	n.Routing = routing
	return b
}

// Switch adds an orcon.switch item-routing node with the given routing mode.
func (b *Builder) Switch(id string, routing flow.Routing) *Builder {
	n := b.node(id, "orcon.switch@1", flow.ElementInclusiveGateway)
	n.Routing = routing
	return b
}

// Loop adds an orcon.loop control node with the given iteration limit.
func (b *Builder) Loop(id string, maxIterations int) *Builder {
	n := b.node(id, "orcon.loop@1", flow.ElementExclusiveGateway)
	n.Loop = &flow.Loop{MaxIterations: maxIterations}
	return b
}

// Join gives an existing node a join block with the mode and APPEND data.
func (b *Builder) Join(id string, mode flow.JoinMode) *Builder {
	b.mustNode(id).Join = &flow.Join{Mode: mode, Data: flow.JoinDataAppend}
	return b
}

// IterationVariable declares an iteration-scope variable that is carried to the
// next iteration.
func (b *Builder) IterationVariable(name string) *Builder {
	carry := true
	b.doc.Variables = append(b.doc.Variables, flow.Variable{Name: name, Scope: flow.ScopeIteration, Carry: &carry})
	return b
}

// Remaining sets the remaining option of an existing join block.
func (b *Builder) Remaining(id string, remaining flow.JoinRemaining) *Builder {
	b.mustNode(id).Join.Remaining = remaining
	return b
}

// Modify applies fn to the node with the given id, for features the other
// methods do not cover.
func (b *Builder) Modify(id string, fn func(*flow.Node)) *Builder {
	fn(b.mustNode(id))
	return b
}

func (b *Builder) mustNode(id string) *flow.Node {
	for i := range b.doc.Nodes {
		if b.doc.Nodes[i].ID == flow.NodeID(id) {
			return &b.doc.Nodes[i]
		}
	}
	panic(fmt.Sprintf("executiontest: no node %q", id))
}

// EdgeOption adjusts an edge added with [Builder.Edge].
type EdgeOption func(*flow.Edge)

// Port sets the output port of the edge; the default is "main".
func Port(port string) EdgeOption { return func(e *flow.Edge) { e.From.Port = port } }

// Cond sets the condition expression of the edge.
func Cond(expr string) EdgeOption { return func(e *flow.Edge) { e.Condition = expr } }

// Default marks the edge as the default edge of its source.
func Default() EdgeOption {
	return func(e *flow.Edge) {
		t := true
		e.Default = &t
	}
}

// Order sets the sibling order of the edge for SEQUENTIAL branch mode.
func Order(n int) EdgeOption { return func(e *flow.Edge) { e.Order = n } }

// Back makes the edge a loop back edge.
func Back() EdgeOption { return func(e *flow.Edge) { e.Kind = flow.EdgeBack } }

// Edge connects two nodes. The edge ids are e1, e2 and so on, in the order added.
func (b *Builder) Edge(from, to string, opts ...EdgeOption) *Builder {
	b.edges++
	e := flow.Edge{
		ID:   flow.EdgeID("e" + strconv.Itoa(b.edges)),
		From: flow.Endpoint{Node: flow.NodeID(from), Port: flow.PortMain},
		To:   flow.NodeID(to),
		Kind: flow.EdgeSequence,
	}
	for _, o := range opts {
		o(&e)
	}
	b.doc.Edges = append(b.doc.Edges, e)
	return b
}

// Chain connects the nodes one after another with plain edges.
func (b *Builder) Chain(ids ...string) *Builder {
	for i := 1; i < len(ids); i++ {
		b.Edge(ids[i-1], ids[i])
	}
	return b
}

// Doc returns a copy of the document built so far.
func (b *Builder) Doc() flow.Document {
	d := b.doc
	d.Nodes = append([]flow.Node(nil), b.doc.Nodes...)
	d.Edges = append([]flow.Edge(nil), b.doc.Edges...)
	return d
}
