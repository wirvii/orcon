// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func assignmentsParams(members ...jsonvalue.Value) jsonvalue.Value {
	return obj(jsonvalue.Member{Key: "assignments", Value: jsonvalue.Array(members...)})
}

func assignment(members ...jsonvalue.Member) jsonvalue.Value { return obj(members...) }

func member(key string, v jsonvalue.Value) jsonvalue.Member {
	return jsonvalue.Member{Key: key, Value: v}
}

// TestValidateVariablesNodes covers orcon.variables@1, the only node type that
// writes execution variables (ADR-0008 section 4.3, amendment 6). The base
// flow has two concurrent writers of an accumulator and two sequential
// writers of a plain variable; each case breaks one rule.
func TestValidateVariablesNodes(t *testing.T) {
	t.Parallel()
	runRuleCases(t, []ruleCase{
		{"concurrent writers of a plain variable", "variables", func(d *flow.Document) {
			d.Variables[0].Accumulator = ""
		}, flow.CodeV9, flow.SeverityError},
		{"a variable name that is an expression", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = assignmentsParams(assignment(
				member("variable", jsonvalue.String("={{ 'total' }}")), member("value", jsonvalue.Int(1))))
		}, flow.CodeValue, flow.SeverityError},
		{"an undeclared variable", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = assignmentsParams(assignment(
				member("variable", jsonvalue.String("ghost")), member("value", jsonvalue.Int(1))))
		}, flow.CodeReference, flow.SeverityError},
		{"no assignments", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = assignmentsParams()
		}, flow.CodeStructure, flow.SeverityError},
		{"parameters without assignments", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = obj()
		}, flow.CodeStructure, flow.SeverityError},
		{"an assignment without a value", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = assignmentsParams(assignment(member("variable", jsonvalue.String("total"))))
		}, flow.CodeStructure, flow.SeverityError},
		{"an assignment without a variable", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = assignmentsParams(assignment(member("value", jsonvalue.Int(1))))
		}, flow.CodeStructure, flow.SeverityError},
		{"a variable that is not a string", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = assignmentsParams(assignment(
				member("variable", jsonvalue.Int(3)), member("value", jsonvalue.Int(1))))
		}, flow.CodeStructure, flow.SeverityError},
		{"an assignment that is not an object", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = assignmentsParams(jsonvalue.String("total"))
		}, flow.CodeStructure, flow.SeverityError},
		{"assignments that are not an array", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = obj(member("assignments", jsonvalue.String("total")))
		}, flow.CodeStructure, flow.SeverityError},
		{"an unknown assignment member", "variables", func(d *flow.Document) {
			node(d, "left").Parameters = assignmentsParams(assignment(
				member("variable", jsonvalue.String("total")), member("value", jsonvalue.Int(1)), member("op", jsonvalue.String("add"))))
		}, flow.CodeStructure, flow.SeverityError},
		{"an unknown parameter", "variables", func(d *flow.Document) {
			p := node(d, "left").Parameters
			node(d, "left").Parameters = p.With("mode", jsonvalue.String("x"))
		}, flow.CodeStructure, flow.SeverityError},
		{"an element other than task", "variables", func(d *flow.Document) {
			node(d, "left").Element = flow.ElementServiceTask
		}, flow.CodeBlock, flow.SeverityError},
		{"a value that reads a sibling branch", "variables", func(d *flow.Document) {
			node(d, "right").Parameters = assignmentsParams(assignment(
				member("variable", jsonvalue.String("total")), member("value", jsonvalue.String("={{ $('Count left').item.json.n }}"))))
		}, flow.CodeV8, flow.SeverityError},
	})
}

func TestValidateVariablesNodesAcceptSequentialWritesAndAccumulators(t *testing.T) {
	t.Parallel()
	doc := load(t, "variables")
	// Two nodes write the plain variable "status" one after the other, and two
	// concurrent branches write the accumulator "total": both are valid.
	if r := flow.Validate(&doc); len(r.Issues) > 0 {
		t.Fatalf("Validate = %v, want no issues", r.Issues)
	}
	// Writing the same plain variable twice in one node is sequential too.
	node(&doc, "opened").Parameters = assignmentsParams(
		assignment(member("variable", jsonvalue.String("status")), member("value", jsonvalue.String("a"))),
		assignment(member("variable", jsonvalue.String("status")), member("value", jsonvalue.String("b"))))
	if r := flow.Validate(&doc); r.HasErrors() {
		t.Errorf("Validate = %v, want no errors", r.Errors())
	}
}

func TestLoopCarryIsUnchanged(t *testing.T) {
	t.Parallel()
	doc := load(t, "loop")
	if r := flow.Validate(&doc); len(r.Issues) > 0 {
		t.Fatalf("Validate = %v, want no issues", r.Issues)
	}
	node(&doc, "loop").Loop.Carry = []string{"pages"} // an execution variable cannot be carried
	if r := flow.Validate(&doc); !r.Has(flow.CodeReference, flow.SeverityError) {
		t.Errorf("Validate = %v, want a REFERENCE error", r.Issues)
	}
}
