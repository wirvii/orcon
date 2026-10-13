// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

func calls(r *executiontest.TraceRunner) []string {
	var out []string
	for _, id := range r.Calls() {
		out = append(out, string(id))
	}
	return out
}

func TestSequenceRunsNodesInOrder(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("sequence").
		Start("s").Task("a").Task("b").Task("c").End("e").
		Chain("s", "a", "b", "c", "e")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{"id":1}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"a", "b", "c"}, calls(runner)); diff != "" {
		t.Errorf("activity order mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"end:a>b>c"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
	if got := executiontest.TokenStates(res); len(res.Tokens) != 1 || got[execution.TokenCompleted] != 1 {
		t.Errorf("tokens = %v, want a single COMPLETED token (a sequence never forks)", res.Tokens)
	}
}

func TestExclusiveGatewayTakesFirstTrueEdge(t *testing.T) {
	t.Parallel()
	build := func() *executiontest.Builder {
		return executiontest.NewFlow("xor").
			Start("s").Exclusive("x").Task("a").Task("b").Task("d").Task("m").End("e").
			Edge("s", "x").
			Edge("x", "a", executiontest.Cond("c1")).
			Edge("x", "b", executiontest.Cond("c2")).
			Edge("x", "d", executiontest.Default()).
			Edge("a", "m").Edge("b", "m").Edge("d", "m").Edge("m", "e")
	}
	tests := []struct {
		name  string
		conds executiontest.Conditions
		want  string
	}{
		{"first true wins", executiontest.Conditions{"c1": true, "c2": true}, "end:a>m"},
		{"second true", executiontest.Conditions{"c1": false, "c2": true}, "end:b>m"},
		{"none true takes the default", executiontest.Conditions{"c1": false, "c2": false}, "end:d>m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := executiontest.Run(t, build(), execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: tt.conds}, executiontest.Batch(`{}`))
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if diff := cmp.Diff([]string{tt.want}, executiontest.Paths(res)); diff != "" {
				t.Errorf("end paths mismatch (-want +got):\n%s", diff)
			}
			if got := executiontest.Executed(res, "m"); got != 1 {
				t.Errorf("simple merge ran %d times, want 1 (only one branch is live)", got)
			}
		})
	}
}

func TestExclusiveGatewayConditionsAreLazy(t *testing.T) {
	t.Parallel()
	// c2 is not in the table: evaluating it would fail the execution, so the
	// test proves that the first true condition stops the evaluation.
	b := executiontest.NewFlow("lazy").
		Start("s").Exclusive("x").Task("a").Task("b").End("e").
		Edge("s", "x").Edge("x", "a", executiontest.Cond("c1")).Edge("x", "b", executiontest.Cond("c2")).
		Edge("a", "e").Edge("b", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"c1": true}}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
}

func TestGatewayWithoutAPathFails(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"exclusive", "inclusive"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := executiontest.NewFlow(kind).Start("s").Task("a").Task("b").End("e")
			if kind == "exclusive" {
				b.Exclusive("x")
			} else {
				b.Inclusive("x")
			}
			b.Edge("s", "x").Edge("x", "a", executiontest.Cond("c1")).Edge("x", "b", executiontest.Cond("c2")).Edge("a", "e").Edge("b", "e")
			res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"c1": false, "c2": false}}, executiontest.Batch(`{}`))
			if res.Status != execution.ExecFailed {
				t.Fatalf("status = %s, want FAILED", res.Status)
			}
			if res.Failure == nil || res.Failure.Code != execution.CodeNoPathTaken || res.Failure.Node != "x" {
				t.Errorf("failure = %+v, want NoPathTaken at node x", res.Failure)
			}
			states := executiontest.TokenStates(res)
			if states[execution.TokenFailed] != 1 {
				t.Errorf("token states = %v, want one FAILED token", states)
			}
		})
	}
}

