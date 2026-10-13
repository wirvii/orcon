// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// TestValidateVariableReads covers rule V8 for execution variables
// (SPEC-001 amendment 10, B.3, criterion E-FMT-5): a literal declared name is
// accepted, in the dot and in the bracket form; any other use is an error with
// a JSON Pointer to the field.
func TestValidateVariableReads(t *testing.T) {
	t.Parallel()
	bad := []struct{ name, code string }{
		{"an undeclared name", "={{ $execution.variables.ghost }}"},
		{"an undeclared name in brackets", `={{ $execution.variables["ghost"] }}`},
		{"a computed key", "={{ $execution.variables[$json.k] }}"},
		{"the whole object", "={{ JSON.stringify($execution.variables) }}"},
		{"enumeration", "={{ Object.keys($execution.variables) }}"},
		{"destructuring", "={{ (() => { const { total } = $execution.variables; return total; })() }}"},
		{"the in operator", "={{ 'total' in $execution.variables }}"},
		{"an alias of $execution", "={{ (() => { const e = $execution; return e.variables.total; })() }}"},
		{"destructuring $execution", "={{ (() => { const { variables } = $execution; return variables.total; })() }}"},
		{"a built key into $execution", "={{ $execution['vari' + 'ables'].total }}"},
		{"spreading $execution", "={{ ({ ...$execution }).variables.total }}"},
		{"keys of $execution", "={{ Object.keys($execution) }}"},
		{"passing $execution to a function", "={{ JSON.stringify($execution) }}"},
		{"an unknown member of $execution", "={{ $execution.customData }}"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, "loop")
			setCondition(&doc, tc.code)
			report := flow.Validate(&doc)
			var found *flow.Issue
			for i := range report.Issues {
				if report.Issues[i].Code == flow.CodeV8 && report.Issues[i].Severity == flow.SeverityError {
					found = &report.Issues[i]
				}
			}
			if found == nil {
				t.Fatalf("want a V8 error, got %v", report.Issues)
			}
			if !strings.HasPrefix(found.Path, "/nodes/3/parameters/") {
				t.Errorf("path = %q, want a pointer into the parameters of the reading node", found.Path)
			}
		})
	}
	good := []struct{ name, code string }{
		{"a declared name", "={{ $execution.variables.total > 3 }}"},
		{"a declared name in brackets", `={{ $execution.variables["pages"].length > 0 }}`},
		{"the same name twice", "={{ $execution.variables.total + $execution.variables.total }}"},
		{"other members of $execution", "={{ $execution.id + $execution.mode + $execution.resumeUrl + $execution.resumeFormUrl }}"},
	}
	for _, tc := range good {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, "loop")
			setCondition(&doc, tc.code)
			if report := flow.Validate(&doc); report.Has(flow.CodeV8, flow.SeverityError) {
				t.Errorf("unexpected V8 error: %v", report.Issues)
			}
		})
	}
	t.Run("an edge condition", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "loop")
		node(&doc, "more").Parameters = obj()
		edge(&doc, "e5").Condition = "={{ $execution.variables.ghost }}"
		report := flow.Validate(&doc)
		if !report.Has(flow.CodeV8, flow.SeverityError) {
			t.Fatalf("want a V8 error, got %v", report.Issues)
		}
		for _, is := range report.Issues {
			if is.Code == flow.CodeV8 && is.Path != "/edges/4/condition" {
				t.Errorf("path = %q, want /edges/4/condition", is.Path)
			}
		}
	})
}

// withBodyWriter puts a variables node in the loop body that writes name.
func withBodyWriter(doc *flow.Document, name string) {
	doc.Nodes = append(doc.Nodes, flow.Node{
		ID: "note", Name: "Note", Element: flow.ElementTask, Type: "orcon.variables@1",
		Parameters: assignmentsParams(assignment(member("variable", jsonvalue.String(name)), member("value", jsonvalue.Int(1)))),
	})
	edge(doc, "e3").To = "note"
	doc.Edges = append(doc.Edges, flow.Edge{
		ID: "e3b", From: flow.Endpoint{Node: "note", Port: "main"}, To: "more", Kind: flow.EdgeSequence,
	})
}

// TestLoopExitReadsVariables covers the variable part of V13 (ADR-0010
// section 11): a condition that reads an execution variable counts as reading
// something the body writes when a node of the body writes that variable or
// the loop carries it.
func TestLoopExitReadsVariables(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(d *flow.Document)
		warns  bool
	}{
		{"a variable the loop carries", func(d *flow.Document) { setCondition(d, "={{ $execution.variables.total < 10 }}") }, false},
		{"a variable the loop carries, in brackets", func(d *flow.Document) { setCondition(d, `={{ $execution.variables["total"] < 10 }}`) }, false},
		{"a variable no node of the body writes", func(d *flow.Document) { setCondition(d, "={{ $execution.variables.pages.length > 3 }}") }, true},
		{"a variable a node of the body writes", func(d *flow.Document) {
			withBodyWriter(d, "pages")
			setCondition(d, "={{ $execution.variables.pages.length > 3 }}")
		}, false},
		{"a variable written outside the body", func(d *flow.Document) {
			d.Nodes = append(d.Nodes, flow.Node{
				ID: "init", Name: "Init", Element: flow.ElementTask, Type: "orcon.variables@1",
				Parameters: assignmentsParams(assignment(member("variable", jsonvalue.String("pages")), member("value", jsonvalue.Int(1)))),
			})
			edge(d, "e1").To = "init"
			d.Edges = append(d.Edges, flow.Edge{ID: "e0", From: flow.Endpoint{Node: "init", Port: "main"}, To: "loop", Kind: flow.EdgeSequence})
			setCondition(d, "={{ $execution.variables.pages.length > 3 }}")
		}, true},
		{"a variable used as a whole cannot be classified", func(d *flow.Document) {
			setCondition(d, "={{ Object.keys($execution.variables).length > 0 }}")
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, "loop")
			tc.mutate(&doc)
			report := flow.Validate(&doc)
			// A use of the whole object is a V8 error but must stay silent in V13.
			if got := report.Has(flow.CodeV13, flow.SeverityWarning); got != tc.warns {
				t.Errorf("V13 warning = %v, want %v; issues: %v", got, tc.warns, report.Issues)
			}
		})
	}
}
