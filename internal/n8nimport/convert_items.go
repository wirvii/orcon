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

// Orcon types of the item-list nodes of SPEC-003 Annex B (CN13).
const (
	typeOrconFilter = "orcon.filter@1"
	typeOrconSort   = "orcon.sort@1"
	typeOrconLimit  = "orcon.limit@1"
)

// object builds a JSON object from members. The keys of every call site are
// distinct literals, so the error of a duplicate key cannot happen.
func object(members ...jsonvalue.Member) jsonvalue.Value {
	v, _ := jsonvalue.Object(members...)
	return v
}

func member(key string, v jsonvalue.Value) jsonvalue.Member {
	return jsonvalue.Member{Key: key, Value: v}
}

// unsupportedParam is the result of a node that has a parameter Orcon cannot
// keep: the finding names the parameter.
func (c *converter) unsupportedParam(n *Node, field, msg string) convResult {
	return c.unsupported(n, []Finding{paramFinding(n, field, msg)}, flow.ElementTask)
}

// adapted is a finding that keeps the node converted and states a difference.
func adapted(n *Node, field, msg string) Finding {
	return Finding{Code: FindAdapted, NodeID: n.ID, Node: n.Name, Field: field, Message: msg}
}

// itemNode is the converted node of a pure item-list type.
func itemNode(n *Node, typ string, parameters jsonvalue.Value, findings ...Finding) convResult {
	return convResult{
		node:      flow.Node{Element: flow.ElementTask, Type: typ, Parameters: parameters, Ext: importMembers(n)},
		orconType: typ,
		findings:  findings,
	}
}

// versionAtLeast reports whether the typeVersion of the node is at least min.
func versionAtLeast(n *Node, minimum float64) bool {
	ver, err := strconv.ParseFloat(n.TypeVersion, 64)
	return err == nil && ver >= minimum
}

// convertFilter maps a Filter node of version 2 or later. Its conditions are
// translated like the conditions of an IF node, into the one boolean
// expression that orcon.filter@1 takes as its condition.
func (c *converter) convertFilter(n *Node) convResult {
	if !versionAtLeast(n, 2) {
		return c.unsupportedParam(n, "typeVersion", "Filter version 1 keeps its conditions in another shape; only version 2 and later are converted")
	}
	if _, secrets := redactNode(n); len(secrets) > 0 {
		return c.unsupported(n, nil, flow.ElementTask)
	}
	p := params(n)
	cond, ok := p.Get("conditions")
	if !ok || cond.Kind() != jsonvalue.KindObject {
		return c.unsupportedParam(n, "parameters.conditions", "the Filter node has no conditions object")
	}
	if opts, ok := p.Get("options"); ok && opts.Kind() == jsonvalue.KindObject {
		for _, m := range opts.Members() {
			switch m.Key {
			case "ignoreCase":
				if b, _ := m.Value.Bool(); b {
					cond = withCaseInsensitive(cond)
				}
			case "looseTypeValidation":
			default:
				return c.unsupportedParam(n, "parameters.options."+m.Key,
					"the Filter option "+m.Key+" is not converted; remove it or rebuild the node")
			}
		}
	}
	js, err := filterJS(cond)
	if err != nil {
		return c.unsupported(n, []Finding{filterFinding(n, "parameters.conditions", err)}, flow.ElementTask)
	}
	return itemNode(n, typeOrconFilter, object(member("condition", jsonvalue.String(whenParam(js)))))
}

// withCaseInsensitive returns the filter with its case-sensitivity option off.
func withCaseInsensitive(cond jsonvalue.Value) jsonvalue.Value {
	opts, ok := cond.Get("options")
	if !ok || opts.Kind() != jsonvalue.KindObject {
		opts = object()
	}
	return cond.With("options", opts.With("caseSensitive", jsonvalue.Bool(false)))
}