func TestInclusiveGatewayTakesEveryTrueEdge(t *testing.T) {
	t.Parallel()
	build := func() *executiontest.Builder {
		return executiontest.NewFlow("or").
			Start("s").Inclusive("o").Task("a").Task("b").Task("d").Parallel("j").Task("after").End("e").
			Join("j", flow.JoinActivated).
			Edge("s", "o").
			Edge("o", "a", executiontest.Cond("c1")).
			Edge("o", "b", executiontest.Cond("c2")).
			Edge("o", "d", executiontest.Default()).
			Edge("a", "j").Edge("b", "j").Edge("d", "j").Chain("j", "after", "e")
	}
	tests := []struct {
		name    string
		conds   executiontest.Conditions
		want    []string
		ranJoin int
	}{
		{"both", executiontest.Conditions{"c1": true, "c2": true}, []string{"end:a>after", "end:b>after"}, 1},
		{"only the second", executiontest.Conditions{"c1": false, "c2": true}, []string{"end:b>after"}, 1},
		{"none takes the default", executiontest.Conditions{"c1": false, "c2": false}, []string{"end:d>after"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := executiontest.Run(t, build(), execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: tt.conds}, executiontest.Batch(`{}`))
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if diff := cmp.Diff(tt.want, executiontest.Paths(res)); diff != "" {
				t.Errorf("end paths mismatch (-want +got):\n%s", diff)
			}
			if got := executiontest.Executed(res, "j"); got != tt.ranJoin {
				t.Errorf("join ran %d times, want %d: the join waits for every edge to complete, then fires once", got, tt.ranJoin)
			}
		})
	}
}

func TestParallelSplitAndAllJoin(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("and").
		Start("s").Parallel("p").Task("a").Task("b").Task("c").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("p", "c").
		Edge("a", "j").Edge("b", "j").Edge("c", "j").Chain("j", "after", "e")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{"id":1}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"a", "b", "c", "after"}, calls(runner)); diff != "" {
		t.Errorf("activity order mismatch (-want +got):\n%s", diff)
	}
	// APPEND concatenates in incoming-edge order.
	if diff := cmp.Diff([]string{"end:a>after", "end:b>after", "end:c>after"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("combined batch mismatch (-want +got):\n%s", diff)
	}
	if got := executiontest.Count(res, execution.EventJoinFired); got != 1 {
		t.Errorf("join.fired events = %d, want 1", got)
	}
	states := executiontest.TokenStates(res)
	// root (forked), 3 children (consumed by the join), continuation (completed).
	if states[execution.TokenConsumed] != 4 || states[execution.TokenCompleted] != 1 {
		t.Errorf("token states = %v, want 4 CONSUMED and 1 COMPLETED", states)
	}
}

