// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

// endBatches maps each end node to the items it received, as canonical JSON
// texts, in arrival order.
func endBatches(res execution.Result) map[string][]string {
	out := make(map[string][]string)
	for _, end := range res.Ends {
		out[string(end.Node)] = append(out[string(end.Node)], executiontest.Texts(end.Batch)...)
	}
	return out
}

// routeBy sends an item to the ports listed in its "to" array.
func routeBy(_ *flow.Node, it execution.Item) []string {
	v, ok := it.JSON.Get("to")
	if !ok {
		return nil
	}
	var ports []string
	for _, p := range v.Items() {
		s, _ := p.Str()
		ports = append(ports, s)
	}
	return ports
}

// routingFlow builds start -> router -> {t, f} -> {et, ef}.
func routingFlow(routing flow.Routing) *executiontest.Builder {
	b := executiontest.NewFlow("routing").Start("s").If("r", routing).Task("t").Task("f").End("et").End("ef")
	b.Edge("s", "r").
		Edge("r", "t", executiontest.Port("true")).
		Edge("r", "f", executiontest.Port("false")).
		Edge("t", "et").Edge("f", "ef")
	return b
}

func TestItemRoutingSplitsABatchPerItem(t *testing.T) {
	t.Parallel()
	input := executiontest.Batch(
		`{"id":1,"to":["true"]}`,
		`{"id":2,"to":["false"]}`,
		`{"id":3,"to":["true"]}`,
		`{"id":4,"to":["false"]}`,
	)
	res := executiontest.Run(t, routingFlow(flow.RoutingExclusive),
		execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(routeBy)}, input)
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	want := map[string][]string{
		"et": {`{"id":1,"path":["t"],"to":["true"]}`, `{"id":3,"path":["t"],"to":["true"]}`},
		"ef": {`{"id":2,"path":["f"],"to":["false"]}`, `{"id":4,"path":["f"],"to":["false"]}`},
	}
	if diff := cmp.Diff(want, endBatches(res)); diff != "" {
		t.Errorf("batches per end mismatch (-want +got): each port gets its items in item order\n%s", diff)
	}
	// Two live ports: the activation forked once.
	if got := executiontest.Count(res, execution.EventTokenForked); got != 1 {
		t.Errorf("token.forked events = %d, want 1", got)
	}
}

