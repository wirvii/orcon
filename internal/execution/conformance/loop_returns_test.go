// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

// Loops whose body can return several tokens in one iteration merge them: the
// next iteration starts once, after every path back in the current one has
// finished, with all the returned items (amendment 9, section C-1, option 1).

// bothBack builds start -> loop -> work -> test, where the test sends the item
// to one of two ports that both go back to the loop, or out when n reaches 2.
func bothBack(maxIterations int) *executiontest.Builder {
	return executiontest.NewFlow("both-back").
		Start("s").Loop("L", maxIterations).Task("work").If("c", flow.RoutingExclusive).End("e").
		Edge("s", "L").Edge("L", "work", executiontest.Port("loop")).Edge("work", "c").
		Edge("c", "L", executiontest.Port("again"), executiontest.Back()).
		Edge("c", "L", executiontest.Port("also"), executiontest.Back()).
		Edge("c", "e", executiontest.Port("exit"))
}

// bothBackRouter sends an item out once n reaches 2, and otherwise to the port
// its field k names.
func bothBackRouter() executiontest.RouterFunc {
	return func(_ *flow.Node, it execution.Item) []string {
		if v, ok := it.JSON.Get("n"); ok {
			if n, _ := v.Int64(); n >= 2 {
				return []string{"exit"}
			}
		}
		if v, ok := it.JSON.Get("k"); ok {
			if s, _ := v.Str(); s == "b" {
				return []string{"also"}
			}
		}
		return []string{"again"}
	}
}

func bothBackDeps() execution.Deps {
	return execution.Deps{
		Router: bothBackRouter(),
		Runner: &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"work": executiontest.Increment("n")}},
	}
}

func iterations(res execution.Result) int {
	return executiontest.Count(res, execution.EventLoopIterationStarted)
}

func TestLoopReturnsMergePerIteration(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// Three items split 2/1 between the two ports that go back. The next
	// iteration starts once, with all three, in back-edge order.
	input := executiontest.Batch(`{"id":1,"k":"a","n":0}`, `{"id":2,"k":"a","n":0}`, `{"id":3,"k":"b","n":0}`)
	res := executiontest.Run(t, bothBack(2), bothBackDeps(), input)
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := iterations(res); got != 2 {
		t.Errorf("loop.iteration_started events = %d, want 2: one per iteration, not one per return", got)
	}
	if got := executiontest.Executed(res, "work"); got != 2 {
		t.Errorf("body ran %d times, want 2", got)
	}
	if len(res.Ends) != 1 || res.Ends[0].Batch.Len() != 3 {
		t.Fatalf("ends = %+v, want one arrival with the three items", res.Ends)
	}
	var ids []string
	for _, it := range res.Ends[0].Batch.Items() {
		v, _ := it.JSON.Get("id")
		n, _ := v.Int64()
		ids = append(ids, strconv.FormatInt(n, 10))
	}
	if diff := cmp.Diff([]string{"1", "2", "3"}, ids); diff != "" {
		t.Errorf("item order mismatch (-want +got): back-edge order, then arrival order\n%s", diff)
	}
	var merged int
	for i := range res.Events {
		if e := &res.Events[i]; e.Type == execution.EventTokenConsumed && e.Detail == "LOOP_RETURN" {
			merged++
		}
	}
	if merged != 2 {
		t.Errorf("returns merged = %d, want 2 (the two tokens that came back in iteration 0)", merged)
	}
}

func TestLoopMergedReturnsRespectMaxIterations(t *testing.T) {
	t.Parallel()
	requireStore(t)
	input := executiontest.Batch(`{"id":1,"k":"a","n":0}`, `{"id":2,"k":"b","n":0}`)
	// Two returns in iteration 0 cost one iteration: a budget of 2 is enough
	// for iterations 0 and 1; a budget of 1 is not.
	if res := executiontest.Run(t, bothBack(2), bothBackDeps(), input); res.Status != execution.ExecSucceeded {
		t.Errorf("maxIterations 2: status = %s (failure %v), want SUCCEEDED", res.Status, res.Failure)
	}
	res := executiontest.Run(t, bothBack(1), bothBackDeps(), input)
	if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != execution.CodeLoopLimitExceeded {
		t.Errorf("maxIterations 1: result = %s %+v, want FAILED with LoopLimitExceeded", res.Status, res.Failure)
	}
}

