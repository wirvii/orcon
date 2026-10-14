// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestOperationSelectionOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		params jsonvalue.Value
		want   flow.Selection
	}{
		{"no parameters", jsonvalue.Value{}, flow.Selection{}},
		{"no selector", obj(jsonvalue.Member{Key: "other", Value: jsonvalue.String("x")}), flow.Selection{}},
		{"a literal", obj(jsonvalue.Member{Key: "operation", Value: jsonvalue.String("order.create")}), flow.Selection{Present: true, Literal: true, Operation: "order.create"}},
		{"an expression", obj(jsonvalue.Member{Key: "operation", Value: jsonvalue.String("={{ $json.op }}")}), flow.Selection{Present: true}},
		{"not a string", obj(jsonvalue.Member{Key: "operation", Value: jsonvalue.Int(3)}), flow.Selection{Present: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := flow.SelectionOf(&flow.Node{Parameters: tt.params}); got != tt.want {
				t.Errorf("SelectionOf() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestOperationSelectionMessages: each selection problem is a V10 error at the
// selector with a message of its own that names the node.
func TestOperationSelectionMessages(t *testing.T) {
	t.Parallel()
	doc := withConnectorNode(t, &flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest})
	tests := []struct {
		problem flow.SelectionProblem
		want    []string
	}{
		{flow.SelectionMissing, []string{`node "fetch"`, "must select", "a.b, c.d"}},
		{flow.SelectionNotLiteral, []string{`node "fetch"`, "expression"}},
		{flow.SelectionUnknown, []string{`node "fetch"`, `"order.create"`, "a.b, c.d"}},
		{flow.SelectionOnTrigger, []string{`node "fetch"`, "trigger node types select no operation"}},
	}
	for _, tt := range tests {
		t.Run(tt.want[1], func(t *testing.T) {
			t.Parallel()
			catalog := resolver{resolve: func(*flow.Node) flow.NodeResolution {
				return flow.NodeResolution{Known: true, Selection: tt.problem, Operations: []string{"a.b", "c.d"}}
			}}
			errs := flow.ValidateWithTypes(t.Context(), &doc, catalog).Errors()
			if len(errs) != 1 || errs[0].Code != flow.CodeV10 || errs[0].Path != "/nodes/1/parameters/operation" {
				t.Fatalf("errors = %v", errs)
			}
			for _, want := range tt.want {
				if !strings.Contains(errs[0].Message, want) {
					t.Errorf("message %q does not mention %q", errs[0].Message, want)
				}
			}
		})
	}
}

// safetyNet is a catalog whose resolver does not know the core type that its
// plain lookup does know.
type safetyNet struct{ resolved *int }

func (safetyNet) Lookup(_ context.Context, nodeType string) (flow.NodeType, bool) {
	if strings.HasPrefix(nodeType, "orcon.") {
		return flow.NodeType{Purity: flow.PurityRead}, true
	}
	return flow.NodeType{}, false
}

func (s safetyNet) ResolveNode(context.Context, *flow.Node) flow.NodeResolution {
	*s.resolved++
	return flow.NodeResolution{}
}

// TestCompositeCatalogSafetyNet: a type no package can own that the resolver
// does not know is looked up before it is called unknown, and never receives
// selection errors; a package type is not rescued by the lookup.
func TestCompositeCatalogSafetyNet(t *testing.T) {
	t.Parallel()
	t.Run("core type", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "linear")
		calls := 0
		if r := flow.ValidateWithTypes(t.Context(), &doc, safetyNet{resolved: &calls}); r.HasErrors() {
			t.Errorf("errors = %v", r.Errors())
		}
		if calls == 0 {
			t.Error("the resolver was never asked")
		}
	})
	t.Run("a selection reported for a core type is ignored", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "linear")
		if r := flow.ValidateWithTypes(t.Context(), &doc, coreWithSelection{}); r.HasErrors() {
			t.Errorf("errors = %v", r.Errors())
		}
	})
	t.Run("unknown everywhere", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "linear")
		node(&doc, "fetch").Type = "vendor.thing@1"
		calls := 0
		r := flow.ValidateWithTypes(t.Context(), &doc, safetyNet{resolved: &calls})
		if !r.Has(flow.CodeV10, flow.SeverityError) {
			t.Errorf("errors = %v", r.Errors())
		}
	})
	t.Run("package type", func(t *testing.T) {
		t.Parallel()
		doc := withConnectorNode(t, &flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest})
		calls := 0
		r := flow.ValidateWithTypes(t.Context(), &doc, safetyNet{resolved: &calls})
		if !r.Has(flow.CodeV10, flow.SeverityError) {
			t.Errorf("an unknown package type passed: %v", r.Errors())
		}
	})
}

// coreWithSelection resolves every node as a known core type but, wrongly,
// attaches a selection problem: the validator must not report it.
type coreWithSelection struct{}

func (coreWithSelection) Lookup(context.Context, string) (flow.NodeType, bool) {
	return flow.NodeType{Purity: flow.PurityRead}, true
}

func (coreWithSelection) ResolveNode(context.Context, *flow.Node) flow.NodeResolution {
	return flow.NodeResolution{Known: true, Type: flow.NodeType{Purity: flow.PurityRead}, Selection: flow.SelectionMissing}
}
