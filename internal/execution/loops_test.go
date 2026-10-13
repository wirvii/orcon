// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"slices"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

// counterLoop builds start -> loop -> work -> more -> {back to loop, end}.
func counterLoop(maxIterations int) *executiontest.Builder {
	return executiontest.NewFlow("loop").
		Start("s").Loop("L", maxIterations).Task("work").If("more", flow.RoutingExclusive).End("e").
		Edge("s", "L").
		Edge("L", "work", executiontest.Port("loop")).
		Edge("work", "more").
		Edge("more", "L", executiontest.Port("true"), executiontest.Back()).
		Edge("more", "e", executiontest.Port("false"))
}

func counterDeps(limit int64) execution.Deps {
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){
		"work": executiontest.Increment("n"),
	}}
	return execution.Deps{Runner: runner, Router: executiontest.WhileLess("n", limit)}
}

func scopesOf(res execution.Result, node string) []string {
	var out []string
	for _, ne := range res.NodeExecutions {
		if ne.Node == flow.NodeID(node) {
			out = append(out, executiontest.MaskEntries(string(ne.Scope)))
		}
	}
	return out
}

func TestLoopRunsUntilTheBodyExits(t *testing.T) {
	t.Parallel()
	res := executiontest.Run(t, counterLoop(5), counterDeps(3), executiontest.Batch(`{"n":0}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	wantScopes := []string{"L:L#0", "L:L#1", "L:L#2"}
	for _, node := range []string{"L", "work", "more"} {
		if diff := cmp.Diff(wantScopes, scopesOf(res, node)); diff != "" {
			t.Errorf("scopes of %s mismatch (-want +got): one execution per iteration scope\n%s", node, diff)
		}
	}
	if diff := cmp.Diff([]string{`{"n":3,"path":["work","work","work"]}`}, endBatches(res)["e"]); diff != "" {
		t.Errorf("end batch mismatch (-want +got):\n%s", diff)
	}
	if len(res.Ends) != 1 || res.Ends[0].Scope != "" {
		t.Errorf("ends = %+v, want one arrival back at the top-level scope", res.Ends)
	}
	var iterations []int
	for _, e := range res.Events {
		if e.Type == execution.EventLoopIterationStarted {
			iterations = append(iterations, e.Iteration)
		}
	}
	if diff := cmp.Diff([]int{0, 1, 2}, iterations); diff != "" {
		t.Errorf("loop.iteration_started iterations mismatch (-want +got):\n%s", diff)
	}
	if got := executiontest.TokenStates(res); len(res.Tokens) != 1 || got[execution.TokenCompleted] != 1 {
		t.Errorf("tokens = %d %v, want the single root token, COMPLETED", len(res.Tokens), got)
	}
}

func TestLoopExactlyAtMaxIterationsSucceeds(t *testing.T) {
	t.Parallel()
	res := executiontest.Run(t, counterLoop(3), counterDeps(3), executiontest.Batch(`{"n":0}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED: 3 iterations fit maxIterations 3 (failure %v)", res.Status, res.Failure)
	}
}

func TestLoopLimitExceeded(t *testing.T) {
	t.Parallel()
	res := executiontest.Run(t, counterLoop(2), counterDeps(3), executiontest.Batch(`{"n":0}`))
	if res.Status != execution.ExecFailed {
		t.Fatalf("status = %s, want FAILED", res.Status)
	}
	if f := res.Failure; f == nil || f.Code != execution.CodeLoopLimitExceeded || f.Node != "L" || f.Scope != "" {
		t.Errorf("failure = %+v, want LoopLimitExceeded at loop L in the top-level scope", f)
	}
	var limit *execution.Event
	for i, e := range res.Events {
		if e.Type == execution.EventLoopLimitExceeded {
			limit = &res.Events[i]
		}
	}
	if limit == nil || limit.Iteration != 2 {
		t.Errorf("loop.limit_exceeded = %+v, want iteration 2", limit)
	}
	if got := executiontest.Executed(res, "work"); got != 2 {
		t.Errorf("body ran %d times, want 2 (the third iteration never starts)", got)
	}
}

func TestLoopPerItemExitsWaitForTheLoopBeforeCompletingTheirEdge(t *testing.T) {
	t.Parallel()
	// Items leave the loop at different iterations. The join after the loop must
	// see every exit token before the exit edge completes: the completion signal
	// of an exit edge comes once, after the whole loop is done.
	b := executiontest.NewFlow("loop-join").
		Start("s").Parallel("p").Loop("L", 10).Task("work").If("more", flow.RoutingExclusive).Task("z").Parallel("j").End("e").
		Join("j", flow.JoinActivated).
		Edge("s", "p").Edge("p", "L").Edge("p", "z").
		Edge("L", "work", executiontest.Port("loop")).Edge("work", "more").
		Edge("more", "L", executiontest.Port("true"), executiontest.Back()).
		Edge("z", "j").Edge("more", "j", executiontest.Port("false")).Edge("j", "e")
	deps := counterDeps(3)
	res := executiontest.Run(t, b, deps, executiontest.Batch(`{"n":0}`, `{"n":2}`, `{"n":5}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "j"); got != 1 {
		t.Fatalf("join ran %d times, want 1: it waits for the loop to close", got)
	}
	got := endBatches(res)["e"]
	slices.Sort(got)
	want := []string{
		`{"n":3,"path":["work","work","work"]}`,
		`{"n":3,"path":["work"]}`,
		`{"n":6,"path":["work"]}`,
		// z receives the three input items untouched by the loop.
		`{"n":0,"path":["z"]}`,
		`{"n":2,"path":["z"]}`,
		`{"n":5,"path":["z"]}`,
	}
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("joined batch mismatch (-want +got):\n%s", diff)
	}
	if got := executiontest.Executed(res, "work"); got != 3 {
		t.Errorf("body ran %d times, want 3 iterations", got)
	}
}

func TestLoopThatIsNeverEnteredCompletesItsExitEdges(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("dead-loop").
		Start("s").Exclusive("x").Loop("L", 3).Task("work").If("more", flow.RoutingExclusive).Task("skip").Parallel("j").End("e").
		Join("j", flow.JoinActivated).
		Edge("s", "x").Edge("x", "L", executiontest.Cond("enter")).Edge("x", "skip", executiontest.Default()).
		Edge("L", "work", executiontest.Port("loop")).Edge("work", "more").
		Edge("more", "L", executiontest.Port("true"), executiontest.Back()).
		Edge("more", "j", executiontest.Port("false")).Edge("skip", "j").Edge("j", "e")
	deps := counterDeps(3)
	deps.Conditions = executiontest.Conditions{"enter": false}
	res := executiontest.Run(t, b, deps, executiontest.Batch(`{"n":0}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "work") + executiontest.Executed(res, "L"); got != 0 {
		t.Errorf("loop nodes ran %d times, want 0", got)
	}
	if got := executiontest.Executed(res, "j"); got != 1 {
		t.Errorf("join ran %d times, want 1: the dead loop completed its exit edge with no token", got)
	}
}

func TestNestedLoopsHaveNestedScopePaths(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("nested").
		Start("s").Loop("L1", 5).Task("a").Loop("L2", 5).Task("b").If("m2", flow.RoutingExclusive).If("m1", flow.RoutingExclusive).End("e").
		Edge("s", "L1").
		Edge("L1", "a", executiontest.Port("loop")).
		Edge("a", "L2").
		Edge("L2", "b", executiontest.Port("loop")).
		Edge("b", "m2").
		Edge("m2", "L2", executiontest.Port("true"), executiontest.Back()).
		Edge("m2", "m1", executiontest.Port("false")).
		Edge("m1", "L1", executiontest.Port("true"), executiontest.Back()).
		Edge("m1", "e", executiontest.Port("false"))
	reset := func(call execution.NodeCall) (execution.NodeOutput, error) {
		out, err := executiontest.SetField("i", 0)(call)
		if err != nil {
			return out, err
		}
		call.Input = out.Ports[0].Batch
		return executiontest.Increment("o")(call)
	}
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){
		"a": reset, "b": executiontest.Increment("i"),
	}}
	router := executiontest.RouterFunc(func(n *flow.Node, it execution.Item) []string {
		if n.ID == "m2" {
			return executiontest.WhileLess("i", 2)(n, it)
		}
		return executiontest.WhileLess("o", 3)(n, it)
	})
	res := executiontest.Run(t, b, execution.Deps{Runner: runner, Router: router}, executiontest.Batch(`{"o":0,"i":0}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	wantB := []string{
		"L:L1#0/L:L2#0", "L:L1#0/L:L2#1",
		"L:L1#1/L:L2#0", "L:L1#1/L:L2#1",
		"L:L1#2/L:L2#0", "L:L1#2/L:L2#1",
	}
	if diff := cmp.Diff(wantB, scopesOf(res, "b")); diff != "" {
		t.Errorf("scopes of b mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"L:L1#0", "L:L1#1", "L:L1#2"}, scopesOf(res, "a")); diff != "" {
		t.Errorf("scopes of a mismatch (-want +got):\n%s", diff)
	}
	if len(res.Ends) != 1 || res.Ends[0].Scope != "" {
		t.Errorf("ends = %+v, want one arrival at the top-level scope", res.Ends)
	}
	if got := executiontest.Field(res.Ends[0].Batch, "o"); got != "3" {
		t.Errorf("final o = %s, want 3", got)
	}
}

func TestJoinsInsideALoopMatchWithinTheIterationScope(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("loop-fork").
		Start("s").Loop("L", 5).Parallel("p").Task("x").Task("y").Parallel("j").If("more", flow.RoutingExclusive).End("e").
		Join("j", flow.JoinAll).
		Edge("s", "L").Edge("L", "p", executiontest.Port("loop")).Edge("p", "x").Edge("p", "y").
		Edge("x", "j").Edge("y", "j").Edge("j", "more").
		Edge("more", "L", executiontest.Port("true"), executiontest.Back()).Edge("more", "e", executiontest.Port("false"))
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){
		"x": executiontest.Increment("n"), "y": executiontest.Increment("n"),
	}}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner, Router: executiontest.WhileLess("n", 2)}, executiontest.Batch(`{"n":0}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"L:L#0", "L:L#1"}, scopesOf(res, "j")); diff != "" {
		t.Errorf("scopes of the join mismatch (-want +got): one join instance per iteration\n%s", diff)
	}
	if got := executiontest.Count(res, execution.EventJoinFired); got != 2 {
		t.Errorf("join.fired events = %d, want 2", got)
	}
	// Iteration 0: 1 item -> 2 after the join. Iteration 1: 2 items to each of two branches -> 4.
	if len(res.Ends) != 1 || res.Ends[0].Batch.Len() != 4 {
		t.Errorf("ends = %+v, want one arrival with 4 items", res.Ends)
	}
}

func TestLoopBackEdgeFromAGatewayWithConditions(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("gateway-loop").
		Start("s").Loop("L", 5).Task("work").Exclusive("again").End("e").
		Edge("s", "L").Edge("L", "work", executiontest.Port("loop")).Edge("work", "again").
		Edge("again", "L", executiontest.Cond("repeat"), executiontest.Back()).
		Edge("again", "e", executiontest.Default())
	var mu sync.Mutex
	var scopes []string
	asked := 0
	conds := executiontest.ConditionFunc(func(_ string, env execution.EvalEnv) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		asked++
		scopes = append(scopes, executiontest.MaskEntries(string(env.Scope)))
		return asked <= 2, nil
	})
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: conds}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"L:L#0", "L:L#1", "L:L#2"}, scopes); diff != "" {
		t.Errorf("scopes the conditions saw mismatch (-want +got):\n%s", diff)
	}
	if got := executiontest.Executed(res, "work"); got != 3 {
		t.Errorf("body ran %d times, want 3", got)
	}
}
