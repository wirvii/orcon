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

// Parameter names of orcon.filter@1.
const (
	paramCondition    = "condition"
	maxConditionBytes = 16 << 10
)

// FilterParams is the parsed parameters of orcon.filter@1.
type FilterParams struct {
	// Condition is the test every item must pass to stay. It has the form of
	// the condition of orcon.if@1: text that is the code of one expression
	// segment, or a template that starts with "=" (ADR-0018 section 1). It
	// must evaluate to a boolean.
	Condition string
}

// ParseFilter reads the parameters of orcon.filter@1. The condition is
// required; any other key is an error.
func ParseFilter(params jsonvalue.Value) (FilterParams, error) {
	m, err := readMembers(params, paramCondition)
	if err != nil {
		return FilterParams{}, err
	}
	v, ok := m[paramCondition]
	if !ok {
		return FilterParams{}, fmt.Errorf("%s is required", paramCondition)
	}
	s, isString := v.Str()
	if !isString || strings.TrimSpace(s) == "" {
		return FilterParams{}, fmt.Errorf("%s must be a non-empty string", paramCondition)
	}
	if len(s) > maxConditionBytes {
		return FilterParams{}, fmt.Errorf("%s is longer than %d bytes", paramCondition, maxConditionBytes)
	}
	return FilterParams{Condition: s}, nil
}

// errNotBoolean means a condition evaluated to something other than true or
// false.
var errNotBoolean = errors.New("the condition did not evaluate to a boolean")

// ApplyFilter returns the items the condition accepts, in their order. Each
// item is tested alone: the expression sees it as $json. The kept items are
// the input items themselves, lineage included.
func ApplyFilter(ctx context.Context, p FilterParams, ev Evaluator, items []execution.Item) ([]execution.Item, error) {
	if ev == nil {
		return nil, errors.New("a condition needs an evaluator, none is configured")
	}
	template := templateOf(p.Condition)
	kept := make([]execution.Item, 0, len(items))
	for i, it := range items {
		v, err := ev.Evaluate(ctx, template, it.JSON)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		pass, ok := v.Bool()
		if !ok {
			return nil, fmt.Errorf("item %d: %w", i, errNotBoolean)
		}
		if pass {
			kept = append(kept, it)
		}
	}
	return kept, nil
}

func runFilter(ctx context.Context, r *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseFilter(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	kept, err := ApplyFilter(ctx, p, r.evaluator(call), call.Input.Items())
	if err != nil {
		return execution.NodeOutput{}, failure("FILTER_FAILED", err)
	}
	return output(kept), nil
}
