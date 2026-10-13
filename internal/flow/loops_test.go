// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// loopDoc builds a document with a start event, a loop control L and the given
// nodes. Each edge is written "from>to" or "from:port>to", and a back edge ends
// in "!".
func loopDoc(t *testing.T, nodes map[string]flow.Node, edges []string) *flow.Document {
	t.Helper()
	params, err := jsonvalue.Object()
	if err != nil {
		t.Fatal(err)
	}
	doc := &flow.Document{Format: flow.FormatTag, ID: "01930000-0000-7000-8000-0000000000cc", Name: "loops"}
	add := func(id string, n flow.Node) {
		n.ID, n.Name, n.Parameters = flow.NodeID(id), id, params
		doc.Nodes = append(doc.Nodes, n)
	}
	add("start", flow.Node{Element: flow.ElementStartEvent, Type: "orcon.start@1"})
	add("L", flow.Node{Element: flow.ElementExclusiveGateway, Type: "orcon.loop@1", Loop: &flow.Loop{MaxIterations: 3}})
	for _, id := range []string{"a", "b", "c", "j", "m", "p", "end", "L2", "q"} {
		if n, ok := nodes[id]; ok {
			add(id, n)
		}
	}
	doc.Edges = append(doc.Edges, flow.Edge{ID: "e0", From: flow.Endpoint{Node: "start", Port: "main"}, To: "L", Kind: flow.EdgeSequence})
	for i, spec := range edges {
		kind := flow.EdgeSequence
		if spec[len(spec)-1] == '!' {
			kind, spec = flow.EdgeBack, spec[:len(spec)-1]
		}
		var from, port, to string
		port = "main"
		colon, gt := -1, -1
		for k, c := range spec {
			switch {
			case c == ':' && colon < 0:
				colon = k
			case c == '>':
				gt = k
			}
		}
		to = spec[gt+1:]
		if colon >= 0 {
			from, port = spec[:colon], spec[colon+1:gt]
		} else {
			from = spec[:gt]
		}
		doc.Edges = append(doc.Edges, flow.Edge{
			ID: flow.EdgeID("e" + string(rune('1'+i))), From: flow.Endpoint{Node: flow.NodeID(from), Port: port}, To: flow.NodeID(to), Kind: kind,
		})
	}
	return doc
}

func TestLoopsReturningSeveral(t *testing.T) {
	t.Parallel()
	task := flow.Node{Element: flow.ElementTask, Type: "orcon.set@1"}
	gate := func(el flow.Element, typ string) flow.Node { return flow.Node{Element: el, Type: typ} }
	join := func(mode flow.JoinMode, rem flow.JoinRemaining) flow.Node {
		return flow.Node{Element: flow.ElementParallelGateway, Type: "orcon.merge@1", Join: &flow.Join{Mode: mode, Data: flow.JoinDataAppend, Remaining: rem}}
	}
	tests := []struct {
		name    string
		nodes   map[string]flow.Node
		edges   []string
		several bool
	}{
		{
			"one task going back",
			map[string]flow.Node{"a": task},
			[]string{"L:loop>a", "a>L!"}, false,
		},
		{
			"an exclusive choice whose branches both go back",
			map[string]flow.Node{"a": gate(flow.ElementExclusiveGateway, "orcon.exclusive@1")},
			[]string{"L:loop>a", "a>L!", "a>L!"}, true,
		},
		{
			"a routing node with two ports back",
			map[string]flow.Node{"a": {Element: flow.ElementExclusiveGateway, Type: "orcon.if@1", Routing: flow.RoutingExclusive}},
			[]string{"L:loop>a", "a:true>L!", "a:false>L!"}, true,
		},
		{
			"a parallel split with a thread going back from each branch",
			map[string]flow.Node{"p": gate(flow.ElementParallelGateway, "orcon.parallel@1"), "a": task, "b": task},
			[]string{"L:loop>p", "p>a", "p>b", "a>L!", "b>L!"}, true,
		},
		{
			"a parallel split joined with ALL before the way back",
			map[string]flow.Node{"p": gate(flow.ElementParallelGateway, "orcon.parallel@1"), "a": task, "b": task, "j": join(flow.JoinAll, "")},
			[]string{"L:loop>p", "p>a", "p>b", "a>j", "b>j", "j>L!"}, false,
		},
		{
			"a parallel split merged by a plain node before the way back",
			map[string]flow.Node{"p": gate(flow.ElementParallelGateway, "orcon.parallel@1"), "a": task, "b": task, "m": task},
			[]string{"L:loop>p", "p>a", "p>b", "a>m", "b>m", "m>L!"}, true,
		},
		{
			"a discriminator that lets the others finish",
			map[string]flow.Node{"p": gate(flow.ElementParallelGateway, "orcon.parallel@1"), "a": task, "b": task, "j": join(flow.JoinFirst, flow.RemainingLetFinish)},
			[]string{"L:loop>p", "p>a", "p>b", "a>j", "b>j", "j>L!"}, true,
		},
		{
			"a discriminator that cancels the others",
			map[string]flow.Node{"p": gate(flow.ElementParallelGateway, "orcon.parallel@1"), "a": task, "b": task, "j": join(flow.JoinFirst, flow.RemainingCancel)},
			[]string{"L:loop>p", "p>a", "p>b", "a>j", "b>j", "j>L!"}, false,
		},
		{
			"a nested loop before the way back",
			map[string]flow.Node{
				"L2": {Element: flow.ElementExclusiveGateway, Type: "orcon.loop@1", Loop: &flow.Loop{MaxIterations: 2}},
				"a":  task, "b": task,
			},
			[]string{"L:loop>L2", "L2:loop>a", "a>L2!", "L2:loop>b", "b>L!"}, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := loopDoc(t, tt.nodes, tt.edges)
			if err := flow.Validate(doc).Err(); err != nil {
				t.Fatalf("the test document is invalid: %v", err)
			}
			got := flow.LoopsReturningSeveral(doc)["L"]
			if got != tt.several {
				t.Errorf("LoopsReturningSeveral()[L] = %v, want %v", got, tt.several)
			}
		})
	}
}