func TestJoinCombinesInIncomingEdgeOrderNotArrivalOrder(t *testing.T) {
	t.Parallel()
	// Edge b->j is declared before a->j, so b's batch comes first even though
	// the split runs a first.
	b := executiontest.NewFlow("order").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").End("e").
		Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").
		Edge("b", "j").Edge("a", "j").Edge("j", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{}`))
	if diff := cmp.Diff([]string{"end:b", "end:a"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("combined batch mismatch (-want +got):\n%s", diff)
	}
}

func TestDeadPathElimination(t *testing.T) {
	t.Parallel()
	// The b branch has three nodes and is not taken. Its edges still complete,
	// so the ACTIVATED join fires without waiting for them.
	b := executiontest.NewFlow("dpe").
		Start("s").Exclusive("x").Task("a").Task("b1").Task("b2").Task("b3").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinActivated).
		Edge("s", "x").Edge("x", "a", executiontest.Cond("go")).Edge("x", "b1", executiontest.Default()).
		Chain("b1", "b2", "b3").Edge("a", "j").Edge("b3", "j").Chain("j", "after", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"go": true}}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	for _, dead := range []string{"b1", "b2", "b3"} {
		if got := executiontest.Executed(res, dead); got != 0 {
			t.Errorf("node %s on the dead path ran %d times, want none (no node execution on a dead path)", dead, got)
		}
	}
	live := map[string]bool{}
	for _, e := range res.Events {
		if e.Type == execution.EventEdgeClosed {
			live[string(e.Edge)] = e.Live
		}
	}
	// e4, e5, e6 chain b1..b3 into the join: closed with no live token.
	for _, id := range []string{"e4", "e5", "e7"} {
		if v, ok := live[id]; !ok || v {
			t.Errorf("edge %s closed live = %v (present %v), want closed and not live", id, v, ok)
		}
	}
	if diff := cmp.Diff([]string{"end:a>after"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
}

func TestJoinAllMixedFails(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("all-fails").
		Start("s").Exclusive("x").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinAll).
		Edge("s", "x").Edge("x", "a", executiontest.Cond("go")).Edge("x", "b", executiontest.Default()).
		Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"go": true}}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecFailed {
		t.Fatalf("status = %s, want FAILED", res.Status)
	}
	if res.Failure == nil || res.Failure.Code != execution.CodeJoinRequirementNotMet || res.Failure.Node != "j" {
		t.Errorf("failure = %+v, want JoinRequirementNotMet at join j", res.Failure)
	}
	if got := executiontest.Count(res, execution.EventJoinFailed); got != 1 {
		t.Errorf("join.failed events = %d, want 1", got)
	}
	if got := executiontest.Executed(res, "after"); got != 0 {
		t.Errorf("node after the failed join ran %d times, want 0", got)
	}
	// The failing join has the node execution it would have had when firing,
	// with the same deterministic id (amendment 9 section C-3).
	fires := executiontest.Run(t, syncJoin(), execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{}`))
	var firedID, failedID execution.NodeExecutionID
	for _, ne := range fires.NodeExecutions {
		if ne.Node == "j" {
			firedID = ne.ID
		}
	}
	for _, ne := range res.NodeExecutions {
		if ne.Node == "j" {
			failedID = ne.ID
			if ne.State != execution.NodeFailed {
				t.Errorf("node execution of j is %s, want FAILED", ne.State)
			}
		}
	}
	if firedID == "" || failedID != firedID || res.Failure.NodeExecution != firedID {
		t.Errorf("node execution ids: fired %q, failed %q, failure %q; want one deterministic id", firedID, failedID, res.Failure.NodeExecution)
	}
	if res.Failure.Class != execution.ClassPermanent || res.Failure.Origin != execution.FailureEngine {
		t.Errorf("failure class %s origin %s, want PERMANENT raised by the engine", res.Failure.Class, res.Failure.Origin)
	}
	// One atomic step, in this order, with no attempt events and no continuation.
	var tail []execution.EventType
	for i := range res.Events {
		if res.Events[i].NodeExecution == failedID || res.Events[i].Type == execution.EventExecutionFailed {
			tail = append(tail, res.Events[i].Type)
		}
	}
	want := []execution.EventType{execution.EventNodeStarted, execution.EventJoinFailed, execution.EventNodeFailed, execution.EventExecutionFailed}
	if diff := cmp.Diff(want, tail); diff != "" {
		t.Errorf("events of the failing join mismatch (-want +got):\n%s", diff)
	}
	if got := executiontest.Count(res, execution.EventJoinFired); got != 0 {
		t.Errorf("join.fired events = %d, want 0", got)
	}
}

func TestActivatedJoinWithNoLiveTokenCompletesWithZeroTokens(t *testing.T) {
	t.Parallel()
	// Both branches emit nothing, so the ACTIVATED join completes its outputs
	// with zero tokens and the node after it never runs.
	empty := func(execution.NodeCall) (execution.NodeOutput, error) { return execution.NodeOutput{}, nil }
	b := executiontest.NewFlow("empty").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinActivated).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"a": empty, "b": empty}}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Count(res, execution.EventJoinFired); got != 0 {
		t.Errorf("join.fired events = %d, want 0 (nothing arrived)", got)
	}
	if got := executiontest.Executed(res, "after"); got != 0 {
		t.Errorf("node after the join ran %d times, want 0", got)
	}
	if len(res.Ends) != 0 {
		t.Errorf("end arrivals = %d, want 0", len(res.Ends))
	}
}

