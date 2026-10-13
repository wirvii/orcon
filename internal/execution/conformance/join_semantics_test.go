// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

// The tests below carry the Orcon join semantics of amendment 9 into the
// conformance suite, under names the E-CF-2 selection (TestJoin|TestDPE|
// TestItemRouting|TestSequentialBranches|TestLoop) matches.

func TestJoinAllDeadPathPropagates(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// The AND block lies on the branch the exclusive choice did not take: its
	// ALL join neither fires nor fails, has no node execution, and closes its
	// outgoing edges with no live token.
	b := executiontest.NewFlow("dead-and").
		Start("s").Exclusive("x").Task("live").Parallel("p").Task("a").Task("b").Parallel("and").Task("tail").Task("m").End("e").
		Join("and", flow.JoinAll).
		Edge("s", "x").Edge("x", "live", executiontest.Cond("go")).Edge("x", "p", executiontest.Default()).
		Edge("p", "a").Edge("p", "b").Edge("a", "and").Edge("b", "and").Edge("and", "tail").Edge("tail", "m").
		Edge("live", "m").Edge("m", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"go": true}}, one())
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	for _, node := range []string{"p", "a", "b", "and", "tail"} {
		if got := executiontest.Executed(res, node); got != 0 {
			t.Errorf("node %s on the dead path has %d node executions, want none", node, got)
		}
	}
	if executiontest.Count(res, execution.EventJoinFired)+executiontest.Count(res, execution.EventJoinFailed) != 0 {
		t.Error("the dead join emitted join.fired or join.failed")
	}
	if diff := cmp.Diff([]string{"end:live>m"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
}

func TestJoinAllMixedFails(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// One branch of the split arrives live and the other was not taken.
	b := executiontest.NewFlow("mixed").
		Start("s").Exclusive("x").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinAll).
		Edge("s", "x").Edge("x", "a", executiontest.Cond("go")).Edge("x", "b", executiontest.Default()).
		Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"go": true}}, one())
	if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != execution.CodeJoinRequirementNotMet || res.Failure.Node != "j" {
		t.Fatalf("result = %s %+v, want FAILED with JoinRequirementNotMet at j", res.Status, res.Failure)
	}
	// The failing join has a FAILED node execution and no continuation.
	failed := 0
	for _, ne := range res.NodeExecutions {
		if ne.Node == "j" && ne.State == execution.NodeFailed && ne.ID == res.Failure.NodeExecution {
			failed++
		}
	}
	if failed != 1 || executiontest.Count(res, execution.EventJoinFired) != 0 {
		t.Errorf("failed node executions of j = %d, join.fired = %d, want 1 and 0", failed, executiontest.Count(res, execution.EventJoinFired))
	}
}

func TestJoinAllCanceledInputFails(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// The losing branch of a FIRST/CANCEL discriminator also feeds an external
	// ALL join that receives a live token from the winner: a canceled edge
	// counts like any edge closed with no live token.
	b := executiontest.NewFlow("canceled-input").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").Parallel("x").Task("done").End("e").
		Join("j", flow.JoinFirst).Remaining("j", flow.RemainingCancel).Join("x", flow.JoinAll).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Edge("b", "x").
		Edge("j", "after").Edge("after", "x").Chain("x", "done", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, one())
	if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != execution.CodeJoinRequirementNotMet || res.Failure.Node != "x" {
		t.Fatalf("result = %s %+v, want FAILED with JoinRequirementNotMet at x", res.Status, res.Failure)
	}
}

func TestJoinInCanceledRegionIsCanceled(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// The ALL join jj lies in branch B of the split p; the discriminator on p
	// cancels B after b1 reached jj. jj neither fires nor fails, has no node
	// execution, and the execution ends through the winning branch.
	b := executiontest.NewFlow("canceled-region").
		Start("s").Parallel("p").Task("bb").Parallel("q").Task("b1").Task("b2").Parallel("jj").Task("a").Parallel("j").Task("after").End("e").
		Join("jj", flow.JoinAll).Join("j", flow.JoinFirst).Remaining("j", flow.RemainingCancel).
		Chain("s", "p").Edge("p", "bb").Edge("p", "a").Edge("bb", "q").Edge("q", "b1").Edge("q", "b2").
		Edge("b1", "jj").Edge("b2", "jj").Edge("jj", "j").Edge("a", "j").Chain("j", "after", "e")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, one())
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "jj"); got != 0 {
		t.Errorf("the join in the canceled region has %d node executions, want none", got)
	}
	if diff := cmp.Diff([]string{"end:a>after"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("end paths mismatch (-want +got):\n%s", diff)
	}
}

