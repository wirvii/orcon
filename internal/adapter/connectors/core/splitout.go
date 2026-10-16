// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Parameter names and values of orcon.splitOut@1.
const (
	paramSplitField = "field"

	// OthersNone leaves every other field out of the new items.
	OthersNone = "NO_OTHER_FIELDS"
	// OthersAll copies every other field of the item into each new item.
	OthersAll = "ALL_OTHER_FIELDS"
	// OthersSelected copies the listed fields into each new item.
	OthersSelected = "SELECTED_OTHER_FIELDS"
)

// SplitOutParams is the parsed parameters of orcon.splitOut@1.
type SplitOutParams struct {
	// Field is the list to split: a field name or a path.
	Field string
	// Destination is the field that holds one element in each new item; the
	// name of Field when empty.
	Destination string
	// Include says which other fields of the item the new items keep.
	Include       string
	IncludeFields []string
}

// ParseSplitOut reads the parameters of orcon.splitOut@1. The field to split
// is required.
func ParseSplitOut(params jsonvalue.Value) (SplitOutParams, error) {
	m, err := readMembers(params, paramSplitField, paramDestination, paramInclude, paramIncludeNames)
	if err != nil {
		return SplitOutParams{}, err
	}
	raw, ok := m[paramSplitField]
	if !ok {
		return SplitOutParams{}, fmt.Errorf("%s is required", paramSplitField)
	}
	out := SplitOutParams{}
	if out.Field, err = fieldName(raw, paramSplitField); err != nil {
		return SplitOutParams{}, err
	}
	out.Destination = out.Field
	if v, ok := m[paramDestination]; ok {
		if out.Destination, err = fieldName(v, paramDestination); err != nil {
			return SplitOutParams{}, err
		}
	}
	if out.Include, err = m.enum(paramInclude, OthersNone, OthersNone, OthersAll, OthersSelected); err != nil {
		return SplitOutParams{}, err
	}
	if out.IncludeFields, err = m.fieldList(paramIncludeNames); err != nil {
		return SplitOutParams{}, err
	}
	switch {
	case out.Include != OthersSelected && len(out.IncludeFields) > 0:
		return SplitOutParams{}, fmt.Errorf("%s is allowed only when %s is %s", paramIncludeNames, paramInclude, OthersSelected)
	case out.Include == OthersSelected && len(out.IncludeFields) == 0:
		return SplitOutParams{}, fmt.Errorf("%s must list at least one field when %s is %s", paramIncludeNames, paramInclude, OthersSelected)
	}
	return out, nil
}

// ApplySplitOut returns one item for each element of the list that each input
// item holds in the field, in order. The element is written in the destination
// field of the new item. An item whose field is missing or is not a list fails
// the node; an empty list gives no item.
func ApplySplitOut(p SplitOutParams, items []execution.Item) ([]execution.Item, error) {
	var out []execution.Item
	for i, it := range items {
		if it.JSON.Kind() != jsonvalue.KindObject {
			return nil, fmt.Errorf("item %d is not a JSON object", i)
		}
		list := lookup(it.JSON, p.Field)
		if list.Kind() != jsonvalue.KindArray {
			return nil, fmt.Errorf("item %d: the field %q is %s, want a list", i, p.Field, describe(list))
		}
		base := baseOf(p, it.JSON)
		for _, element := range list.Items() {
			out = append(out, execution.Item{JSON: base.With(p.Destination, element), Lineage: it.Lineage})
		}
	}
	return out, nil
}

// baseOf is the item the new items start from, by what they keep of the other
// fields.
func baseOf(p SplitOutParams, item jsonvalue.Value) jsonvalue.Value {
	switch p.Include {
	case OthersAll:
		return withoutField(item, p.Field)
	case OthersSelected:
		return onlyFields(item, p.IncludeFields)
	default:
		empty, _ := jsonvalue.Object()
		return empty
	}
}

// describe names what a value is in a message: "missing" for a field that is
// not there, the kind otherwise.
func describe(v jsonvalue.Value) string {
	if v.IsZero() {
		return "missing"
	}
	return "a " + v.Kind().String()
}

func runSplitOut(_ context.Context, _ *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseSplitOut(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	out, err := ApplySplitOut(p, call.Input.Items())
	if err != nil {
		return execution.NodeOutput{}, failure("SPLIT_OUT_FAILED", err)
	}
	return output(out), nil
}
