// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// These tests feed the V9 analysis through the validator's writers function
// with a stub, so the graph rules are tested apart from how a node names the
// variables it writes (see [writersOf] and variables_test.go for that).

func loadInternal(t *testing.T, name string) Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "conformance", name+".json")) //nolint:gosec // G304: the name comes from the test tables
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(v)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func nodeByID(d *Document, id string) *Node {
	for i := range d.Nodes {
		if d.Nodes[i].ID == NodeID(id) {
			return &d.Nodes[i]
		}
	}
	panic("node not found: " + id)
}

// stubWriters returns a writers function for the given node-to-variables map.
func stubWriters(byNode map[NodeID][]string) func(*Node) []string {
	return func(n *Node) []string { return byNode[n.ID] }
}

func validateWriters(d *Document, byNode map[NodeID][]string) Report {
	return validate(d, validatorConfig{writers: stubWriters(byNode)})
}

func TestWritersOfReadsVariablesNodes(t *testing.T) {
	t.Parallel()
	doc := loadInternal(t, "variables")
	want := map[NodeID][]string{"left": {"total"}, "right": {"total"}, "opened": {"status"}, "closed": {"status", "total"}}
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		got := writersOf(n)
		if !slices.Equal(got, want[n.ID]) {
			t.Errorf("writersOf(%s) = %v, want %v", n.ID, got, want[n.ID])
		}
	}
	// A node of another type writes nothing, whatever its parameters say.
	other := loadInternal(t, "parallel-join")
	nodeByID(&other, "a").Parameters = doc.Nodes[2].Parameters
	if got := writersOf(nodeByID(&other, "a")); got != nil {
		t.Errorf("writersOf(non-variables node) = %v, want none", got)
	}
}

func TestConcurrentWritersRule(t *testing.T) {
	t.Parallel()
	both := map[NodeID][]string{"a": {"seen"}, "b": {"seen"}}
	tests := []struct {
		name    string
		base    string
		mutate  func(d *Document)
		writers map[NodeID][]string
		want    bool // a V9 error is expected
	}{
		{"parallel branches write a plain variable", "parallel-join", func(d *Document) { d.Variables[0].Accumulator = "" }, both, true},
		{"parallel branches write an accumulator", "parallel-join", func(*Document) {}, both, false},
		{"one writer before a join", "parallel-join", func(d *Document) { d.Variables[0].Accumulator = "" }, map[NodeID][]string{"a": {"seen"}}, false},
		{"a join node writes after both branches", "parallel-join", func(d *Document) { d.Variables[0].Accumulator = "" }, map[NodeID][]string{"join": {"seen"}}, false},
		{"writer after an uncontrolled merge", "parallel-join", func(d *Document) {
			d.Variables[0].Accumulator = ""
			nodeByID(d, "join").Join = nil
		}, map[NodeID][]string{"join": {"seen"}}, true},
		{"parallel multi-instance body", "multi-instance", func(d *Document) { d.Variables[0].Accumulator = "" }, map[NodeID][]string{"each": {"processed"}}, true},
		{"parallel multi-instance body with an accumulator", "multi-instance", func(*Document) {}, map[NodeID][]string{"each": {"processed"}}, false},
		{"sequential multi-instance body", "multi-instance", func(d *Document) {
			d.Variables[0].Accumulator = ""
			nodeByID(d, "each").MultiInstance.Sequential = true
		}, map[NodeID][]string{"each": {"processed"}}, false},
		{"multi-instance with concurrency one", "multi-instance", func(d *Document) {
			d.Variables[0].Accumulator = ""
			nodeByID(d, "each").MultiInstance.Concurrency = &IntOrKeyword{N: 1}
			nodeByID(d, "each").MultiInstance.Sequential = true
		}, map[NodeID][]string{"each": {"processed"}}, false},
		{"exclusive gateway branches never overlap", "exclusive-and-inclusive", func(d *Document) {
			d.Variables = append(d.Variables, Variable{Name: "size", Scope: ScopeExecution})
		}, map[NodeID][]string{"small": {"size"}, "large": {"size"}}, false},
		{"inclusive gateway branches may overlap", "exclusive-and-inclusive", func(d *Document) {
			d.Variables = append(d.Variables, Variable{Name: "sent", Scope: ScopeExecution})
		}, map[NodeID][]string{"mail": {"sent"}, "chat": {"sent"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := loadInternal(t, tt.base)
			tt.mutate(&doc)
			r := validateWriters(&doc, tt.writers)
			if got := r.Has(CodeV9, SeverityError); got != tt.want {
				t.Errorf("V9 error = %v, want %v; issues %v", got, tt.want, r.Issues)
			}
		})
	}
}

func TestWritersMustBeDeclaredVariables(t *testing.T) {
	t.Parallel()
	doc := loadInternal(t, "parallel-join")
	r := validateWriters(&doc, map[NodeID][]string{"a": {"nope"}})
	if !r.Has(CodeReference, SeverityError) {
		t.Errorf("issues = %v, want a REFERENCE error for the undeclared variable", r.Issues)
	}
}

func TestWritersFunctionIsTheOnlySourceOfWrites(t *testing.T) {
	t.Parallel()
	doc := loadInternal(t, "parallel-join")
	doc.Variables[0].Accumulator = ""
	calls := 0
	cfg := validatorConfig{writers: func(*Node) []string { calls++; return nil }}
	validate(&doc, cfg)
	if calls < len(doc.Nodes) {
		t.Errorf("writers function called %d times for %d nodes", calls, len(doc.Nodes))
	}
}