// convertLimit maps a Limit node. Both its parameters must be literals.
func (c *converter) convertLimit(n *Node) convResult {
	p := params(n)
	maxItems := int64(1)
	if v, ok := p.Get("maxItems"); ok {
		got, isInt := v.Int64()
		if !isInt || got < 1 || got > 1_000_000_000 {
			return c.unsupportedParam(n, "parameters.maxItems", "the maximum must be a whole number of at least 1 and not an expression")
		}
		maxItems = got
	}
	keep := "FIRST_ITEMS"
	if v, ok := p.Get("keep"); ok {
		s, _ := v.Str()
		switch s {
		case "firstItems":
		case "lastItems":
			keep = "LAST_ITEMS"
		default:
			return c.unsupportedParam(n, "parameters.keep", "keep must be firstItems or lastItems and not an expression")
		}
	}
	return itemNode(n, typeOrconLimit, object(
		member("maxItems", jsonvalue.Int(maxItems)),
		member("keep", jsonvalue.String(keep)),
	))
}

// convertSort maps a Sort node in its simple and random types. The code type
// runs JavaScript and waits for the Code node.
func (c *converter) convertSort(n *Node) convResult {
	p := params(n)
	typ, _ := stringParam(p, "type")
	if v, ok := p.Get("type"); ok && typ == "" {
		if s, _ := v.Str(); s != "" {
			return c.unsupportedParam(n, "parameters.type", "the sort type must be simple or random and not an expression")
		}
	}
	switch typ {
	case "random":
		return itemNode(n, typeOrconSort, object(member("mode", jsonvalue.String("RANDOM"))))
	case "code":
		return c.unsupportedParam(n, "parameters.type", "the code type sorts with JavaScript, which needs the Code node; use the simple type")
	case "", "simple":
	default:
		return c.unsupportedParam(n, "parameters.type", "the sort type "+typ+" is not converted")
	}
	ui, _ := p.Get("sortFieldsUi")
	list, _ := ui.Get("sortField")
	if list.Kind() != jsonvalue.KindArray || list.Len() == 0 {
		return c.unsupportedParam(n, "parameters.sortFieldsUi.sortField", "the Sort node has no fields to sort by")
	}
	noDots := false
	if opts, ok := p.Get("options"); ok && opts.Kind() == jsonvalue.KindObject {
		for _, m := range opts.Members() {
			if m.Key != "disableDotNotation" {
				return c.unsupportedParam(n, "parameters.options."+m.Key, "the Sort option "+m.Key+" is not converted")
			}
			noDots, _ = m.Value.Bool()
		}
	}
	fields := make([]jsonvalue.Value, 0, list.Len())
	seen := map[string]bool{}
	dotted := false
	for i, it := range list.Items() {
		field := fmt.Sprintf("parameters.sortFieldsUi.sortField[%d]", i)
		name, ok := stringOf(it, "fieldName")
		if !ok || strings.TrimSpace(name) == "" || strings.HasPrefix(name, "=") {
			return c.unsupportedParam(n, field+".fieldName", "a sort field must be a literal field name, not an expression")
		}
		if seen[name] {
			return c.unsupportedParam(n, field+".fieldName", "the field "+name+" is listed twice; Orcon sorts by each field once")
		}
		seen[name] = true
		dotted = dotted || strings.Contains(name, ".")
		order := "ASCENDING"
		if o, ok := it.Get("order"); ok {
			s, _ := o.Str()
			switch s {
			case "ascending":
			case "descending":
				order = "DESCENDING"
			default:
				return c.unsupportedParam(n, field+".order", "the order must be ascending or descending and not an expression")
			}
		}
		fields = append(fields, object(member("field", jsonvalue.String(name)), member("order", jsonvalue.String(order))))
	}
	findings := []Finding{adapted(n, "parameters.sortFieldsUi",
		"Orcon orders strings by code unit, numbers by value and puts missing fields first when ascending; n8n orders strings by locale, so compare the order of text with the n8n run")}
	if noDots && dotted {
		findings = append(findings, adapted(n, "parameters.options.disableDotNotation",
			"n8n never reads a nested path here; Orcon reads one when the item has no field with that exact name"))
	}
	return itemNode(n, typeOrconSort, object(member("mode", jsonvalue.String("SIMPLE")), member("fields", jsonvalue.Array(fields...))), findings...)
}
