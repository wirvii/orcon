// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// typeVariables is the built-in node type that writes execution variables
// (ADR-0008 section 4.3, amendment 6). It belongs to the format, like
// orcon.loop, and is the only type that writes execution variables.
const typeVariables = "orcon.variables"

// TypeVariables is the full type name of the variables node, as written in a
// document.
const TypeVariables = typeVariables + "@1"

// Assignment is one write of an orcon.variables node.
type Assignment struct {
	// Variable is the literal name of a variable declared in variables[].
	Variable string
	// Value is a literal or an expression; for an accumulator variable it is
	// the delta.
	Value jsonvalue.Value
}

// assignmentsOf reads the assignments of an orcon.variables node and
// reports the shape problems of its parameters, with pointers relative to the
// node. It returns nothing for any other node type.
func assignmentsOf(n *Node, base string) ([]Assignment, []Issue) {
	if typeName(n.Type) != typeVariables {
		return nil, nil
	}
	var issues []Issue
	fail := func(severity Severity, code, path, msg string) {
		issues = append(issues, Issue{Severity: severity, Code: code, Path: path, Message: msg})
	}
	params := n.Parameters
	if params.Kind() != jsonvalue.KindObject {
		return nil, nil // already reported as a structure error
	}
	for _, m := range params.Members() {
		if m.Key != "assignments" {
			fail(SeverityError, CodeStructure, base+"/parameters/"+escapePointer(m.Key), "unknown property "+strconv.Quote(m.Key))
		}
	}
	list, ok := params.Get("assignments")
	if !ok {
		fail(SeverityError, CodeStructure, base+"/parameters/assignments", "missing required property \"assignments\"")
		return nil, issues
	}
	items := list.Items()
	if list.Kind() != jsonvalue.KindArray || len(items) == 0 {
		fail(SeverityError, CodeStructure, base+"/parameters/assignments", "must be an array with at least one assignment")
		return nil, issues
	}
	out := make([]Assignment, 0, len(items))
	for i, item := range items {
		path := base + "/parameters/assignments/" + strconv.Itoa(i)
		a, problems := readAssignment(item, path)
		issues = append(issues, problems...)
		if len(problems) == 0 {
			out = append(out, a)
		}
	}
	return out, issues
}

func readAssignment(item jsonvalue.Value, path string) (Assignment, []Issue) {
	var issues []Issue
	fail := func(code, p, msg string) {
		issues = append(issues, Issue{Severity: SeverityError, Code: code, Path: p, Message: msg})
	}
	if item.Kind() != jsonvalue.KindObject {
		fail(CodeStructure, path, "must be an object with variable and value")
		return Assignment{}, issues
	}
	for _, m := range item.Members() {
		if m.Key != "variable" && m.Key != "value" {
			fail(CodeStructure, path+"/"+escapePointer(m.Key), "unknown property "+strconv.Quote(m.Key))
		}
	}
	a := Assignment{}
	name, hasName := item.Get("variable")
	if !hasName {
		fail(CodeStructure, path+"/variable", "missing required property \"variable\"")
	} else if s, isStr := name.Str(); !isStr {
		fail(CodeStructure, path+"/variable", "must be a string")
	} else if strings.HasPrefix(s, "=") || !variablePattern.MatchString(s) {
		fail(CodeValue, path+"/variable", "the variable must be a literal name declared in variables[], not an expression or a malformed name: "+strconv.Quote(s))
	} else {
		a.Variable = s
	}
	value, hasValue := item.Get("value")
	if !hasValue {
		fail(CodeStructure, path+"/value", "missing required property \"value\"")
	}
	a.Value = value
	return a, issues
}

// writersOf lists the execution variables a node writes. It is the single
// place V9 and level-4 validation learn who writes what: only a node of the
// built-in type orcon.variables@1 writes variables, and it names them in
// parameters.assignments[].variable. Every other type writes none.
func writersOf(n *Node) []string {
	assignments, _ := assignmentsOf(n, "")
	names := make([]string, 0, len(assignments))
	for _, a := range assignments {
		names = append(names, a.Variable)
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// variablesShape checks the shape of orcon.variables nodes: the element must
// be task and the parameters must hold well-formed assignments.
func (v *validator) variablesShape(i int, n *Node) {
	if typeName(n.Type) != typeVariables {
		return
	}
	base := ptr("nodes", i)
	if n.Element != ElementTask {
		v.r.errorf(CodeBlock, base+"/element", "%s nodes have element task, not %s", typeVariables, n.Element)
	}
	_, issues := assignmentsOf(n, base)
	v.r.issues = append(v.r.issues, issues...)
}
