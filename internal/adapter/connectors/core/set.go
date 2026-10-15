// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Parameter names of orcon.set@1. The set is closed: an unknown key is an
// error, so a misspelled parameter never silently does nothing.
const (
	paramAssignments       = "assignments"
	paramIncludeOther      = "includeOtherFields"
	paramAssignName        = "name"
	paramAssignValue       = "value"
	expressionPrefix       = "="
	maxAssignmentNameBytes = 255
)

// Assignment is one field the Set node writes: its name and the value it gets.
// A value that is a top-level string starting with "=" is an expression and is
// evaluated per item by the [Evaluator]; any other value is literal.
type Assignment struct {
	Name  string
	Value jsonvalue.Value
}

// SetParams is the parsed parameters of orcon.set@1.
type SetParams struct {
	// Assignments are applied in order. Names are unique.
	Assignments []Assignment
	// IncludeOtherFields keeps every field of the input item; when false the
	// output item holds only the assigned fields.
	IncludeOtherFields bool
}

// Evaluator evaluates an expression of a parameter value against one item
// (ADR-0018). The expression language belongs to another package; this port
// lets the Set node run without knowing it.
type Evaluator interface {
	Evaluate(ctx context.Context, expression string, item jsonvalue.Value) (jsonvalue.Value, error)
}

// ParseSet reads the parameters of orcon.set@1 and reports the first problem.
// An absent or null parameter object means no assignments.
func ParseSet(params jsonvalue.Value) (SetParams, error) {
	var out SetParams
	if params.IsZero() || params.Kind() == jsonvalue.KindNull {
		return out, nil
	}
	if params.Kind() != jsonvalue.KindObject {
		return out, errors.New("parameters must be an object")
	}
	seen := map[string]bool{}
	for _, m := range params.Members() {
		switch m.Key {
		case paramAssignments:
			list, err := parseAssignments(m.Value, seen)
			if err != nil {
				return SetParams{}, err
			}
			out.Assignments = list
		case paramIncludeOther:
			b, ok := m.Value.Bool()
			if !ok {
				return SetParams{}, fmt.Errorf("%s must be a boolean", paramIncludeOther)
			}
			out.IncludeOtherFields = b
		default:
			return SetParams{}, fmt.Errorf("unknown parameter %q", m.Key)
		}
	}
	return out, nil
}

func parseAssignments(v jsonvalue.Value, seen map[string]bool) ([]Assignment, error) {
	if v.Kind() != jsonvalue.KindArray {
		return nil, fmt.Errorf("%s must be an array", paramAssignments)
	}
	items := v.Items()
	out := make([]Assignment, 0, len(items))
	for i, a := range items {
		if a.Kind() != jsonvalue.KindObject {
			return nil, fmt.Errorf("%s[%d] must be an object", paramAssignments, i)
		}
		var as Assignment
		var hasName, hasValue bool
		for _, m := range a.Members() {
			switch m.Key {
			case paramAssignName:
				name, ok := m.Value.Str()
				if !ok || strings.TrimSpace(name) == "" {
					return nil, fmt.Errorf("%s[%d].name must be a non-empty string", paramAssignments, i)
				}
				if len(name) > maxAssignmentNameBytes {
					return nil, fmt.Errorf("%s[%d].name is longer than %d bytes", paramAssignments, i, maxAssignmentNameBytes)
				}
				as.Name, hasName = name, true
			case paramAssignValue:
				as.Value, hasValue = m.Value, true
			default:
				return nil, fmt.Errorf("%s[%d] has unknown key %q", paramAssignments, i, m.Key)
			}
		}
		if !hasName {
			return nil, fmt.Errorf("%s[%d] has no name", paramAssignments, i)
		}
		if !hasValue {
			return nil, fmt.Errorf("%s[%d] has no value", paramAssignments, i)
		}
		if seen[as.Name] {
			return nil, fmt.Errorf("field %q is assigned more than once", as.Name)
		}
		seen[as.Name] = true
		out = append(out, as)
	}
	return out, nil
}

// ApplySet returns the output item of one input item: the assignments applied
// in order to a copy of the input (when IncludeOtherFields is set) or to an
// empty object. The input must be a JSON object.
func ApplySet(ctx context.Context, p SetParams, ev Evaluator, item jsonvalue.Value) (jsonvalue.Value, error) {
	if item.Kind() != jsonvalue.KindObject {
		return jsonvalue.Value{}, errors.New("the input item is not a JSON object")
	}
	out, err := jsonvalue.Object()
	if err != nil {
		return jsonvalue.Value{}, err
	}
	if p.IncludeOtherFields {
		out = item
	}
	for _, as := range p.Assignments {
		val, err := valueOf(ctx, as.Value, ev, item)
		if err != nil {
			return jsonvalue.Value{}, fmt.Errorf("field %q: %w", as.Name, err)
		}
		out = out.With(as.Name, val)
	}
	return out, nil
}

func valueOf(ctx context.Context, v jsonvalue.Value, ev Evaluator, item jsonvalue.Value) (jsonvalue.Value, error) {
	s, ok := v.Str()
	if !ok || !strings.HasPrefix(s, expressionPrefix) {
		return v, nil
	}
	if ev == nil {
		return jsonvalue.Value{}, errors.New("an expression needs an evaluator, none is configured")
	}
	return ev.Evaluate(ctx, strings.TrimPrefix(s, expressionPrefix), item)
}