func TestJoinFirstCancelDiscriminator(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// FIRST with CANCEL: the first arrival fires, the branches that have not
	// reached the join are canceled, and a canceled branch that also feeds a
	// join outside the block closes its edge, so that join completes.
	b := executiontest.NewFlow("cancel").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").Parallel("x").Task("done").End("e").
		Join("j", flow.JoinFirst).Remaining("j", flow.RemainingCancel).Join("x", flow.JoinActivated).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Edge("b", "x").
		Edge("j", "after").Edge("after", "x").Chain("x", "done", "e")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, one())
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if got := executiontest.Executed(res, "b"); got != 0 {
		t.Errorf("canceled branch b ran %d times, want 0", got)
	}
	if got := executiontest.Executed(res, "x"); got != 1 {
		t.Errorf("the external join x ran %d times, want 1", got)
	}
	if got := executiontest.TokenStates(res)[execution.TokenCanceled]; got != 1 {
		t.Errorf("canceled tokens = %d, want 1", got)
	}
}

func TestJoinLetFinishLetsTheOtherBranchesFinish(t *testing.T) {
	t.Parallel()
	requireStore(t)
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, branchesFlow(flow.JoinFirst).Remaining("j", flow.RemainingLetFinish), execution.Deps{Runner: runner}, one())
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"a", "after", "b", "c"}, sorted(tasks(runner))); diff != "" {
		t.Errorf("every branch runs, the continuation once (-want +got):\n%s", diff)
	}
	if got := executiontest.TokenStates(res)[execution.TokenCanceled]; got != 0 {
		t.Errorf("canceled tokens = %d, want none", got)
	}
}

func TestJoinLetFinishRoundsWaitForTheRoundBefore(t *testing.T) {
	t.Parallel()
	requireStore(t)
	// Two split instances in one scope (an uncontrolled merge before the
	// split): the second round fires only after the first reset.
	res := executiontest.Run(t, wcp28Flow(), execution.Deps{Runner: &executiontest.TraceRunner{}}, one())
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	seq := func(typ execution.EventType, round int) int {
		for i := range res.Events {
			if e := &res.Events[i]; e.Type == typ && e.Round == round {
				return e.Sequence
			}
		}
		return 0
	}
	fired1, reset1, fired2 := seq(execution.EventJoinFired, 1), seq(execution.EventJoinReset, 1), seq(execution.EventJoinFired, 2)
	if fired1 == 0 || reset1 == 0 || fired2 == 0 || fired1 >= reset1 || reset1 >= fired2 {
		t.Errorf("round events out of order: fired1=%d reset1=%d fired2=%d", fired1, reset1, fired2)
	}
	if got := executiontest.Executed(res, "after"); got != 2 {
		t.Errorf("the node after the join ran %d times, want 2", got)
	}
}

// canceledInsideLoop builds the case where a discriminator cancels a branch in
// the body of a loop that is itself live: the loop's exit must carry a live
// token to the ALL join k, which also waits for y.
func canceledInsideLoop(remaining flow.JoinRemaining) *executiontest.Builder {
	b := executiontest.NewFlow("loop-cancel").
		Start("s").Parallel("p0").Task("y").Loop("L", 3).Parallel("p").Task("a").Task("b").Parallel("j").Task("work").If("more", flow.RoutingExclusive).
		Parallel("k").Task("done").End("e").
		Join("j", flow.JoinFirst).Join("k", flow.JoinAll).
		Chain("s", "p0").Edge("p0", "y").Edge("p0", "L").
		Edge("L", "p", executiontest.Port("loop")).Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").
		Edge("j", "work").Edge("work", "more").
		Edge("more", "L", executiontest.Port("true"), executiontest.Back()).
		Edge("more", "k", executiontest.Port("false")).Edge("y", "k").Chain("k", "done", "e")
	if remaining != "" {
		b.Remaining("j", remaining)
	}
	return b
}

func TestLoopCanceledBranchInsideBodyKeepsTheExits(t *testing.T) {
	t.Parallel()
	for _, remaining := range []flow.JoinRemaining{flow.RemainingCancel, flow.RemainingLetFinish} {
		t.Run(string(remaining), func(t *testing.T) {
			t.Parallel()
			requireStore(t)
			deps := execution.Deps{
				Router: executiontest.WhileLess("n", 1),
				Runner: &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"work": executiontest.Increment("n")}},
			}
			res := executiontest.Run(t, canceledInsideLoop(remaining), deps, executiontest.Batch(`{"n":0}`))
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if canceled := executiontest.TokenStates(res)[execution.TokenCanceled]; (canceled > 0) != (remaining == flow.RemainingCancel) {
				t.Errorf("canceled tokens = %d with remaining %s: the CANCEL case must cancel a branch, LET_FINISH none", canceled, remaining)
			}
			if got := executiontest.Executed(res, "done"); got != 1 {
				t.Errorf("the node after the ALL join ran %d times, want 1: the loop exit is live, canceling a branch of its body does not cancel it", got)
			}
		})
	}
}
