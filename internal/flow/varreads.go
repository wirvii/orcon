// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"slices"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// variableReads extends rule V8 to the reads of execution variables
// (ADR-0008 section 4.3, ADR-0018 section 1, SPEC-001 amendment 10, B.3): an
// expression reads one as $execution.variables.name or
// $execution.variables["name"], with a literal name declared in variables[].
// A computed key, the object as a whole, enumeration, destructuring and an
// undeclared name are errors, reported with a JSON Pointer to the field.
func (v *validator) variableReads() {
	declared := make([]string, 0, len(v.doc.Variables))
	for _, vr := range v.doc.Variables {
		declared = append(declared, vr.Name)
	}
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		base := ptr("nodes", i)
		v.valueVariableReads(base+"/parameters", n.Parameters, declared)
		if m := n.MultiInstance; m != nil {
			v.expressionVariableReads(base+"/multiInstance/inputCollection", m.InputCollection, declared)
			v.expressionVariableReads(base+"/multiInstance/completionCondition", m.CompletionCondition, declared)
		}
		if c := n.Call; c != nil && strings.HasPrefix(c.Target, "=") {
			v.expressionVariableReads(base+"/call/target", c.Target, declared)
		}
	}
	for i := range v.doc.Edges {
		v.expressionVariableReads(ptr("edges", i, "condition"), v.doc.Edges[i].Condition, declared)
	}
}

// valueVariableReads walks a parameters value and checks every template
// string in it.
func (v *validator) valueVariableReads(path string, val jsonvalue.Value, declared []string) {
	switch val.Kind() {
	case jsonvalue.KindString:
		s, _ := val.Str()
		v.expressionVariableReads(path, s, declared)
	case jsonvalue.KindArray:
		for i, item := range val.Items() {
			v.valueVariableReads(path+"/"+strconv.Itoa(i), item, declared)
		}
	case jsonvalue.KindObject:
		for _, m := range val.Members() {
			v.valueVariableReads(path+"/"+escapePointer(m.Key), m.Value, declared)
		}
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber:
	}
}

// expressionVariableReads checks one field that may hold an expression: a
// template string, or for an edge condition the bare code.
func (v *validator) expressionVariableReads(path, s string, declared []string) {
	if !strings.Contains(s, "variables") {
		return // cheap exit: no expression reads a variable
	}
	a := analyzeCondition(s)
	for _, f := range a.Findings {
		if f.Kind == expr.FindingExecutionVariables {
			v.r.errorf(CodeV8, path, "%s", f.Message)
		}
	}
	for _, name := range a.Variables {
		if !slices.Contains(declared, name) {
			v.r.errorf(CodeV8, path, "the expression reads $execution.variables.%s, but no variable named %q is declared in variables[]", name, name)
		}
	}
}
