// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// TestModelPricesFollowTheLayers: the agent's price source answers from the
// price layers (ADR-0062 section 3). The embedded table ships no confirmed
// prices, so a model it does not list has an unknown cost, and a layer that
// lists it is used with its version.
func TestModelPricesFollowTheLayers(t *testing.T) {
	ref := flow.ModelRef{Provider: "acme", Name: "model-x"}
	if _, _, ok := newModelPrices().Price(t.Context(), identity.OrgID{}, ref); ok {
		t.Fatal("the embedded table priced a model it does not list")
	}
	layered := modelPrices{layers: cost.Layers{
		InstanceOverride: map[cost.ModelKey]cost.Price{{Provider: "acme", Model: "model-x"}: {Input: 7, Output: 9}},
		Embedded:         cost.Embedded(),
	}}
	p, version, ok := layered.Price(t.Context(), identity.OrgID{}, ref)
	if !ok || p.Input != 7 || p.Output != 9 || version != "instance-override" {
		t.Fatalf("Price() = %+v, %q, %v; want the override", p, version, ok)
	}
}