func TestLoopParallelReturnsMerge(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// A parallel split in the body: both threads count and go back, each by its
	// own test. The next iteration starts once with the items of both.
	b := executiontest.NewFlow("parallel-back").
		Start("s").Loop("L", 5).Parallel("p").Task("a").Task("b").If("ca", flow.RoutingExclusive).If("cb", flow.RoutingExclusive).End("ea").End("eb").
		Edge("s", "L").Edge("L", "p", executiontest.Port("loop")).Edge("p", "a").Edge("p", "b").
		Edge("a", "ca").Edge("b", "cb").
		Edge("ca", "L", executiontest.Port("again"), executiontest.Back()).Edge("ca", "ea", executiontest.Port("exit")).
		Edge("cb", "L", executiontest.Port("again"), executiontest.Back()).Edge("cb", "eb", executiontest.Port("exit"))
	router := executiontest.RouterFunc(func(_ *flow.Node, it execution.Item) []string {
		if v, ok := it.JSON.Get("n"); ok {
			if n, _ := v.Int64(); n >= 2 {
				return []string{"exit"}
			}
		}
		return []string{"again"}
	})
	deps := execution.Deps{
		Router: router,
		Runner: &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){
			"a": executiontest.Increment("n"), "b": executiontest.Increment("n"),
		}},
	}
	res := executiontest.Run(t, b, deps, executiontest.Batch(`{"n":0}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := iterations(res); got != 2 {
		t.Errorf("loop.iteration_started events = %d, want 2", got)
	}
	// Iteration 1 receives two items (one from each thread) and each thread
	// handles both of them.
	got := map[string]int{}
	for _, end := range res.Ends {
		got[string(end.Node)] += end.Batch.Len()
	}
	if got["ea"] != 2 || got["eb"] != 2 {
		t.Errorf("items at the ends = %v, want 2 at each", got)
	}
}

func TestLoopNestedLoopsMergeTheirOwnReturns(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// The outer loop runs twice; each time the inner loop, which merges two
	// back edges, runs two iterations over the three items.
	b := executiontest.NewFlow("nested-merge").
		Start("s").Loop("L1", 3).Task("reset").Loop("L2", 3).Task("work").If("c", flow.RoutingExclusive).Task("outer").If("c1", flow.RoutingExclusive).End("e").
		Edge("s", "L1").Edge("L1", "reset", executiontest.Port("loop")).Edge("reset", "L2").
		Edge("L2", "work", executiontest.Port("loop")).Edge("work", "c").
		Edge("c", "L2", executiontest.Port("again"), executiontest.Back()).
		Edge("c", "L2", executiontest.Port("also"), executiontest.Back()).
		Edge("c", "outer", executiontest.Port("exit")).Edge("outer", "c1").
		Edge("c1", "L1", executiontest.Port("true"), executiontest.Back()).Edge("c1", "e", executiontest.Port("false"))
	inner := bothBackRouter()
	router := executiontest.RouterFunc(func(n *flow.Node, it execution.Item) []string {
		if n.ID == "c1" {
			return executiontest.WhileLess("o", 2)(n, it)
		}
		return inner(n, it)
	})
	deps := execution.Deps{
		Router: router,
		Runner: &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){
			"work": executiontest.Increment("n"), "reset": executiontest.SetField("n", 0), "outer": executiontest.Increment("o"),
		}},
	}
	input := executiontest.Batch(`{"id":1,"k":"a","o":0}`, `{"id":2,"k":"a","o":0}`, `{"id":3,"k":"b","o":0}`)
	res := executiontest.Run(t, b, deps, input)
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "work"); got != 4 {
		t.Errorf("inner body ran %d times, want 4 (2 outer x 2 inner iterations)", got)
	}
	if len(res.Ends) != 1 || res.Ends[0].Batch.Len() != 3 {
		t.Errorf("ends = %+v, want one arrival with the three items", res.Ends)
	}
}

func TestLoopSingleReturnKeepsNoState(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// A loop whose body returns one token at most: the first returning token
	// starts the next iteration at once, with no merge step.
	res := executiontest.Run(t, whileFlow(true), execution.Deps{
		Router: executiontest.WhileLess("n", 3),
		Runner: &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"work": executiontest.Increment("n")}},
	}, executiontest.Batch(`{"n":0}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	for i := range res.Events {
		if e := &res.Events[i]; e.Type == execution.EventTokenConsumed && e.Detail == "LOOP_RETURN" {
			t.Fatalf("event %d merges a return in a loop that returns one token", e.Sequence)
		}
	}
	if got := iterations(res); got != 3 {
		t.Errorf("loop.iteration_started events = %d, want 3", got)
	}
}
