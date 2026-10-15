// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// loopMember returns the loop block of the node with the given identifier in a written document.
func loopMember(t *testing.T, v jsonvalue.Value, id string) jsonvalue.Value {
	t.Helper()
	nodes, ok := v.Get("nodes")
	if !ok {
		t.Fatal("the written document has no nodes")
	}
	for _, n := range nodes.Items() {
		nid, _ := n.Get("id")
		if s, _ := nid.Str(); s == id {
			loop, ok := n.Get("loop")
			if !ok {
				t.Fatalf("node %s has no loop block", id)
			}
			return loop
		}
	}
	t.Fatalf("node %s is not in the written document", id)
	return jsonvalue.Value{}
}

// An empty carry list is not written: the reader requires at least one item in a list, so a
// writer that emitted "carry": [] would produce a document the reader rejects (BL-101). A carry
// list with items is still written.
func TestEmptyLoopCarryIsOmittedAndTheDocumentReadsBack(t *testing.T) {
	t.Parallel()
	doc := load(t, "loop")
	node(&doc, "loop").Loop.Carry = []string{}

	v := doc.Value()
	if _, has := loopMember(t, v, "loop").Get("carry"); has {
		t.Error("an empty carry list was written out; the reader rejects an empty list")
	}
	back, err := flow.FromValue(v)
	if err != nil {
		t.Fatalf("FromValue of the written document = %v, want the document to read back", err)
	}
	got := node(&back, "loop").Loop
	if len(got.Carry) != 0 || got.MaxIterations != 50 {
		t.Errorf("loop after the round trip = %+v, want no carry and maxIterations 50", got)
	}
}

// A non-empty carry list survives the round trip unchanged.
func TestLoopCarryWithItemsIsWrittenAndRoundTrips(t *testing.T) {
	t.Parallel()
	doc := load(t, "loop")
	want := node(&doc, "loop").Loop.Carry
	if len(want) == 0 {
		t.Fatal("the loop fixture no longer carries a variable")
	}

	v := doc.Value()
	if _, has := loopMember(t, v, "loop").Get("carry"); !has {
		t.Error("a carry list with items was not written")
	}
	back, err := flow.FromValue(v)
	if err != nil {
		t.Fatalf("FromValue() = %v", err)
	}
	got := node(&back, "loop").Loop.Carry
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("carry after the round trip = %v, want %v", got, want)
	}
}
