// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Parameter names and values of orcon.aggregate@1.
const (
	paramAggMode      = "mode"
	paramMergeLists   = "mergeLists"
	paramKeepMissing  = "keepMissing"
	paramDestination  = "destinationField"
	paramInclude      = "include"
	paramIncludeNames = "includeFields"
	paramAggField     = "field"
	paramAggOutput    = "outputField"

	// AggregateIndividualFields gathers the values of some fields into lists.
	AggregateIndividualFields = "INDIVIDUAL_FIELDS"
	// AggregateAllItemData gathers whole items into one list.
	AggregateAllItemData = "ALL_ITEM_DATA"

	// IncludeAllFields keeps every field of each gathered item.
	IncludeAllFields = "ALL_FIELDS"
	// IncludeSelectedFields keeps only the listed fields.
	IncludeSelectedFields = "SELECTED_FIELDS"
	// IncludeAllFieldsExcept keeps every field but the listed ones.
	IncludeAllFieldsExcept = "ALL_FIELDS_EXCEPT"

	defaultDestination = "data"
)

// AggregateField is one field whose values are gathered into a list.
type AggregateField struct {
	// Field is the field name or path to read.
	Field string
	// Output is the name of the list in the output item; the field name when
	// empty.
	Output string
}

// name is the key of the list in the output item.
func (f AggregateField) name() string {
	if f.Output != "" {
		return f.Output
	}
	return f.Field
}

// AggregateParams is the parsed parameters of orcon.aggregate@1.
type AggregateParams struct {
	Mode string
	// Fields are the gathered fields, in output order (INDIVIDUAL_FIELDS).
	Fields []AggregateField
	// MergeLists adds the elements of a value that is a list, instead of the
	// list itself (INDIVIDUAL_FIELDS).
	MergeLists bool
	// KeepMissing adds null for an item that lacks the field; otherwise the
	// item adds nothing (INDIVIDUAL_FIELDS).
	KeepMissing bool
	// Destination is the field that holds the list of items (ALL_ITEM_DATA).
	Destination string
	// Include says which fields of each item go into the list (ALL_ITEM_DATA).
	Include       string
	IncludeFields []string
}

// ParseAggregate reads the parameters of orcon.aggregate@1. Each mode has its
// own parameters; one that belongs to the other mode is an error.
func ParseAggregate(params jsonvalue.Value) (AggregateParams, error) {
	m, err := readMembers(params, paramAggMode, paramFields, paramMergeLists, paramKeepMissing,
		paramDestination, paramInclude, paramIncludeNames)
	if err != nil {
		return AggregateParams{}, err
	}
	mode, err := m.enum(paramAggMode, AggregateIndividualFields, AggregateIndividualFields, AggregateAllItemData)
	if err != nil {
		return AggregateParams{}, err
	}
	out := AggregateParams{Mode: mode}
	if mode == AggregateIndividualFields {
		for _, key := range []string{paramDestination, paramInclude, paramIncludeNames} {
			if _, ok := m[key]; ok {
				return AggregateParams{}, fmt.Errorf("%s belongs to the %s mode", key, AggregateAllItemData)
			}
		}
		if out.Fields, err = parseAggregateFields(m); err != nil {
			return AggregateParams{}, err
		}
		if out.MergeLists, err = m.boolean(paramMergeLists, false); err != nil {
			return AggregateParams{}, err
		}
		if out.KeepMissing, err = m.boolean(paramKeepMissing, false); err != nil {
			return AggregateParams{}, err
		}
		return out, nil
	}
	for _, key := range []string{paramFields, paramMergeLists, paramKeepMissing} {
		if _, ok := m[key]; ok {
			return AggregateParams{}, fmt.Errorf("%s belongs to the %s mode", key, AggregateIndividualFields)
		}
	}
	out.Destination = defaultDestination
	if v, ok := m[paramDestination]; ok {
		if out.Destination, err = fieldName(v, paramDestination); err != nil {
			return AggregateParams{}, err
		}
	}
	if out.Include, err = m.enum(paramInclude, IncludeAllFields, IncludeAllFields, IncludeSelectedFields, IncludeAllFieldsExcept); err != nil {
		return AggregateParams{}, err
	}
	if out.IncludeFields, err = m.fieldList(paramIncludeNames); err != nil {
		return AggregateParams{}, err
	}
	switch {
	case out.Include == IncludeAllFields && len(out.IncludeFields) > 0:
		return AggregateParams{}, fmt.Errorf("%s must be empty when %s is %s", paramIncludeNames, paramInclude, IncludeAllFields)
	case out.Include != IncludeAllFields && len(out.IncludeFields) == 0:
		return AggregateParams{}, fmt.Errorf("%s must list at least one field when %s is %s", paramIncludeNames, paramInclude, out.Include)
	}
	return out, nil
}

