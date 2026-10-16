// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/execution"
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
// Every string inside the value that starts with "=" is an expression template
// (ADR-0018 section 1) and is evaluated per item by the [Evaluator]; every
// other string, number, boolean or null is literal.
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

// Evaluator evaluates one parameter string that starts with "=" against one
// item (ADR-0018). The string is passed whole, prefix included, because the
// template syntax is defined on the whole string. The language belongs to
// another package; this port lets the Set node run without knowing it.
type Evaluator interface {
	Evaluate(ctx context.Context, template string, item jsonvalue.Value) (jsonvalue.Value, error)
}

// ActivationEvaluator is an [Evaluator] that depends on the recorded
// nondeterminism of one activation (ADR-0013 section 7, ADR-0018 section 5):
// the runner binds it to the environment of the activation before it
// evaluates, so `$now` and `Math.random()` give the recorded values again when
// the activation is recomputed.
type ActivationEvaluator interface {
	Evaluator
	// Bind returns the evaluator of one activation. It is safe to call per
	// activation and its result is used by that activation only.
	Bind(env execution.NodeEnv) Evaluator
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

// valueOf returns the value an assignment writes for one item: expressions
// are evaluated wherever they appear inside the value, literals are kept.
func valueOf(ctx context.Context, v jsonvalue.Value, ev Evaluator, item jsonvalue.Value) (jsonvalue.Value, error) {
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		if !strings.HasPrefix(s, expressionPrefix) {
			return v, nil
		}
		if ev == nil {
			return jsonvalue.Value{}, errors.New("an expression needs an evaluator, none is configured")
		}
		return ev.Evaluate(ctx, s, item)
	case jsonvalue.KindArray:
		elems := v.Items()
		out := make([]jsonvalue.Value, 0, len(elems))
		for _, e := range elems {
			ve, err := valueOf(ctx, e, ev, item)
			if err != nil {
				return jsonvalue.Value{}, err
			}
			out = append(out, ve)
		}
		return jsonvalue.Array(out...), nil
	case jsonvalue.KindObject:
		members := v.Members()
		out := make([]jsonvalue.Member, 0, len(members))
		for _, m := range members {
			vm, err := valueOf(ctx, m.Value, ev, item)
			if err != nil {
				return jsonvalue.Value{}, err
			}
			out = append(out, jsonvalue.Member{Key: m.Key, Value: vm})
		}
		return jsonvalue.Object(out...)
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber:
		return v, nil
	}
	return v, nil
}