func TestItemRoutingModes(t *testing.T) {
	t.Parallel()
	input := executiontest.Batch(`{"id":1,"to":["true","false"]}`, `{"id":2,"to":["false"]}`)
	tests := []struct {
		name    string
		routing flow.Routing
		want    map[string][]string
	}{
		{
			"exclusive keeps the first matched port", flow.RoutingExclusive,
			map[string][]string{
				"et": {`{"id":1,"path":["t"],"to":["true","false"]}`},
				"ef": {`{"id":2,"path":["f"],"to":["false"]}`},
			},
		},
		{
			"inclusive sends an item to every matched port", flow.RoutingInclusive,
			map[string][]string{
				"et": {`{"id":1,"path":["t"],"to":["true","false"]}`},
				"ef": {`{"id":1,"path":["f"],"to":["true","false"]}`, `{"id":2,"path":["f"],"to":["false"]}`},
			},
		},
		{
			"an unset routing is exclusive", "",
			map[string][]string{
				"et": {`{"id":1,"path":["t"],"to":["true","false"]}`},
				"ef": {`{"id":2,"path":["f"],"to":["false"]}`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := executiontest.Run(t, routingFlow(tt.routing),
				execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(routeBy)}, input)
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if diff := cmp.Diff(tt.want, endBatches(res)); diff != "" {
				t.Errorf("batches per end mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestItemRoutingSingleItemBehavesLikeAGateway(t *testing.T) {
	t.Parallel()
	// With one item, EXCLUSIVE is a BPMN XOR split and INCLUSIVE an OR split.
	xor := executiontest.Run(t, routingFlow(flow.RoutingExclusive),
		execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(routeBy)},
		executiontest.Batch(`{"to":["true","false"]}`))
	if got := len(xor.Ends); got != 1 {
		t.Errorf("exclusive routing of one item reached %d ends, want 1", got)
	}
	or := executiontest.Run(t, routingFlow(flow.RoutingInclusive),
		execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(routeBy)},
		executiontest.Batch(`{"to":["true","false"]}`))
	if got := len(or.Ends); got != 2 {
		t.Errorf("inclusive routing of one item reached %d ends, want 2", got)
	}
}

func TestItemRoutingEmptyPortCompletesWithZeroTokens(t *testing.T) {
	t.Parallel()
	// Every item goes to "true": the "false" edge completes with no live token,
	// so the ACTIVATED join downstream fires without waiting for it.
	b := executiontest.NewFlow("empty-port").
		Start("s").If("r", flow.RoutingExclusive).Task("t").Task("f").Parallel("j").End("e").
		Join("j", flow.JoinActivated).
		Edge("s", "r").Edge("r", "t", executiontest.Port("true")).Edge("r", "f", executiontest.Port("false")).
		Edge("t", "j").Edge("f", "j").Edge("j", "e")
	res := executiontest.Run(t, b,
		execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(routeBy)},
		executiontest.Batch(`{"to":["true"]}`, `{"to":["true"]}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "f"); got != 0 {
		t.Errorf("node on the empty port ran %d times, want 0", got)
	}
	if got := executiontest.Executed(res, "j"); got != 1 {
		t.Errorf("join ran %d times, want 1", got)
	}
	// No fork: a single live port keeps the same token.
	if got := executiontest.Count(res, execution.EventTokenForked); got != 0 {
		t.Errorf("token.forked events = %d, want 0", got)
	}
}

func TestItemRoutingUnmatchedItemsTakeTheDefaultEdge(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("default").
		Start("s").Switch("r", flow.RoutingExclusive).Task("t").Task("other").End("et").End("eo").
		Edge("s", "r").
		Edge("r", "t", executiontest.Port("case:vip")).
		Edge("r", "other", executiontest.Default()).
		Edge("t", "et").Edge("other", "eo")
	input := executiontest.Batch(`{"id":1,"to":["case:vip"]}`, `{"id":2}`, `{"id":3,"to":[]}`)
	res := executiontest.Run(t, b,
		execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(routeBy)}, input)
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	got := endBatches(res)
	if len(got["et"]) != 1 || len(got["eo"]) != 2 {
		t.Errorf("batches = %v, want 1 item at et and the 2 unmatched items at eo", got)
	}
}

func TestItemRoutingWithoutDefaultFailsOnAnUnmatchedItem(t *testing.T) {
	t.Parallel()
	res := executiontest.Run(t, routingFlow(flow.RoutingExclusive),
		execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(routeBy)},
		executiontest.Batch(`{"to":["true"]}`, `{"to":[]}`))
	if res.Status != execution.ExecFailed {
		t.Fatalf("status = %s, want FAILED", res.Status)
	}
	if res.Failure == nil || res.Failure.Code != execution.CodeNoPathTaken || res.Failure.Node != "r" {
		t.Errorf("failure = %+v, want NoPathTaken at node r", res.Failure)
	}
}

func TestItemRouterFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		router   execution.ItemRouter
		wantCode string
		wantErr  error
	}{
		{
			name: "the router panics",
			router: routerFunc(func(execution.RouteCall) ([][]string, error) {
				panic("router bug")
			}),
			wantCode: execution.CodePanic,
		},
		{
			name: "the router returns an error",
			router: routerFunc(func(execution.RouteCall) ([][]string, error) {
				return nil, executiontest.ErrBoom
			}),
			wantCode: execution.CodeNodeError,
		},
		{
			name: "the router answers for the wrong number of items",
			router: routerFunc(func(execution.RouteCall) ([][]string, error) {
				return [][]string{{"true"}}, nil
			}),
			wantErr: execution.ErrPort,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan := executiontest.MustCompile(t, routingFlow(flow.RoutingExclusive))
			in := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}, Router: tt.router})
			res, err := in.Run(t.Context(), plan, execution.Request{
				Execution: "x", Input: executiontest.Batch(`{}`, `{}`),
			})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Run() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v, want a failed execution", err)
			}
			if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != tt.wantCode {
				t.Errorf("result = %s %+v, want FAILED with code %s", res.Status, res.Failure, tt.wantCode)
			}
		})
	}
}

type routerFunc func(execution.RouteCall) ([][]string, error)

func (f routerFunc) RouteItems(_ context.Context, call execution.RouteCall) ([][]string, error) {
	return f(call)
}
