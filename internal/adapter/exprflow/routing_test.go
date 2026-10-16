// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package exprflow

import (
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func batchOf(t *testing.T, docs ...string) execution.Batch {
	t.Helper()
	items := make([]execution.Item, 0, len(docs))
	for _, d := range docs {
		v, err := jsonvalue.Parse([]byte(d))
		if err != nil {
			t.Fatalf("parse %s: %v", d, err)
		}
		items = append(items, execution.Item{JSON: v})
	}
	return execution.NewBatch(items...)
}

func ifNode(t *testing.T, condition string) *flow.Node {
	t.Helper()
	params, err := jsonvalue.Parse([]byte(`{"condition":` + quote(condition) + `}`))
	if err != nil {
		t.Fatal(err)
	}
	return &flow.Node{ID: "check", Type: IfNodeType, Parameters: params}
}

func quote(s string) string { return `"` + s + `"` }

func TestRouteItemsSendsEachItemByItsCondition(t *testing.T) {
	r := New(expr.New(expr.Options{}))
	call := execution.RouteCall{
		Node:  ifNode(t, "={{ $json.amount > 10 }}"),
		Items: batchOf(t, `{"amount": 42}`, `{"amount": 1}`),
	}
	got, err := r.RouteItems(t.Context(), call)
	if err != nil {
		t.Fatalf("route items: %v", err)
	}
	if len(got) != 2 || got[0][0] != PortTrue || got[1][0] != PortFalse {
		t.Fatalf("routes = %v, want [[true] [false]]", got)
	}
}

func TestEvalConditionReadsPlainExpressions(t *testing.T) {
	r := New(expr.New(expr.Options{}))
	ok, err := r.EvalCondition(t.Context(), `$json.amount >= 5`, execution.EvalEnv{Items: batchOf(t, `{"amount": 5}`)})
	if err != nil || !ok {
		t.Fatalf("EvalCondition = %v, %v; want true, nil", ok, err)
	}
}

func TestNonBooleanConditionFails(t *testing.T) {
	r := New(expr.New(expr.Options{}))
	_, err := r.RouteItems(t.Context(), execution.RouteCall{
		Node:  ifNode(t, "={{ $json.amount }}"),
		Items: batchOf(t, `{"amount": 42}`),
	})
	if !errors.Is(err, ErrNotBoolean) {
		t.Fatalf("err = %v, want ErrNotBoolean", err)
	}
}

func TestRouteItemsRefusesOtherNodeTypes(t *testing.T) {
	r := New(expr.New(expr.Options{}))
	node := ifNode(t, "={{ true }}")
	node.Type = "orcon.switch@1"
	_, err := r.RouteItems(t.Context(), execution.RouteCall{Node: node, Items: batchOf(t, `{}`)})
	if !errors.Is(err, ErrUnsupportedNode) {
		t.Fatalf("err = %v, want ErrUnsupportedNode", err)
	}
}
