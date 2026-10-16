// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Types of the data nodes of SPEC-003 Annex B (CN13).
const (
	typeOrconNoop = "orcon.noop@1"
	typeOrconSet  = "orcon.set@1"
)

// convertNoOp maps No Operation to orcon.noop@1. Both pass their input through
// unchanged, so the node converts without a finding.
func (c *converter) convertNoOp(n *Node) convResult {
	return convResult{
		node: flow.Node{
			Element: flow.ElementTask, Type: typeOrconNoop, Parameters: emptyObject(),
			Ext: importMembers(n),
		},
		orconType: typeOrconNoop,
	}
}

// convertSet maps Edit Fields (Set) in manual mode, version 3 and later, to
// orcon.set@1. Each field keeps its name and its value. A field whose meaning
// Orcon cannot keep makes the whole node unsupported, and its finding names the
// field: the converter never drops or approximates a field silently.
func (c *converter) convertSet(n *Node) convResult {
	if ver, _ := strconv.ParseFloat(n.TypeVersion, 64); ver < 3 {
		return c.unsupported(n, []Finding{paramFinding(n, "typeVersion",
			"Edit Fields versions 1 and 2 keep their fields in another shape; only version 3 and later are converted")}, flow.ElementTask)
	}
	// A secret anywhere in the node (by field name or by value) keeps the node
	// unsupported; redactNode reports each field and the original stays redacted.
	if _, secrets := redactNode(n); len(secrets) > 0 {
		return c.unsupported(n, nil, flow.ElementTask)
	}
	p := params(n)
	if mode, ok := p.Get("mode"); ok {
		if s, _ := mode.Str(); s != "manual" {
			return c.unsupported(n, []Finding{paramFinding(n, "parameters.mode",
				"only the manual mode is converted; the raw JSON mode and expressions in the mode are not")}, flow.ElementTask)
		}
	}
	if opts, ok := p.Get("options"); ok && opts.Kind() == jsonvalue.KindObject && len(opts.Members()) > 0 {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.options",
			"the Edit Fields options are not converted; remove them or rebuild the node")}, flow.ElementTask)
	}
	include := false
	if v, ok := p.Get("includeOtherFields"); ok {
		b, isBool := v.Bool()
		if !isBool {
			return c.unsupported(n, []Finding{paramFinding(n, "parameters.includeOtherFields",
				"includeOtherFields must be true or false")}, flow.ElementTask)
		}
		include = b
	}
	items, problem := setAssignments(n, p)
	if problem != nil {
		return c.unsupported(n, []Finding{*problem}, flow.ElementTask)
	}

	var findings []Finding
	fields := make([]jsonvalue.Value, 0, len(items))
	for i, it := range items {
		field := fmt.Sprintf("parameters.assignments.assignments[%d]", i)
		out, adapted, blocking := setField(n, it, field)
		if blocking != nil {
			return c.unsupported(n, []Finding{*blocking}, flow.ElementTask)
		}
		fields = append(fields, out)
		if adapted != nil {
			findings = append(findings, *adapted)
		}
	}

	parameters, err := jsonvalue.Object(
		jsonvalue.Member{Key: "assignments", Value: jsonvalue.Array(fields...)},
		jsonvalue.Member{Key: "includeOtherFields", Value: jsonvalue.Bool(include)},
	)
	if err != nil {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters", err.Error())}, flow.ElementTask)
	}
	return convResult{
		node: flow.Node{
			Element: flow.ElementTask, Type: typeOrconSet, Parameters: parameters,
			Ext: importMembers(n),
		},
		orconType: typeOrconSet,
		findings:  findings,
	}
}

// setAssignments reads parameters.assignments.assignments, the list of fields.
// A missing list means no fields, which n8n also runs.
func setAssignments(n *Node, p jsonvalue.Value) ([]jsonvalue.Value, *Finding) {
	a, ok := p.Get("assignments")
	if !ok {
		return nil, nil
	}
	if a.Kind() != jsonvalue.KindObject {
		f := paramFinding(n, "parameters.assignments", "the assignments must be an object")
		return nil, &f
	}
	list, ok := a.Get("assignments")
	if !ok {
		return nil, nil
	}
	if list.Kind() != jsonvalue.KindArray {
		f := paramFinding(n, "parameters.assignments.assignments", "the fields must be a list")
		return nil, &f
	}
	return list.Items(), nil
}

// setField converts one field. It returns the orcon field, an optional finding
// that keeps the node converted, and a blocking finding that makes it
// unsupported.
func setField(n *Node, it jsonvalue.Value, field string) (jsonvalue.Value, *Finding, *Finding) {
	name, ok := stringOf(it, "name")
	if !ok || name == "" || strings.HasPrefix(name, "=") || strings.Contains(name, ".") {
		f := paramFinding(n, field+".name",
			"a field name must be a literal name without dots; nested paths and expressions as names are not converted")
		return jsonvalue.Value{}, nil, &f
	}
	value, ok := it.Get("value")
	if !ok {
		f := paramFinding(n, field+".value", "the field "+name+" has no value")
		return jsonvalue.Value{}, nil, &f
	}
	typ := ""
	if t, ok := it.Get("type"); ok {
		s, _ := t.Str()
		if !declaredType(s) {
			f := paramFinding(n, field+".type", "the type "+s+" of the field "+name+" is not converted")
			return jsonvalue.Value{}, nil, &f
		}
		typ = s
	}
	if typ != "" && !literalMatchesType(value, typ) {
		f := paramFinding(n, field+".value", "the literal value of the field "+name+" does not match its declared type "+typ)
		return jsonvalue.Value{}, nil, &f
	}
	out, err := jsonvalue.Object(
		jsonvalue.Member{Key: "name", Value: jsonvalue.String(name)},
		jsonvalue.Member{Key: "value", Value: value},
	)
	if err != nil {
		f := paramFinding(n, field, err.Error())
		return jsonvalue.Value{}, nil, &f
	}
	if containsExpression(value) {
		adapted := Finding{Code: FindAdapted, NodeID: n.ID, Node: n.Name, Field: field + ".value",
			Message: "the expression of the field " + name + " keeps n8n's template syntax and runs per item; n8n converts its result to the declared type and Orcon does not"}
		return out, &adapted, nil
	}
	return out, nil, nil
}

func declaredType(s string) bool {
	switch s {
	case "string", "number", "boolean", "array", "object":
		return true
	default:
		return false
	}
}

func literalMatchesType(v jsonvalue.Value, typ string) bool {
	switch typ {
	case "string":
		return v.Kind() == jsonvalue.KindString
	case "number":
		return v.Kind() == jsonvalue.KindNumber
	case "boolean":
		return v.Kind() == jsonvalue.KindBool
	case "array":
		return v.Kind() == jsonvalue.KindArray
	case "object":
		return v.Kind() == jsonvalue.KindObject
	default:
		return false
	}
}

// stringOf reads a string member of an object.
func stringOf(v jsonvalue.Value, key string) (string, bool) {
	m, ok := v.Get(key)
	if !ok {
		return "", false
	}
	return m.Str()
}

// containsExpression reports whether any string inside v starts with "=", the
// n8n marker of an expression.
func containsExpression(v jsonvalue.Value) bool {
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		return strings.HasPrefix(s, "=")
	case jsonvalue.KindArray:
		for _, it := range v.Items() {
			if containsExpression(it) {
				return true
			}
		}
	case jsonvalue.KindObject:
		for _, m := range v.Members() {
			if containsExpression(m.Value) {
				return true
			}
		}
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber:
	}
	return false
}
