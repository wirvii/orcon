// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
)

// fakeCatalog is a hand-written node-type catalog. It records what the
// validator asks for, so tests can check that format types are not looked up.
type fakeCatalog struct {
	types map[string]flow.NodeType

	mu     sync.Mutex
	asked  []string
	sawCtx bool
}

func (c *fakeCatalog) Lookup(ctx context.Context, nodeType string) (flow.NodeType, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asked = append(c.asked, nodeType)
	c.sawCtx = c.sawCtx || ctx != nil
	nt, ok := c.types[nodeType]
	return nt, ok
}

const (
	chargeType = "orcon.payments.charge@2"
	refundType = "orcon.payments.refund@2"
)

// catalogFor knows every non-format node type of doc as a pure type, except
// the payment types, which are effect types with real contracts.
func catalogFor(doc *flow.Document) *fakeCatalog {
	types := map[string]flow.NodeType{}
	for i := range doc.Nodes {
		types[doc.Nodes[i].Type] = flow.NodeType{Purity: flow.PurityPure}
	}
	keyed := keyedContract()
	unsafe := unsafeContract()
	types[chargeType] = flow.NodeType{Purity: flow.PurityEffect, Contract: &keyed}
	types[refundType] = flow.NodeType{Purity: flow.PurityEffect, Contract: &unsafe}
	return &fakeCatalog{types: types}
}

func TestValidateWithTypesAcceptsConformanceFlows(t *testing.T) {
	t.Parallel()
	for _, name := range conformanceNames(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, name)
			cat := catalogFor(&doc)
			r := flow.ValidateWithTypes(t.Context(), &doc, cat)
			if len(r.Issues) > 0 {
				t.Errorf("ValidateWithTypes(%s) issues = %v, want none", name, r.Issues)
			}
			if !cat.sawCtx && len(cat.asked) > 0 {
				t.Error("the catalog was asked without a context")
			}
		})
	}
}

func TestValidateWithTypesDoesNotAskAboutFormatTypes(t *testing.T) {
	t.Parallel()
	doc := load(t, "loop")
	cat := catalogFor(&doc)
	for _, format := range []string{"orcon.loop@1", "orcon.if@1", "orcon.switch@1"} {
		delete(cat.types, format)
	}
	if r := flow.ValidateWithTypes(t.Context(), &doc, cat); r.HasErrors() {
		t.Fatalf("errors = %v", r.Errors())
	}
	for _, asked := range cat.asked {
		if strings.HasPrefix(asked, "orcon.loop") || strings.HasPrefix(asked, "orcon.if") {
			t.Errorf("the catalog was asked about the format type %s", asked)
		}
	}
}

type catalogCase struct {
	name   string
	base   string
	mutate func(d *flow.Document, c *fakeCatalog)
	code   string
	sev    flow.Severity
	msg    string // fragment of the message
}

func runCatalogCases(t *testing.T, cases []catalogCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, tt.base)
			cat := catalogFor(&doc)
			tt.mutate(&doc, cat)
			r := flow.ValidateWithTypes(t.Context(), &doc, cat)
			for _, issue := range r.Issues {
				if issue.Code == tt.code && issue.Severity == tt.sev && strings.Contains(issue.Message, tt.msg) {
					return
				}
			}
			t.Errorf("issues = %v, want %s %s mentioning %q", r.Issues, tt.sev, tt.code, tt.msg)
		})
	}
}

func TestValidateV10UsesTheNodeTypeCatalog(t *testing.T) {
	t.Parallel()
	const base = "effects-calls-and-settings"
	runCatalogCases(t, []catalogCase{
		{"unknown node type", base, func(_ *flow.Document, c *fakeCatalog) { delete(c.types, chargeType) },
			flow.CodeV10, flow.SeverityError, chargeType},
		{"effect type without a contract", base, func(_ *flow.Document, c *fakeCatalog) {
			c.types[chargeType] = flow.NodeType{Purity: flow.PurityEffect}
		}, flow.CodeV10, flow.SeverityError, "without a safety contract"},
		{"effect type with an invalid contract", base, func(_ *flow.Document, c *fakeCatalog) {
			bad := keyedContract()
			bad.Idempotency.TTL = ""
			c.types[chargeType] = flow.NodeType{Purity: flow.PurityEffect, Contract: &bad}
		}, flow.CodeV10, flow.SeverityError, "ttl"},
		{"unsafe contract that retries", base, func(_ *flow.Document, c *fakeCatalog) {
			bad := unsafeContract()
			bad.RetrySafe = true
			c.types[refundType] = flow.NodeType{Purity: flow.PurityEffect, Contract: &bad}
		}, flow.CodeV10, flow.SeverityError, "retrySafe false"},
		{"attention OFF on a node whose type writes", base, func(d *flow.Document, _ *fakeCatalog) {
			node(d, "charge").Attention.Notify = flow.AttentionOff
		}, flow.CodeAttention, flow.SeverityError, "cannot be turned OFF"},
		{"attention on a pure node that does not wait", base, func(d *flow.Document, _ *fakeCatalog) {
			node(d, "child").Attention = &flow.Attention{Notify: flow.AttentionOn}
		}, flow.CodeAttention, flow.SeverityError, "neither waits"},
	})
}

func TestValidateAttentionAllowedCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		base   string
		mutate func(d *flow.Document, c *fakeCatalog)
	}{
		{"OFF on an operator task", "waits-and-boundary", func(d *flow.Document, _ *fakeCatalog) {
			node(d, "review").Attention.Notify = flow.AttentionOff
		}},
		{"OFF on a read-only effect type", "effects-calls-and-settings", func(d *flow.Document, c *fakeCatalog) {
			readOnly := flow.Contract{Effect: flow.EffectRead, Level: flow.SafetyReadOnly}
			c.types["orcon.http@1"] = flow.NodeType{Purity: flow.PurityEffect, Contract: &readOnly}
			node(d, "lookup").Attention = &flow.Attention{Notify: flow.AttentionOff}
		}},
		{"ON on a signal wait", "waits-and-boundary", func(d *flow.Document, _ *fakeCatalog) {
			node(d, "callback").Attention = &flow.Attention{Notify: flow.AttentionOn}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, tt.base)
			cat := catalogFor(&doc)
			tt.mutate(&doc, cat)
			if r := flow.ValidateWithTypes(t.Context(), &doc, cat); r.HasErrors() {
				t.Errorf("errors = %v, want none", r.Errors())
			}
		})
	}
}

func TestValidateWithoutACatalogSkipsTheNodeTypeRules(t *testing.T) {
	t.Parallel()
	doc := load(t, "effects-calls-and-settings")
	node(&doc, "charge").Attention.Notify = flow.AttentionOff
	node(&doc, "charge").Type = "vendor.unheard.of@1"
	if r := flow.Validate(&doc); slices.ContainsFunc(r.Issues, func(i flow.Issue) bool {
		return i.Code == flow.CodeV10 || i.Code == flow.CodeAttention
	}) {
		t.Errorf("Validate without a catalog reported node-type issues: %v", r.Issues)
	}
}