func parseAggregateFields(m members) ([]AggregateField, error) {
	raw, ok := m[paramFields]
	if !ok || raw.Kind() != jsonvalue.KindArray || raw.Len() == 0 {
		return nil, fmt.Errorf("%s must list at least one field in the %s mode", paramFields, AggregateIndividualFields)
	}
	if raw.Len() > maxListedFields {
		return nil, fmt.Errorf("%s has more than %d fields", paramFields, maxListedFields)
	}
	var out []AggregateField
	names := map[string]bool{}
	for i, v := range raw.Items() {
		if v.Kind() != jsonvalue.KindObject {
			return nil, fmt.Errorf("%s[%d] must be an object", paramFields, i)
		}
		var f AggregateField
		var hasField bool
		for _, member := range v.Members() {
			switch member.Key {
			case paramAggField:
				name, err := fieldName(member.Value, fmt.Sprintf("%s[%d].%s", paramFields, i, paramAggField))
				if err != nil {
					return nil, err
				}
				f.Field, hasField = name, true
			case paramAggOutput:
				name, err := fieldName(member.Value, fmt.Sprintf("%s[%d].%s", paramFields, i, paramAggOutput))
				if err != nil {
					return nil, err
				}
				f.Output = name
			default:
				return nil, fmt.Errorf("%s[%d] has unknown key %q", paramFields, i, member.Key)
			}
		}
		if !hasField {
			return nil, fmt.Errorf("%s[%d] has no %s", paramFields, i, paramAggField)
		}
		if names[f.name()] {
			return nil, fmt.Errorf("%s gives the output name %q more than once", paramFields, f.name())
		}
		names[f.name()] = true
		out = append(out, f)
	}
	return out, nil
}

// ApplyAggregate returns the one item that gathers all the items, or none when
// there are no items to gather. The item comes from every input item, so it
// carries all their lineage.
func ApplyAggregate(p AggregateParams, items []execution.Item) ([]execution.Item, error) {
	if len(items) == 0 {
		return nil, nil
	}
	var body jsonvalue.Value
	var err error
	if p.Mode == AggregateAllItemData {
		body, err = gatherItems(p, items)
	} else {
		body, err = gatherFields(p, items)
	}
	if err != nil {
		return nil, err
	}
	return []execution.Item{{JSON: body, Lineage: mergedLineage(items)}}, nil
}

func gatherFields(p AggregateParams, items []execution.Item) (jsonvalue.Value, error) {
	out, err := jsonvalue.Object()
	if err != nil {
		return jsonvalue.Value{}, err
	}
	for _, f := range p.Fields {
		values := make([]jsonvalue.Value, 0, len(items))
		for _, it := range items {
			v := lookup(it.JSON, f.Field)
			switch {
			case v.IsZero():
				if p.KeepMissing {
					values = append(values, jsonvalue.Null())
				}
			case p.MergeLists && v.Kind() == jsonvalue.KindArray:
				values = append(values, v.Items()...)
			default:
				values = append(values, v)
			}
		}
		out = out.With(f.name(), jsonvalue.Array(values...))
	}
	return out, nil
}

func gatherItems(p AggregateParams, items []execution.Item) (jsonvalue.Value, error) {
	values := make([]jsonvalue.Value, 0, len(items))
	for i, it := range items {
		if it.JSON.Kind() != jsonvalue.KindObject {
			return jsonvalue.Value{}, fmt.Errorf("item %d is not a JSON object", i)
		}
		v := it.JSON
		switch p.Include {
		case IncludeSelectedFields:
			v = onlyFields(v, p.IncludeFields)
		case IncludeAllFieldsExcept:
			for _, name := range p.IncludeFields {
				v = withoutField(v, name)
			}
		}
		values = append(values, v)
	}
	return jsonvalue.Object(jsonvalue.Member{Key: p.Destination, Value: jsonvalue.Array(values...)})
}

func runAggregate(_ context.Context, _ *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseAggregate(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	out, err := ApplyAggregate(p, call.Input.Items())
	if err != nil {
		return execution.NodeOutput{}, failure("AGGREGATE_FAILED", err)
	}
	return output(out), nil
}
