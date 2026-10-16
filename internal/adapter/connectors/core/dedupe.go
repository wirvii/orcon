// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Parameter names and values shared by the nodes that name fields.
const (
	paramCompare     = "compare"
	paramFields      = "fields"
	paramRemoveOther = "removeOtherFields"

	// CompareAllFields treats two items as duplicates when every field is equal.
	CompareAllFields = "ALL_FIELDS"
	// CompareAllFieldsExcept ignores the listed fields when it compares.
	CompareAllFieldsExcept = "ALL_FIELDS_EXCEPT"
	// CompareSelectedFields compares only the listed fields.
	CompareSelectedFields = "SELECTED_FIELDS"
)

// DedupeParams is the parsed parameters of orcon.removeDuplicates@1.
type DedupeParams struct {
	Compare string
	// Fields are the fields to ignore or to compare, by Compare.
	Fields []string
	// RemoveOtherFields keeps, in the items that stay, only the compared
	// fields. It is allowed only when Compare is [CompareSelectedFields].
	RemoveOtherFields bool
}

// ParseRemoveDuplicates reads the parameters of orcon.removeDuplicates@1.
// Comparing all fields takes no field list; the other two modes need at least
// one field.
func ParseRemoveDuplicates(params jsonvalue.Value) (DedupeParams, error) {
	m, err := readMembers(params, paramCompare, paramFields, paramRemoveOther)
	if err != nil {
		return DedupeParams{}, err
	}
	compare, err := m.enum(paramCompare, CompareAllFields, CompareAllFields, CompareAllFieldsExcept, CompareSelectedFields)
	if err != nil {
		return DedupeParams{}, err
	}
	fields, err := m.fieldList(paramFields)
	if err != nil {
		return DedupeParams{}, err
	}
	removeOther, err := m.boolean(paramRemoveOther, false)
	if err != nil {
		return DedupeParams{}, err
	}
	switch {
	case compare == CompareAllFields && len(fields) > 0:
		return DedupeParams{}, fmt.Errorf("%s must be empty when %s is %s", paramFields, paramCompare, CompareAllFields)
	case compare != CompareAllFields && len(fields) == 0:
		return DedupeParams{}, fmt.Errorf("%s must list at least one field when %s is %s", paramFields, paramCompare, compare)
	case removeOther && compare != CompareSelectedFields:
		return DedupeParams{}, fmt.Errorf("%s is allowed only when %s is %s", paramRemoveOther, paramCompare, CompareSelectedFields)
	}
	return DedupeParams{Compare: compare, Fields: fields, RemoveOtherFields: removeOther}, nil
}

// ApplyRemoveDuplicates returns the items without the ones that repeat an
// earlier item of the same batch: the first of each group stays, in input
// order. The items must be JSON objects.
func ApplyRemoveDuplicates(p DedupeParams, items []execution.Item) ([]execution.Item, error) {
	seen := make(map[string]bool, len(items))
	out := make([]execution.Item, 0, len(items))
	for i, it := range items {
		if it.JSON.Kind() != jsonvalue.KindObject {
			return nil, fmt.Errorf("item %d is not a JSON object", i)
		}
		key := dedupeKey(p, it.JSON)
		if seen[key] {
			continue
		}
		seen[key] = true
		if p.RemoveOtherFields {
			it = execution.Item{JSON: onlyFields(it.JSON, p.Fields), Lineage: it.Lineage}
		}
		out = append(out, it)
	}
	return out, nil
}

// dedupeKey is the comparison key of one item.
func dedupeKey(p DedupeParams, item jsonvalue.Value) string {
	switch p.Compare {
	case CompareAllFieldsExcept:
		for _, name := range p.Fields {
			item = withoutField(item, name)
		}
		return valueKey(item)
	case CompareSelectedFields:
		var b strings.Builder
		for _, name := range p.Fields {
			k := valueKey(lookup(item, name))
			b.WriteString(strconv.Itoa(len(k)) + ":" + k)
		}
		return b.String()
	default:
		return valueKey(item)
	}
}

func runRemoveDuplicates(_ context.Context, _ *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseRemoveDuplicates(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	kept, err := ApplyRemoveDuplicates(p, call.Input.Items())
	if err != nil {
		return execution.NodeOutput{}, failure("REMOVE_DUPLICATES_FAILED", err)
	}
	return output(kept), nil
}
