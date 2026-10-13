// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

func TestSequentialBranchesRunInEdgeOrder(t *testing.T) {
	t.Parallel()
	build := func(sequential bool) *executiontest.Builder {
		b := executiontest.NewFlow("order").Start("s").Parallel("p").Task("a").Task("b").Task("c").End("ea").End("eb").End("ec")
		if sequential {
			b.Sequential()
		}
		return b.Chain("s", "p").
			Edge("p", "a", executiontest.Order(2)).
			Edge("p", "b", executiontest.Order(1)).
			Edge("p", "c").
			Edge("a", "ea").Edge("b", "eb").Edge("c", "ec")
	}
	tests := []struct {
		name       string
		sequential bool
		want       []string
	}{
		{"parallel keeps the document order", false, []string{"a", "b", "c"}},
		{"sequential follows the edge order, unordered edges last", true, []string{"b", "a", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runner := &executiontest.TraceRunner{}
			res := executiontest.Run(t, build(tt.sequential), execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if diff := cmp.Diff(tt.want, calls(runner)); diff != "" {
				t.Errorf("activity order mismatch (-want +got):\n%s", diff)
			}
			wantDeferred := 0
			if tt.sequential {
				wantDeferred = 2
			}
			if got := executiontest.Count(res, execution.EventTokenDeferred); got != wantDeferred {
				t.Errorf("token.deferred events = %d, want %d", got, wantDeferred)
			}
		})
	}
}

func TestSequentialBranchWaitsForEveryDescendantOfThePreviousSibling(t *testing.T) {
	t.Parallel()
	// Sibling a1 forks again into a2x and a2y. In PARALLEL mode b1 runs between
	// them; in SEQUENTIAL mode b1 starts only when the whole a branch is done.
	build := func(sequential bool) *executiontest.Builder {
		b := executiontest.NewFlow("deep").
			Start("s").Parallel("p").Task("a1").Parallel("q").Task("a2x").Task("a2y").Task("b1").End("e1").End("e2").End("e3")
		if sequential {
			b.Sequential()
		}
		return b.Chain("s", "p").
			Edge("p", "a1", executiontest.Order(1)).Edge("p", "b1", executiontest.Order(2)).
			Edge("a1", "q").Edge("q", "a2x", executiontest.Order(1)).Edge("q", "a2y", executiontest.Order(2)).
			Edge("a2x", "e1").Edge("a2y", "e2").Edge("b1", "e3")
	}
	tests := []struct {
		name       string
		sequential bool
		want       []string
	}{
		{"parallel interleaves the branches", false, []string{"a1", "a2x", "b1", "a2y"}},
		{"sequential finishes a branch first", true, []string{"a1", "a2x", "a2y", "b1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runner := &executiontest.TraceRunner{}
			res := executiontest.Run(t, build(tt.sequential), execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if diff := cmp.Diff(tt.want, calls(runner)); diff != "" {
				t.Errorf("activity order mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSequentialBranchesWithAnAllJoin(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("seq-join").Sequential().
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "a", executiontest.Order(1)).Edge("p", "b", executiontest.Order(2)).
		Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"a", "b", "after"}, calls(runner)); diff != "" {
		t.Errorf("activity order mismatch (-want +got):\n%s", diff)
	}
}

func TestSequentialBranchesWithADiscriminatorCancelTheDeferredSiblings(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("seq-first").Sequential().
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinFirst).
		Chain("s", "p").Edge("p", "a", executiontest.Order(1)).Edge("p", "b", executiontest.Order(2)).
		Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"a", "after"}, calls(runner)); diff != "" {
		t.Errorf("activity order mismatch (-want +got): the deferred sibling b is canceled\n%s", diff)
	}
	if got := executiontest.TokenStates(res)[execution.TokenCanceled]; got != 1 {
		t.Errorf("canceled tokens = %d, want 1", got)
	}
}

func TestSequentialItemRoutingFollowsEdgeOrder(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("seq-route").Sequential().
		Start("s").If("r", flow.RoutingExclusive).Task("t").Task("f").End("et").End("ef").
		Edge("s", "r").
		Edge("r", "t", executiontest.Port("true"), executiontest.Order(2)).
		Edge("r", "f", executiontest.Port("false"), executiontest.Order(1)).
		Edge("t", "et").Edge("f", "ef")
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, b, execution.Deps{Runner: runner, Router: executiontest.RouterFunc(routeBy)},
		executiontest.Batch(`{"to":["true"]}`, `{"to":["false"]}`))
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if diff := cmp.Diff([]string{"f", "t"}, calls(runner)); diff != "" {
		t.Errorf("activity order mismatch (-want +got):\n%s", diff)
	}
}
