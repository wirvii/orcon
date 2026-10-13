// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
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
	input := executiontest.Batch(`{"id":1,"to":["true"]}`, `{"id":2,"to":["false"]}`)
	tests := []struct {
		name    string
		routing flow.Routing
		want    map[string][]string
	}{
		{
			"exclusive with one matched port per item", flow.RoutingExclusive,
			map[string][]string{
				"et": {`{"id":1,"path":["t"],"to":["true"]}`},
				"ef": {`{"id":2,"path":["f"],"to":["false"]}`},
			},
		},
		{
			"inclusive sends an item to every matched port", flow.RoutingInclusive,
			map[string][]string{
				"et": {`{"id":1,"path":["t"],"to":["true"]}`},
				"ef": {`{"id":2,"path":["f"],"to":["false"]}`},
			},
		},
		{
			"an unset routing is exclusive", "",
			map[string][]string{
				"et": {`{"id":1,"path":["t"],"to":["true"]}`},
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

func TestInclusiveRoutingSendsAnItemToEveryMatchedPort(t *testing.T) {
	t.Parallel()
	res := executiontest.Run(t, routingFlow(flow.RoutingInclusive),
		execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(routeBy)},
		executiontest.Batch(`{"id":1,"to":["true","false"]}`, `{"id":2,"to":["false"]}`))
	want := map[string][]string{
		"et": {`{"id":1,"path":["t"],"to":["true","false"]}`},
		"ef": {`{"id":1,"path":["f"],"to":["true","false"]}`, `{"id":2,"path":["f"],"to":["false"]}`},
	}
	if diff := cmp.Diff(want, endBatches(res)); diff != "" {
		t.Errorf("batches per end mismatch (-want +got):\n%s", diff)
	}
}

// closedEdges lists "edge live" for every edge.closed event, by edge position:
// the edges of the two flows below are declared in the same order, so their
// lists compare directly.
func closedEdges(res execution.Result) []string {
	var out []string
	for i := range res.Events {
		if e := &res.Events[i]; e.Type == execution.EventEdgeClosed {
			out = append(out, string(e.Edge)+" live="+strconv.FormatBool(e.Live))
		}
	}
	slices.Sort(out)
	return out
}

func TestItemRoutingOfOneItemMatchesTheBPMNGateways(t *testing.T) {
	t.Parallel()
	gateways := map[flow.Routing]func(*executiontest.Builder, string) *executiontest.Builder{
		flow.RoutingExclusive: func(b *executiontest.Builder, id string) *executiontest.Builder { return b.Exclusive(id) },
		flow.RoutingInclusive: func(b *executiontest.Builder, id string) *executiontest.Builder { return b.Inclusive(id) },
	}
	tests := []struct {
		routing flow.Routing
		ports   []string // the ports the item matches
		conds   executiontest.Conditions
	}{
		{flow.RoutingExclusive, []string{"true"}, executiontest.Conditions{"t": true, "f": false}},
		{flow.RoutingExclusive, []string{"false"}, executiontest.Conditions{"t": false, "f": true}},
		{flow.RoutingExclusive, nil, executiontest.Conditions{"t": false, "f": false}},
		{flow.RoutingInclusive, []string{"true", "false"}, executiontest.Conditions{"t": true, "f": true}},
		{flow.RoutingInclusive, []string{"false"}, executiontest.Conditions{"t": false, "f": true}},
		{flow.RoutingInclusive, nil, executiontest.Conditions{"t": false, "f": false}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %v", tt.routing, tt.ports), func(t *testing.T) {
			t.Parallel()
			// Item routing node: the unmatched item takes the default edge to d.
			routed := executiontest.NewFlow("routed").Start("s").If("r", tt.routing).Task("t").Task("f").Task("d").End("e").
				Edge("s", "r").
				Edge("r", "t", executiontest.Port("true")).Edge("r", "f", executiontest.Port("false")).
				Edge("r", "d", executiontest.Port("dflt"), executiontest.Default()).
				Edge("t", "e").Edge("f", "e").Edge("d", "e")
			// BPMN gateway with the same verdicts.
			gate := gateways[tt.routing](executiontest.NewFlow("gate").Start("s"), "r").Task("t").Task("f").Task("d").End("e").
				Edge("s", "r").
				Edge("r", "t", executiontest.Cond("t")).Edge("r", "f", executiontest.Cond("f")).
				Edge("r", "d", executiontest.Default()).
				Edge("t", "e").Edge("f", "e").Edge("d", "e")
			item := executiontest.Batch(`{"id":1}`)
			router := executiontest.RouterFunc(func(*flow.Node, execution.Item) []string { return tt.ports })
			a := executiontest.Run(t, routed, execution.Deps{Runner: &executiontest.TraceRunner{}, Router: router}, item)
			b := executiontest.Run(t, gate, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: tt.conds}, item)
			if a.Status != b.Status {
				t.Fatalf("routing node ends %s, gateway ends %s", a.Status, b.Status)
			}
			ran := func(res execution.Result) []string {
				var out []string
				for _, n := range []string{"t", "f", "d"} {
					if executiontest.Executed(res, n) > 0 {
						out = append(out, n)
					}
				}
				return out
			}
			if diff := cmp.Diff(ran(b), ran(a)); diff != "" {
				t.Errorf("nodes run differ (-gateway +routing):\n%s", diff)
			}
			if diff := cmp.Diff(closedEdges(b), closedEdges(a)); diff != "" {
				t.Errorf("edge closures differ (-gateway +routing):\n%s", diff)
			}
			if x, y := executiontest.Count(b, execution.EventTokenForked), executiontest.Count(a, execution.EventTokenForked); x != y {
				t.Errorf("forks: gateway %d, routing node %d", x, y)
			}
		})
	}
}

func TestItemRoutingKeepsTheOutputOfAPortWithoutAnEdge(t *testing.T) {
	t.Parallel()
	store := execution.NewMemoryPayloads()
	b := executiontest.NewFlow("edgeless").Start("s").If("r", flow.RoutingExclusive).Task("t").End("e").
		Edge("s", "r").Edge("r", "t", executiontest.Port("true")).Edge("t", "e")
	router := executiontest.RouterFunc(func(_ *flow.Node, it execution.Item) []string {
		if v, _ := it.JSON.Get("id"); v.Kind() == 0 {
			return nil
		}
		return []string{"nowhere"}
	})
	before := store.Len()
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Router: router, Payloads: store}, executiontest.Batch(`{"id":1}`))
	if res.Status != execution.ExecSucceeded || len(res.Ends) != 0 {
		t.Fatalf("result = %s with %d ends, want SUCCEEDED where the items stop at the port", res.Status, len(res.Ends))
	}
	// Input, the start output and the router's output on the port "nowhere".
	if got := store.Len() - before; got != 3 {
		t.Errorf("stored batches = %d, want 3 (input, start output, edgeless port output)", got)
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
			wantCode: "NodeError",
		},
		{
			name: "the router answers two ports for one item of an EXCLUSIVE node",
			router: routerFunc(func(execution.RouteCall) ([][]string, error) {
				return [][]string{{"true", "false"}, {"true"}}, nil
			}),
			wantErr: execution.ErrPort,
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
