// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
)

// withConfigNode puts a node named "Config" between the start event and the
// loop, so a condition can read a node outside the loop body.
func withConfigNode(doc *flow.Document) {
	doc.Nodes = append(doc.Nodes, flow.Node{
		ID: "config", Name: "Config", Element: flow.ElementTask, Type: "orcon.set@1", Parameters: obj(),
	})
	edge(doc, "e1").To = "config"
	doc.Edges = append(doc.Edges, flow.Edge{
		ID: "e0", From: flow.Endpoint{Node: "config", Port: "main"}, To: "loop", Kind: flow.EdgeSequence,
	})
}

func setCondition(doc *flow.Document, template string) {
	node(doc, "more").Parameters = params(template)
}

// TestLoopExitWarning covers rule V13 (ADR-0010 section 11, criterion
// E-FMT-5): a loop whose exit condition reads nothing the body writes
// publishes with a warning stored with the version; if it reads something the
// body writes, there is no warning. The warning never blocks publication.
func TestLoopExitWarning(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(d *flow.Document)
		warns  bool
	}{
		{"the conformance loop reads a node of its body", func(*flow.Document) {}, false},
		{"reads the data flowing in the body", func(d *flow.Document) { setCondition(d, "={{ $json.hasMore }}") }, false},
		{"reads the iteration counter", func(d *flow.Document) { setCondition(d, "={{ $runIndex < 3 }}") }, false},
		{"reads a body node through the legacy syntax", func(d *flow.Document) { setCondition(d, "={{ $node['Fetch page'].json.more }}") }, false},
		{"reads a body node guarded by isExecuted", func(d *flow.Document) {
			setCondition(d, "={{ $('Fetch page').isExecuted && $('Fetch page').item.json.more }}")
		}, false},
		{"reads only the clock", func(d *flow.Document) { setCondition(d, "={{ $now.hour < 12 }}") }, true},
		{"is a constant", func(d *flow.Document) { setCondition(d, "={{ true }}") }, true},
		{"reads only workflow metadata", func(d *flow.Document) { setCondition(d, "={{ $workflow.name == 'x' }}") }, true},
		{"reads only a node outside the body", func(d *flow.Document) {
			withConfigNode(d)
			setCondition(d, "={{ $('Config').first().json.limit > 3 }}")
		}, true},
		{"reads a node outside the body and one inside", func(d *flow.Document) {
			withConfigNode(d)
			setCondition(d, "={{ $('Config').first().json.limit > $('Fetch page').item.json.n }}")
		}, false},
		{"is not an expression", func(d *flow.Document) { setCondition(d, "done") }, true},
		{"cannot be classified: a syntax error", func(d *flow.Document) { setCondition(d, "={{ $now. }}") }, false},
		{"unclassified name", func(d *flow.Document) { setCondition(d, "={{ mystery }}") }, false},
		{"cannot be classified: a node chosen at run time", func(d *flow.Document) { setCondition(d, "={{ $($json.name).first().json.x }}") }, false},
		{"is decided by an edge condition on the exit edge", func(d *flow.Document) {
			node(d, "more").Parameters = obj()
			edge(d, "e5").Condition = "={{ $json.done }}"
		}, false},
		{"is decided by an edge condition that reads nothing", func(d *flow.Document) {
			node(d, "more").Parameters = obj()
			edge(d, "e5").Condition = "={{ $now.hour > 3 }}"
		}, true},
		{"no exit edge", func(d *flow.Document) {
			dropEdge(d, "e5")
			d.Nodes = d.Nodes[:len(d.Nodes)-1] // the end event, now unreachable
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, "loop")
			tc.mutate(&doc)
			report := flow.Validate(&doc)
			if report.HasErrors() {
				t.Fatalf("the mutation broke another rule: %v", report.Errors())
			}
			if got := report.Has(flow.CodeV13, flow.SeverityWarning); got != tc.warns {
				t.Errorf("V13 warning = %v, want %v; issues: %v", got, tc.warns, report.Issues)
			}
		})
	}
}

func TestLoopExitWarningIsStoredWithTheVersion(t *testing.T) {
	t.Parallel()
	doc := load(t, "loop")
	setCondition(&doc, "={{ $now.hour < 12 }}")
	f := flow.NewFlow("loop")
	if _, err := f.SaveDraft(0, doc.Value()); err != nil {
		t.Fatal(err)
	}
	v, err := f.Publish()
	if err != nil {
		t.Fatalf("a V13 warning must not block publication: %v", err)
	}
	var found *flow.Issue
	for i := range v.Warnings {
		if v.Warnings[i].Code == flow.CodeV13 {
			found = &v.Warnings[i]
		}
	}
	if found == nil {
		t.Fatalf("Warnings = %v, want a V13 warning stored with the version", v.Warnings)
	}
	if found.Path != "/nodes/1" || !strings.Contains(found.Message, `"loop"`) {
		t.Errorf("warning = %+v, want it to point at the loop node", *found)
	}

	// With a reference to the body, the version carries no V13 warning.
	good := load(t, "loop")
	f2 := flow.NewFlow("loop")
	if _, err := f2.SaveDraft(0, good.Value()); err != nil {
		t.Fatal(err)
	}
	v2, err := f2.Publish()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range v2.Warnings {
		if w.Code == flow.CodeV13 {
			t.Errorf("unexpected V13 warning %v", w)
		}
	}
}
