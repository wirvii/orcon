// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Orcon types of the list nodes of SPEC-003 Annex B (CN13).
const (
	typeOrconRemoveDuplicates = "orcon.removeDuplicates@1"
	typeOrconAggregate        = "orcon.aggregate@1"
	typeOrconSplitOut         = "orcon.splitOut@1"
)

// fieldNames reads a list of field names the way n8n stores it: one string of
// names separated by commas, a fixed collection {fields: [{fieldName}]}, or a
// list of strings. It reports false for an expression and for any other shape,
// which the converter cannot carry. A missing value is an empty list.
func fieldNames(v jsonvalue.Value, present bool) ([]string, bool) {
	if !present {
		return nil, true
	}
	var raw []string
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		raw = strings.Split(s, ",")
	case jsonvalue.KindObject:
		list, _ := v.Get("fields")
		if list.Kind() != jsonvalue.KindArray {
			return nil, false
		}
		for _, it := range list.Items() {
			s, ok := stringOf(it, "fieldName")
			if !ok {
				return nil, false
			}
			raw = append(raw, s)
		}
	case jsonvalue.KindArray:
		for _, it := range v.Items() {
			s, ok := it.Str()
			if !ok {
				return nil, false
			}
			raw = append(raw, s)
		}
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber:
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "=") {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// stringValues builds a list of JSON strings.
func stringValues(names []string) jsonvalue.Value {
	items := make([]jsonvalue.Value, len(names))
	for i, s := range names {
		items[i] = jsonvalue.String(s)
	}
	return jsonvalue.Array(items...)
}

// anyDotted reports whether a name is a path through nested fields.
func anyDotted(names ...string) bool {
	for _, n := range names {
		if strings.Contains(n, ".") {
			return true
		}
	}
	return false
}

// listOptions reads the options collection of a list node. Only the options
// the node converter names are allowed; any other one is reported.
func listOptions(p jsonvalue.Value, known ...string) (opts map[string]jsonvalue.Value, unknown string) {
	opts = map[string]jsonvalue.Value{}
	o, ok := p.Get("options")
	if !ok || o.Kind() != jsonvalue.KindObject {
		return opts, ""
	}
	for _, m := range o.Members() {
		found := false
		for _, k := range known {
			found = found || k == m.Key
		}
		if !found {
			return nil, m.Key
		}
		opts[m.Key] = m.Value
	}
	return opts, ""
}

// flag reads an option that is true or false; a missing option is false. It
// reports false for anything else, an expression included.
func flag(opts map[string]jsonvalue.Value, key string) (value, ok bool) {
	v, present := opts[key]
	if !present {
		return false, true
	}
	return v.Bool()
}

// dotNotationFinding states what changes when n8n was told never to read a
// nested path and a name has a dot.
func dotNotationFinding(n *Node, names ...string) []Finding {
	return []Finding{adapted(n, "parameters.options.disableDotNotation",
		"n8n never reads a nested path here, so "+strings.Join(names, ", ")+" is one field name there; Orcon reads a path when the item has no field with that exact name")}
}

// convertRemoveDuplicates maps Remove Duplicates when it removes the items
// that repeat inside the current input. Remembering items across executions
// needs the flow data of ADR-0008 section 4 and is not converted.
func (c *converter) convertRemoveDuplicates(n *Node) convResult {
	p := params(n)
	if op, ok := p.Get("operation"); ok {
		if s, _ := op.Str(); s != "removeDuplicateInputItems" {
			return c.unsupportedParam(n, "parameters.operation",
				"only the removal of repeats inside the current input is converted; remembering items across executions needs flow data, which no node reads yet")
		}
	}
	opts, unknown := listOptions(p, "disableDotNotation", "removeOtherFields")
	if unknown != "" {
		return c.unsupportedParam(n, "parameters.options."+unknown, "the Remove Duplicates option "+unknown+" is not converted")
	}
	noDots, ok1 := flag(opts, "disableDotNotation")
	removeOther, ok2 := flag(opts, "removeOtherFields")
	if !ok1 || !ok2 {
		return c.unsupportedParam(n, "parameters.options", "the options must be true or false and not expressions")
	}
	compare, _ := stringParam(p, "compare")
	if v, ok := p.Get("compare"); ok && compare == "" {
		if s, _ := v.Str(); s != "" {
			return c.unsupportedParam(n, "parameters.compare", "the comparison must be chosen, not computed by an expression")
		}
	}
	var mode, key string
	switch compare {
	case "", "allFields":
		mode = "ALL_FIELDS"
	case "allFieldsExcept":
		mode, key = "ALL_FIELDS_EXCEPT", "fieldsToExclude"
	case "selectedFields":
		mode, key = "SELECTED_FIELDS", "fieldsToCompare"
	default:
		return c.unsupportedParam(n, "parameters.compare", "the comparison "+compare+" is not converted")
	}
	var names []string
	if key != "" {
		v, present := p.Get(key)
		var ok bool
		if names, ok = fieldNames(v, present); !ok || len(names) == 0 {
			return c.unsupportedParam(n, "parameters."+key, "the fields must be literal names, and at least one")
		}
	}
	if removeOther && mode != "SELECTED_FIELDS" {
		removeOther = false // n8n ignores it unless it compares selected fields
	}
	members := []jsonvalue.Member{member("compare", jsonvalue.String(mode))}
	if len(names) > 0 {
		members = append(members, member("fields", stringValues(names)))
	}
	if removeOther {
		members = append(members, member("removeOtherFields", jsonvalue.Bool(true)))
	}
	var findings []Finding
	if noDots && anyDotted(names...) {
		findings = dotNotationFinding(n, names...)
	}
	return itemNode(n, typeOrconRemoveDuplicates, object(members...), findings...)
}

// convertAggregate maps Aggregate in both of its modes.
func (c *converter) convertAggregate(n *Node) convResult {
	p := params(n)
	opts, unknown := listOptions(p, "disableDotNotation", "mergeLists", "keepMissing")
	if unknown != "" {
		return c.unsupportedParam(n, "parameters.options."+unknown, "the Aggregate option "+unknown+" is not converted")
	}
	noDots, ok1 := flag(opts, "disableDotNotation")
	merge, ok2 := flag(opts, "mergeLists")
	keep, ok3 := flag(opts, "keepMissing")
	if !ok1 || !ok2 || !ok3 {
		return c.unsupportedParam(n, "parameters.options", "the options must be true or false and not expressions")
	}
	mode, _ := stringParam(p, "aggregate")
	switch mode {
	case "", "aggregateIndividualFields":
		return c.convertAggregateFields(n, p, noDots, merge, keep)
	case "aggregateAllItemData":
		return c.convertAggregateItems(n, p, noDots)
	default:
		return c.unsupportedParam(n, "parameters.aggregate", "the aggregation "+mode+" is not converted")
	}
}

func (c *converter) convertAggregateFields(n *Node, p jsonvalue.Value, noDots, merge, keep bool) convResult {
	coll, _ := p.Get("fieldsToAggregate")
	list, _ := coll.Get("fieldToAggregate")
	if list.Kind() != jsonvalue.KindArray || list.Len() == 0 {
		return c.unsupportedParam(n, "parameters.fieldsToAggregate.fieldToAggregate", "the Aggregate node has no field to gather")
	}
	fields := make([]jsonvalue.Value, 0, list.Len())
	var names []string
	for i, it := range list.Items() {
		field := fmt.Sprintf("parameters.fieldsToAggregate.fieldToAggregate[%d]", i)
		name, ok := stringOf(it, "fieldToAggregate")
		if !ok || strings.TrimSpace(name) == "" || strings.HasPrefix(name, "=") {
			return c.unsupportedParam(n, field+".fieldToAggregate", "a field to gather must be a literal field name, not an expression")
		}
		names = append(names, name)
		members := []jsonvalue.Member{member("field", jsonvalue.String(name))}
		if r, ok := it.Get("renameField"); ok {
			if rename, isBool := r.Bool(); !isBool {
				return c.unsupportedParam(n, field+".renameField", "renameField must be true or false")
			} else if rename {
				out, ok := stringOf(it, "outputFieldName")
				if !ok || strings.TrimSpace(out) == "" || strings.HasPrefix(out, "=") {
					return c.unsupportedParam(n, field+".outputFieldName", "the output name must be a literal name")
				}
				members = append(members, member("outputField", jsonvalue.String(out)))
			}
		}
		fields = append(fields, object(members...))
	}
	parameters := []jsonvalue.Member{member("mode", jsonvalue.String("INDIVIDUAL_FIELDS")), member("fields", jsonvalue.Array(fields...))}
	if merge {
		parameters = append(parameters, member("mergeLists", jsonvalue.Bool(true)))
	}
	if keep {
		parameters = append(parameters, member("keepMissing", jsonvalue.Bool(true)))
	}
	var findings []Finding
	if noDots && anyDotted(names...) {
		findings = dotNotationFinding(n, names...)
	}
	return itemNode(n, typeOrconAggregate, object(parameters...), findings...)
}

func (c *converter) convertAggregateItems(n *Node, p jsonvalue.Value, noDots bool) convResult {
	destination := "data"
	if v, ok := p.Get("destinationFieldName"); ok {
		s, isString := v.Str()
		if !isString || strings.TrimSpace(s) == "" || strings.HasPrefix(s, "=") {
			return c.unsupportedParam(n, "parameters.destinationFieldName", "the destination must be a literal field name")
		}
		destination = s
	}
	include, _ := stringParam(p, "include")
	var mode, key string
	switch include {
	case "", "allFields":
		mode = "ALL_FIELDS"
	case "specifiedFields":
		mode, key = "SELECTED_FIELDS", "fieldsToInclude"
	case "allFieldsExcept":
		mode, key = "ALL_FIELDS_EXCEPT", "fieldsToExclude"
	default:
		return c.unsupportedParam(n, "parameters.include", "the choice "+include+" of fields is not converted")
	}
	parameters := []jsonvalue.Member{
		member("mode", jsonvalue.String("ALL_ITEM_DATA")),
		member("destinationField", jsonvalue.String(destination)),
		member("include", jsonvalue.String(mode)),
	}
	var names []string
	if key != "" {
		v, present := p.Get(key)
		var ok bool
		if names, ok = fieldNames(v, present); !ok || len(names) == 0 {
			return c.unsupportedParam(n, "parameters."+key, "the fields must be literal names, and at least one")
		}
		parameters = append(parameters, member("includeFields", stringValues(names)))
	}
	var findings []Finding
	if noDots && anyDotted(names...) {
		findings = dotNotationFinding(n, names...)
	}
	return itemNode(n, typeOrconAggregate, object(parameters...), findings...)
}

// convertSplitOut maps Split Out for one field. Several fields at once, in a
// comma separated list, are not converted.
func (c *converter) convertSplitOut(n *Node) convResult {
	p := params(n)
	fv, present := p.Get("fieldToSplitOut")
	names, ok := fieldNames(fv, present)
	if !ok || len(names) != 1 {
		return c.unsupportedParam(n, "parameters.fieldToSplitOut", "the node must split out one literal field; several fields at once are not converted")
	}
	field := names[0]
	opts, unknown := listOptions(p, "disableDotNotation", "destinationFieldName")
	if unknown != "" {
		return c.unsupportedParam(n, "parameters.options."+unknown, "the Split Out option "+unknown+" is not converted")
	}
	noDots, ok := flag(opts, "disableDotNotation")
	if !ok {
		return c.unsupportedParam(n, "parameters.options.disableDotNotation", "the option must be true or false and not an expression")
	}
	destination := ""
	if v, present := opts["destinationFieldName"]; present {
		dn, isNames := fieldNames(v, true)
		if !isNames || len(dn) > 1 {
			return c.unsupportedParam(n, "parameters.options.destinationFieldName", "the destination must be one literal field name")
		}
		if len(dn) == 1 {
			destination = dn[0]
		}
	}
	include, _ := stringParam(p, "include")
	var mode string
	var includeNames []string
	switch include {
	case "", "noOtherFields":
		mode = "NO_OTHER_FIELDS"
	case "allOtherFields":
		mode = "ALL_OTHER_FIELDS"
	case "selectedOtherFields":
		mode = "SELECTED_OTHER_FIELDS"
		v, present := p.Get("fieldsToInclude")
		if includeNames, ok = fieldNames(v, present); !ok || len(includeNames) == 0 {
			return c.unsupportedParam(n, "parameters.fieldsToInclude", "the fields to keep must be literal names, and at least one")
		}
	default:
		return c.unsupportedParam(n, "parameters.include", "the choice "+include+" of other fields is not converted")
	}
	parameters := []jsonvalue.Member{member("field", jsonvalue.String(field))}
	if destination != "" {
		parameters = append(parameters, member("destinationField", jsonvalue.String(destination)))
	}
	if mode != "NO_OTHER_FIELDS" {
		parameters = append(parameters, member("include", jsonvalue.String(mode)))
	}
	if len(includeNames) > 0 {
		parameters = append(parameters, member("includeFields", stringValues(includeNames)))
	}
	var findings []Finding
	if destination == "" {
		findings = append(findings, adapted(n, "parameters.fieldToSplitOut",
			"Orcon writes each element in a field of the new item, named like the list; compare the new items with the n8n run when the elements are objects, and expect an error for an item whose field is missing or is not a list"))
	}
	if noDots && anyDotted(append([]string{field}, includeNames...)...) {
		findings = append(findings, dotNotationFinding(n, field)...)
	}
	return itemNode(n, typeOrconSplitOut, object(parameters...), findings...)
}