func TestUncontrolledMergeActivatesOncePerLiveToken(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("multi-merge").
		Start("s").Parallel("p").Task("a").Task("b").Task("m").End("e").
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "m").Edge("b", "m").Chain("m", "e")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "m"); got != 2 {
		t.Errorf("merge ran %d times, want 2", got)
	}
	if diff := cmp.Diff([]string{"end:a>m", "end:b>m"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
}

func TestMergeClosingSignalReachesAJoinDownstream(t *testing.T) {
	t.Parallel()
	// m is an uncontrolled merge that activates twice. The ACTIVATED join j has
	// a second input z. j must fire once, only after both activations of m and
	// the whole z branch are done, with all three items.
	for _, mode := range []flow.JoinMode{flow.JoinActivated, flow.JoinAll} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			b := executiontest.NewFlow("closing").
				Start("s").Parallel("p").Task("x").Task("y").Task("z").Task("m").Parallel("j").End("e").
				Join("j", mode).
				Chain("s", "p").Edge("p", "x").Edge("p", "y").Edge("p", "z").
				Edge("x", "m").Edge("y", "m").Edge("m", "j").Edge("z", "j").Chain("j", "e")
			res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{}`))
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if got := executiontest.Executed(res, "j"); got != 1 {
				t.Errorf("join ran %d times, want 1", got)
			}
			// Edge order into j: m->j was declared before z->j.
			want := []string{"end:x>m", "end:y>m", "end:z"}
			if diff := cmp.Diff(want, executiontest.Paths(res)); diff != "" {
				t.Errorf("combined batch mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDiscriminatorFiresOnFirstArrivalAndCancelsTheRest(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("first").
		Start("s").Parallel("p").Task("a").Task("b").Task("c").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinFirst).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("p", "c").
		Edge("a", "j").Edge("b", "j").Edge("c", "j").Chain("j", "after", "e")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"a", "after"}, calls(runner)); diff != "" {
		t.Errorf("activities mismatch (-want +got): b and c are canceled before they run\n%s", diff)
	}
	if diff := cmp.Diff([]string{"end:a>after"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
	states := executiontest.TokenStates(res)
	if states[execution.TokenCanceled] != 2 {
		t.Errorf("token states = %v, want the 2 losing branches CANCELED", states)
	}
	var fired execution.Event
	for _, e := range res.Events {
		if e.Type == execution.EventJoinFired {
			fired = e
		}
	}
	if len(fired.Tokens) != 2 {
		t.Errorf("join.fired canceled %d tokens, want 2", len(fired.Tokens))
	}
}

func TestDiscriminatorConsumesLateArrivalsAndFiresOnce(t *testing.T) {
	t.Parallel()
	// Two edges from the split go straight to the join, so both children are
	// already at the join when the first one fires it. The second is a late
	// arrival: consumed, recorded, never a second continuation.
	b := executiontest.NewFlow("late").
		Start("s").Parallel("p").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinFirst).
		Chain("s", "p").Edge("p", "j").Edge("p", "j").Chain("j", "after", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "after"); got != 1 {
		t.Errorf("node after the discriminator ran %d times, want 1", got)
	}
	states := executiontest.TokenStates(res)
	if states[execution.TokenCanceled] != 0 || states[execution.TokenConsumed] != 3 {
		t.Errorf("token states = %v, want no CANCELED and 3 CONSUMED (parent + 2 arrivals)", states)
	}
}

func TestDiscriminatorThatNeverArrivesCompletesWithZeroTokens(t *testing.T) {
	t.Parallel()
	empty := func(execution.NodeCall) (execution.NodeOutput, error) { return execution.NodeOutput{}, nil }
	b := executiontest.NewFlow("first-empty").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinFirst).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"a": empty, "b": empty}}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "after"); got != 0 {
		t.Errorf("node after the join ran %d times, want 0", got)
	}
}

func TestDiscriminatorOnlyCancelsTheSplitItSynchronizes(t *testing.T) {
	t.Parallel()
	// Outer split o forks u and v. u forks again (q) into a and b, which meet
	// at the discriminator j together with nothing else; v is unrelated to j
	// and must still run. The discriminator's split is q: the nearest
	// dominating split on the arriving token's lineage.
	b := executiontest.NewFlow("scoped").
		Start("s").Parallel("o").Parallel("q").Task("a").Task("b").Task("v").Parallel("j").Task("after").End("e").End("e2").
		Join("j", flow.JoinFirst).
		Chain("s", "o").Edge("o", "q").Edge("o", "v").Edge("q", "a").Edge("q", "b").
		Edge("a", "j").Edge("b", "j").Chain("j", "after", "e").Edge("v", "e2")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	got := calls(runner)
	slices.Sort(got)
	if diff := cmp.Diff([]string{"a", "after", "v"}, got); diff != "" {
		t.Errorf("activities mismatch (-want +got): only b is canceled\n%s", diff)
	}
	if n := executiontest.TokenStates(res)[execution.TokenCanceled]; n != 1 {
		t.Errorf("canceled tokens = %d, want 1 (branch b)", n)
	}
}

func TestMultipleStartEventsRunTheChosenOne(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("two-starts").
		Start("s1").Start("s2").Task("a").Task("b").Parallel("j").End("e").
		Join("j", flow.JoinActivated).
		Edge("s1", "a").Edge("s2", "b").Edge("a", "j").Edge("b", "j").Edge("j", "e")
	plan := executiontest.MustCompile(t, b)
	in := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}})
	if _, err := in.Run(t.Context(), plan, execution.Request{Execution: "x"}); !errors.Is(err, execution.ErrInvalidRequest) {
		t.Errorf("Run() with two start events and no choice = %v, want ErrInvalidRequest", err)
	}
	res, err := in.Run(t.Context(), plan, execution.Request{Execution: "x", Start: "s2", Input: executiontest.Batch(`{}`)})
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"end:b"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got): the other start is a dead path\n%s", diff)
	}
	if v := executiontest.Violations(res); len(v) > 0 {
		t.Errorf("violations: %v", v)
	}
}

func TestJoinAllDeadPathPropagates(t *testing.T) {
	t.Parallel()
	// The AND block lies on the branch the exclusive choice did not take. Its
	// ALL join sees no live token on any edge, so it is on a dead path: it does
	// not fire, does not fail, has no node execution, and completes its outgoing
	// edges with no live token (amendment 9, A.1).
	b := executiontest.NewFlow("dead-and").
		Start("s").Exclusive("x").Task("live").Parallel("p").Task("a").Task("b").Parallel("and").Task("tail").Task("m").End("e").
		Join("and", flow.JoinAll).
		Edge("s", "x").Edge("x", "live", executiontest.Cond("go")).Edge("x", "p", executiontest.Default()).
		Edge("p", "a").Edge("p", "b").Edge("a", "and").Edge("b", "and").Edge("and", "tail").Edge("tail", "m").
		Edge("live", "m").Edge("m", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"go": true}}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"end:live>m"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
	for _, node := range []string{"p", "a", "b", "and", "tail"} {
		if got := executiontest.Executed(res, node); got != 0 {
			t.Errorf("node %s on the dead path has %d node executions, want none", node, got)
		}
	}
	if executiontest.Count(res, execution.EventJoinFired)+executiontest.Count(res, execution.EventJoinFailed) != 0 {
		t.Error("the dead join emitted join.fired or join.failed")
	}
	doc := b.Doc()
	ids := map[flow.EdgeID]string{}
	for _, e := range doc.Edges {
		ids[e.ID] = string(e.From.Node) + "->" + string(e.To)
	}
	live := map[string]bool{}
	for _, e := range res.Events {
		if e.Type == execution.EventEdgeClosed {
			live[ids[e.Edge]] = e.Live
		}
	}
	for _, edge := range []string{"and->tail", "tail->m", "a->and", "b->and"} {
		if v, ok := live[edge]; !ok || v {
			t.Errorf("edge %s closed live = %v (closed %v), want closed with no live token", edge, v, ok)
		}
	}
}

func TestDiscriminatorCanceledBranchClosesEdges(t *testing.T) {
	t.Parallel()
	// Branch b is canceled when a wins the discriminator j, but b also feeds the
	// ACTIVATED join x outside the block. Cancellation must close b's edges with
	// no live token, so x completes and the execution ends.
	b := executiontest.NewFlow("cancel-closes").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").Parallel("x").Task("done").End("e").
		Join("j", flow.JoinFirst).Join("x", flow.JoinActivated).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Edge("b", "x").
		Edge("j", "after").Edge("after", "x").Chain("x", "done", "e")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "b"); got != 0 {
		t.Errorf("canceled branch b ran %d times, want 0", got)
	}
	if got := executiontest.Executed(res, "x"); got != 1 {
		t.Errorf("the external join x ran %d times, want 1: it must not wait for the canceled branch", got)
	}
	if diff := cmp.Diff([]string{"end:a>after>done"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
}

func TestDiscriminatorCancelsOnlyTheNearestDominatingSplit(t *testing.T) {
	t.Parallel()
	// The split is fixed at Compile time as the nearest dominating split from
	// which the join is reachable through two branches (amendment 9, A.4).
	b := executiontest.NewFlow("nearest").
		Start("s").Parallel("outer").Parallel("inner").Task("a").Task("b").Task("c").Parallel("j").End("e").End("e2").
		Join("j", flow.JoinFirst).
		Chain("s", "outer").Edge("outer", "inner").Edge("outer", "c").
		Edge("inner", "a").Edge("inner", "b").Edge("a", "j").Edge("b", "j").Edge("j", "e").Edge("c", "e2")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	got := calls(runner)
	slices.Sort(got)
	if diff := cmp.Diff([]string{"a", "c"}, got); diff != "" {
		t.Errorf("activities mismatch (-want +got): only b, the other branch of the inner split, is canceled\n%s", diff)
	}
}

func TestLetFinishDoesNotAbsorbTheFailureOfAnotherBranch(t *testing.T) {
	t.Parallel()
	// The join fired on a's arrival, but b still runs and fails: with
	// STOP_EXECUTION the execution fails, the join does not hide it.
	b := executiontest.NewFlow("let-finish-fails").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinFirst).Remaining("j", flow.RemainingLetFinish).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
	fail := func(execution.NodeCall) (execution.NodeOutput, error) {
		return execution.NodeOutput{}, executiontest.ErrBoom
	}
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"b": fail}}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Node != "b" {
		t.Fatalf("result = %s %+v, want FAILED at node b", res.Status, res.Failure)
	}
	if executiontest.Count(res, execution.EventJoinFired) != 1 {
		t.Error("the join did not fire on the first arrival before b failed")
	}
}

func TestLetFinishRoundsGetIdsFromTheirSplitInstance(t *testing.T) {
	t.Parallel()
	// The continuation ids of the two rounds of the join differ, and they are
	// the same in two runs: they derive from the split instance, not from the
	// order in which the rounds were numbered.
	run := func() []execution.TokenID {
		res := executiontest.Run(t, wcp28Like(), execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{}`))
		var ids []execution.TokenID
		for i := range res.Events {
			if e := &res.Events[i]; e.Type == execution.EventJoinFired {
				ids = append(ids, e.Token)
			}
		}
		return ids
	}
	first, second := run(), run()
	if len(first) != 2 || first[0] == first[1] {
		t.Fatalf("continuation ids = %v, want two different ids", first)
	}
	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("continuation ids differ between runs (-first +second):\n%s", diff)
	}
}

