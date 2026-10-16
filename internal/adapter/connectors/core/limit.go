// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Parameter names and values of orcon.limit@1.
const (
	paramMaxItems = "maxItems"
	paramKeep     = "keep"

	// KeepFirst keeps the first items of the input.
	KeepFirst = "FIRST_ITEMS"
	// KeepLast keeps the last items of the input.
	KeepLast = "LAST_ITEMS"

	maxLimitItems = 1_000_000_000
)

// LimitParams is the parsed parameters of orcon.limit@1.
type LimitParams struct {
	// MaxItems is how many items stay, at least one.
	MaxItems int64
	// Keep is [KeepFirst] or [KeepLast].
	Keep string
}

// ParseLimit reads the parameters of orcon.limit@1. The parameter maxItems is
// required; keep defaults to FIRST_ITEMS.
func ParseLimit(params jsonvalue.Value) (LimitParams, error) {
	m, err := readMembers(params, paramMaxItems, paramKeep)
	if err != nil {
		return LimitParams{}, err
	}
	if _, ok := m[paramMaxItems]; !ok {
		return LimitParams{}, fmt.Errorf("%s is required", paramMaxItems)
	}
	n, err := m.integer(paramMaxItems, 0, 1, maxLimitItems)
	if err != nil {
		return LimitParams{}, err
	}
	keep, err := m.enum(paramKeep, KeepFirst, KeepFirst, KeepLast)
	if err != nil {
		return LimitParams{}, err
	}
	return LimitParams{MaxItems: n, Keep: keep}, nil
}

// ApplyLimit returns at most MaxItems of the items, the first or the last
// ones, in their order.
func ApplyLimit(p LimitParams, items []execution.Item) []execution.Item {
	if int64(len(items)) <= p.MaxItems {
		return items
	}
	if p.Keep == KeepLast {
		return items[int64(len(items))-p.MaxItems:]
	}
	return items[:p.MaxItems]
}

func runLimit(_ context.Context, _ *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseLimit(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	return output(ApplyLimit(p, call.Input.Items())), nil
}
