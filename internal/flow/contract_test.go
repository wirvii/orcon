// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
)

func keyedContract() flow.Contract {
	return flow.Contract{
		Effect: flow.EffectWrite,
		Level:  flow.SafetyIdempotencyKey,
		Idempotency: &flow.Idempotency{
			Mode:     flow.IdempotencyProviderKey,
			Location: &flow.KeyPlace{In: flow.KeyInHeader, Name: "Idempotency-Key"},
			TTL:      "PT24H",
		},
		OnUncertain: flow.OnUncertainResendWithKey,
		RetrySafe:   true,
	}
}

func reconcilableContract() flow.Contract {
	return flow.Contract{
		Effect:       flow.EffectWrite,
		Level:        flow.SafetyReconcile,
		Verification: &flow.Verification{Operation: "getOrder", OperationPurity: flow.PurityRead},
		OnUncertain:  flow.OnUncertainReconcile,
	}
}

func unsafeContract() flow.Contract {
	return flow.Contract{Effect: flow.EffectWrite, Level: flow.SafetyUnsafe, OnUncertain: flow.OnUncertainSuspend}
}

// TestCheckContract covers the validity rules of ADR-0029 section 5, which
// are a pure function of the contract.
func TestCheckContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		build  func() flow.Contract
		mutate func(c *flow.Contract)
		want   string // fragment of the one expected problem; empty means valid
	}{
		{"keyed write is valid", keyedContract, nil, ""},
		{"reconcilable write is valid", reconcilableContract, nil, ""},
		{"unsafe write is valid", unsafeContract, nil, ""},
		{"read only read is valid", func() flow.Contract {
			return flow.Contract{Effect: flow.EffectRead, Level: flow.SafetyReadOnly}
		}, nil, ""},
		{"UNKNOWN is held to the write rules", keyedContract, func(c *flow.Contract) {
			c.Effect = flow.EffectUnknown
			c.Idempotency = nil
		}, "idempotency mode"},
		{"READ_ONLY with a write", func() flow.Contract {
			return flow.Contract{Effect: flow.EffectWrite, Level: flow.SafetyReadOnly}
		}, nil, "READ_ONLY is allowed only with effect READ"},
		{"READ_ONLY with a delete", func() flow.Contract {
			return flow.Contract{Effect: flow.EffectDelete, Level: flow.SafetyReadOnly}
		}, nil, "READ_ONLY is allowed only with effect READ"},
		{"key level without a key mode", keyedContract, func(c *flow.Contract) { c.Idempotency.Mode = flow.IdempotencyNone }, "PROVIDER_KEY or NATURAL_KEY"},
		{"key level without idempotency", keyedContract, func(c *flow.Contract) { c.Idempotency = nil }, "PROVIDER_KEY or NATURAL_KEY"},
		{"key level without location", keyedContract, func(c *flow.Contract) { c.Idempotency.Location = nil }, "location"},
		{"key level with a nameless location", keyedContract, func(c *flow.Contract) { c.Idempotency.Location.Name = "" }, "location"},
		{"key level with an unknown location", keyedContract, func(c *flow.Contract) { c.Idempotency.Location.In = "COOKIE" }, "location"},
		{"key level without ttl", keyedContract, func(c *flow.Contract) { c.Idempotency.TTL = "" }, "ttl"},
		{"key level with a bad ttl", keyedContract, func(c *flow.Contract) { c.Idempotency.TTL = "a day" }, "not an ISO 8601 duration"},
		{"natural key is accepted", keyedContract, func(c *flow.Contract) { c.Idempotency.Mode = flow.IdempotencyNaturalKey }, ""},
		{"reconcile without verification", reconcilableContract, func(c *flow.Contract) { c.Verification = nil }, "verification operation"},
		{"reconcile with an operation that is not READ", reconcilableContract, func(c *flow.Contract) {
			c.Verification.OperationPurity = flow.PurityEffect
		}, "purity READ"},
		{"unsafe that is retry safe", unsafeContract, func(c *flow.Contract) { c.RetrySafe = true }, "retrySafe false"},
		{"unsafe that does not suspend", unsafeContract, func(c *flow.Contract) { c.OnUncertain = flow.OnUncertainReconcile }, "SUSPEND"},
		{"resend without the key level", unsafeContract, func(c *flow.Contract) { c.OnUncertain = flow.OnUncertainResendWithKey }, "RESEND_WITH_KEY needs level IDEMPOTENCY_KEY"},
		{"reconcile recovery without verification", keyedContract, func(c *flow.Contract) { c.OnUncertain = flow.OnUncertainReconcile }, "RECONCILE needs a verification operation"},
		{"missing recovery", keyedContract, func(c *flow.Contract) { c.OnUncertain = "" }, "onUncertain"},
		{"unknown effect", keyedContract, func(c *flow.Contract) { c.Effect = "WRITES" }, "effect"},
		{"unknown level", keyedContract, func(c *flow.Contract) { c.Level = "SAFE" }, "level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := tt.build()
			if tt.mutate != nil {
				tt.mutate(&c)
			}
			problems := flow.CheckContract(&c)
			if tt.want == "" {
				if len(problems) != 0 {
					t.Errorf("CheckContract = %v, want none", problems)
				}
				return
			}
			if !strings.Contains(strings.Join(problems, "; "), tt.want) {
				t.Errorf("CheckContract = %v, want a problem mentioning %q", problems, tt.want)
			}
		})
	}
}