func wcp28Like() *executiontest.Builder {
	return executiontest.NewFlow("rounds").
		Start("s").Parallel("p0").Task("a").Task("b").Task("m").Parallel("sp").Task("x").Task("y").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinFirst).Remaining("j", flow.RemainingLetFinish).
		Chain("s", "p0").Edge("p0", "a").Edge("p0", "b").Edge("a", "m").Edge("b", "m").Edge("m", "sp").
		Edge("sp", "x").Edge("sp", "y").Edge("x", "j").Edge("y", "j").Chain("j", "after", "e")
}

// canceledInputFlow has a FIRST/CANCEL discriminator j over a and b, and an
// external join x that receives b directly and the continuation of j.
func canceledInputFlow(xMode flow.JoinMode) *executiontest.Builder {
	return executiontest.NewFlow("canceled-input").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").Parallel("x").Task("done").End("e").
		Join("j", flow.JoinFirst).Remaining("j", flow.RemainingCancel).Join("x", xMode).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Edge("b", "x").
		Edge("j", "after").Edge("after", "x").Chain("x", "done", "e")
}

func TestJoinAllCanceledInputFails(t *testing.T) {
	t.Parallel()
	// An edge closed by cancellation counts like any edge closed with no live
	// token: x gets a live token from the winner and a canceled input, so it
	// fails with JoinRequirementNotMet and a failed node execution.
	res := executiontest.Run(t, canceledInputFlow(flow.JoinAll), execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != execution.CodeJoinRequirementNotMet || res.Failure.Node != "x" {
		t.Fatalf("result = %s %+v, want FAILED with JoinRequirementNotMet at x", res.Status, res.Failure)
	}
	var failed int
	for _, ne := range res.NodeExecutions {
		if ne.Node == "x" && ne.State == execution.NodeFailed && ne.ID == res.Failure.NodeExecution {
			failed++
		}
	}
	if failed != 1 {
		t.Errorf("failed node executions of x = %d, want 1 matching the failure", failed)
	}
}

func TestJoinInCanceledRegionIsCanceled(t *testing.T) {
	t.Parallel()
	// The ALL join jj lies in branch B of the split p. A discriminator on p
	// cancels B after b1 reached jj; the other input of jj closes by
	// cancellation. jj neither fires nor fails and has no node execution, and
	// the execution ends through the winning branch.
	b := executiontest.NewFlow("canceled-region").
		Start("s").Parallel("p").Task("bb").Parallel("q").Task("b1").Task("b2").Parallel("jj").Task("a").Parallel("j").Task("after").End("e").
		Join("jj", flow.JoinAll).Join("j", flow.JoinFirst).Remaining("j", flow.RemainingCancel).
		Chain("s", "p").Edge("p", "bb").Edge("p", "a").Edge("bb", "q").Edge("q", "b1").Edge("q", "b2").
		Edge("b1", "jj").Edge("b2", "jj").Edge("jj", "j").Edge("a", "j").Chain("j", "after", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "jj"); got != 0 {
		t.Errorf("the join in the canceled region has %d node executions, want none", got)
	}
	for _, e := range res.Events {
		if (e.Type == execution.EventJoinFired || e.Type == execution.EventJoinFailed) && e.Node == "jj" {
			t.Errorf("event %s for the canceled join", e.Type)
		}
	}
	if diff := cmp.Diff([]string{"end:a>after"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
}

// syncJoin is a split with an ALL join in which both branches arrive.
func syncJoin() *executiontest.Builder {
	return executiontest.NewFlow("sync").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
}
