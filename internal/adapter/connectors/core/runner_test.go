// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
)

func input(t *testing.T, items ...string) execution.Batch {
	t.Helper()
	out := make([]execution.Item, 0, len(items))
	for i, s := range items {
		out = append(out, execution.Item{
			JSON:    mustParse(t, s),
			Lineage: []execution.LineageRef{{Port: "main", Index: i}},
		})
	}
	return execution.NewBatch(out...)
}

func portItems(t *testing.T, out execution.NodeOutput, port string) []execution.Item {
	t.Helper()
	for _, p := range out.Ports {
		if p.Port == port {
			return p.Batch.Items()
		}
	}
	t.Fatalf("no output on port %q", port)
	return nil
}

func TestCoreNodesRunnerNoOpPassesItemsThrough(t *testing.T) {
	in := input(t, `{"a":1}`, `{"b":2}`)
	out, err := NewRunner(nil).Execute(t.Context(), execution.NodeCall{
		Node: &flow.Node{Type: TypeNoOp}, Input: in,
	})
	if err != nil {
		t.Fatal(err)
	}
	items := portItems(t, out, "main")
	if len(items) != 2 || marshal(t, items[0].JSON) != `{"a":1}` || marshal(t, items[1].JSON) != `{"b":2}` {
		t.Fatalf("items = %v", items)
	}
	if items[1].Lineage[0].Index != 1 {
		t.Errorf("lineage not kept: %+v", items[1].Lineage)
	}
}

func TestCoreNodesRunnerSetTransformsEveryItem(t *testing.T) {
	in := input(t, `{"id":1}`, `{"id":2}`)
	node := &flow.Node{Type: TypeSet, Parameters: mustParse(t, `{"includeOtherFields":true,"assignments":[{"name":"ok","value":"=$json.id"}]}`)}
	out, err := NewRunner(&fakeEval{value: mustParse(t, `true`)}).Execute(t.Context(), execution.NodeCall{Node: node, Input: in})
	if err != nil {
		t.Fatal(err)
	}
	items := portItems(t, out, "main")
	if len(items) != 2 || marshal(t, items[0].JSON) != `{"id":1,"ok":true}` || marshal(t, items[1].JSON) != `{"id":2,"ok":true}` {
		t.Fatalf("items = %v", items)
	}
	if items[0].Lineage[0].Index != 0 {
		t.Errorf("lineage not kept")
	}
}

func TestCoreNodesRunnerSetFailsWhenAnItemFails(t *testing.T) {
	in := input(t, `{"id":1}`, `[]`)
	node := &flow.Node{Type: TypeSet, Parameters: mustParse(t, `{"assignments":[{"name":"x","value":1}]}`)}
	_, err := NewRunner(nil).Execute(t.Context(), execution.NodeCall{Node: node, Input: in})
	var ne *execution.NodeError
	if !errors.As(err, &ne) || ne.Code != "SET_FAILED" || ne.Class != execution.ClassPermanent {
		t.Fatalf("err = %v, want SET_FAILED permanent", err)
	}
}

func TestCoreNodesRunnerSetRejectsInvalidParameters(t *testing.T) {
	node := &flow.Node{Type: TypeSet, Parameters: mustParse(t, `{"bogus":1}`)}
	_, err := NewRunner(nil).Execute(t.Context(), execution.NodeCall{Node: node, Input: input(t, `{}`)})
	var ne *execution.NodeError
	if !errors.As(err, &ne) || ne.Code != codeInvalidParameters {
		t.Fatalf("err = %v, want %s", err, codeInvalidParameters)
	}
}

func TestCoreNodesRunnerRejectsUnknownTypes(t *testing.T) {
	r := NewRunner(nil)
	if _, err := r.Execute(t.Context(), execution.NodeCall{Node: &flow.Node{Type: "orcon.http@1"}}); err == nil {
		t.Error("an HTTP node reached the core runner")
	}
	if _, err := r.Execute(t.Context(), execution.NodeCall{}); err == nil {
		t.Error("a call without a node was accepted")
	}
}
