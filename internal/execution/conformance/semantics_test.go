// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

// emitNothing is a node behavior for a branch that is not taken: it emits no
// item, so its outgoing edges complete with no live token.
func emitNothing(execution.NodeCall) (execution.NodeOutput, error) {
	return execution.NodeOutput{}, nil
}

// branchesFlow builds start -> split -> {a, b, c} -> join(mode) -> after -> end.
func branchesFlow(mode flow.JoinMode) *executiontest.Builder {
	return executiontest.NewFlow("branches").
		Start("s").Parallel("p").Task("a").Task("b").Task("c").Parallel("j").Task("after").End("e").
		Join("j", mode).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("p", "c").
		Edge("a", "j").Edge("b", "j").Edge("c", "j").Chain("j", "after", "e")
}

// runnerTaking returns a runner whose listed branches emit and whose others do not.
func runnerTaking(taken ...string) *executiontest.TraceRunner {
	behaviors := map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){}
	for _, id := range []string{"a", "b", "c"} {
		behaviors[flow.NodeID(id)] = emitNothing
	}
	for _, id := range taken {
		delete(behaviors, flow.NodeID(id))
	}
	return &executiontest.TraceRunner{Behaviors: behaviors}
}

func TestJoinModes(t *testing.T) {
	t.Parallel()
	const joinFailed = "JoinRequirementNotMet"
	tests := []struct {
		mode  flow.JoinMode
		taken []string
		// wantPaths are the paths at the end event; wantFail is the failure code.
		wantPaths []string
		wantFail  string
	}{
		{flow.JoinAll, []string{"a", "b", "c"}, []string{"end:a>after", "end:b>after", "end:c>after"}, ""},
		{flow.JoinAll, []string{"a", "c"}, nil, joinFailed},
		// No branch is live: the join lies on a dead path and completes empty.
		{flow.JoinAll, nil, nil, ""},
		{flow.JoinActivated, []string{"a", "b", "c"}, []string{"end:a>after", "end:b>after", "end:c>after"}, ""},
		{flow.JoinActivated, []string{"b", "c"}, []string{"end:b>after", "end:c>after"}, ""},
		{flow.JoinActivated, []string{"c"}, []string{"end:c>after"}, ""},
		{flow.JoinActivated, nil, nil, ""},
		// The discriminator takes the first live arrival; the branches after it
		// are canceled, so only that one reaches the node after the join.
		{flow.JoinFirst, []string{"a", "b", "c"}, []string{"end:a>after"}, ""},
		{flow.JoinFirst, []string{"b", "c"}, []string{"end:b>after"}, ""},
		{flow.JoinFirst, []string{"c"}, []string{"end:c>after"}, ""},
		{flow.JoinFirst, nil, nil, ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s taking %v", tt.mode, tt.taken), func(t *testing.T) {
			t.Parallel()
			requireStore(t)
			res := executiontest.Run(t, branchesFlow(tt.mode), execution.Deps{Runner: runnerTaking(tt.taken...)}, one())
			if tt.wantFail != "" {
				if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != tt.wantFail {
					t.Fatalf("result = %s %+v, want FAILED with %s", res.Status, res.Failure, tt.wantFail)
				}
				return
			}
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if diff := cmp.Diff(tt.wantPaths, executiontest.Paths(res)); diff != "" {
				t.Errorf("end paths mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDPEEdgesCloseExactlyOncePerScope(t *testing.T) {
	t.Parallel()
	// Whatever the branches take, every edge of the flow carries exactly one
	// completion signal: the run helper checks it for every run, and here the
	// total is compared with the number of edges.
	for _, taken := range [][]string{{"a", "b", "c"}, {"b"}, {}} {
		t.Run(fmt.Sprint(taken), func(t *testing.T) {
			t.Parallel()
			requireStore(t)
			b := branchesFlow(flow.JoinActivated)
			res := executiontest.Run(t, b, execution.Deps{Runner: runnerTaking(taken...)}, one())
			want := len(b.Doc().Edges)
			if got := executiontest.Count(res, execution.EventEdgeClosed); got != want {
				t.Errorf("edge.closed events = %d, want %d (one per edge)", got, want)
			}
		})
	}
}

func TestDPEPropagatesThroughNodesThatNeverRun(t *testing.T) {
	t.Parallel()
	for _, length := range []int{0, 1, 3, 8} {
		t.Run(fmt.Sprintf("dead branch of %d nodes", length), func(t *testing.T) {
			t.Parallel()
			requireStore(t)
			b := executiontest.NewFlow("dead").Start("s").Exclusive("x").Task("live").Parallel("j").Task("after").End("e").
				Join("j", flow.JoinActivated).
				Edge("s", "x").Edge("x", "live", executiontest.Cond("live")).Edge("live", "j")
			prev := "x"
			for i := range length {
				id := fmt.Sprintf("d%d", i)
				b.Task(id)
				if prev == "x" {
					b.Edge("x", id, executiontest.Default())
				} else {
					b.Edge(prev, id)
				}
				prev = id
			}
			if prev == "x" {
				b.Edge("x", "j", executiontest.Default())
			} else {
				b.Edge(prev, "j")
			}
			b.Chain("j", "after", "e")
			res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"live": true}}, one())
			if res.Status != execution.ExecSucceeded {
				t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
			}
			if diff := cmp.Diff([]string{"end:live>after"}, executiontest.Paths(res)); diff != "" {
				t.Errorf("end paths mismatch (-want +got):\n%s", diff)
			}
			for i := range length {
				if got := executiontest.Executed(res, fmt.Sprintf("d%d", i)); got != 0 {
					t.Errorf("dead node d%d ran %d times, want 0", i, got)
				}
			}
		})
	}
}

func TestItemRoutingSendsEachItemWhereItBelongs(t *testing.T) {
	t.Parallel()
	route := func(_ *flow.Node, it execution.Item) []string {
		v, _ := it.JSON.Get("to")
		var ports []string
		for _, p := range v.Items() {
			s, _ := p.Str()
			ports = append(ports, s)
		}
		return ports
	}
	build := func(routing flow.Routing) *executiontest.Builder {
		return executiontest.NewFlow("route").Start("s").If("r", routing).Task("t").Task("f").End("et").End("ef").
			Edge("s", "r").Edge("r", "t", executiontest.Port("true")).Edge("r", "f", executiontest.Port("false")).
			Edge("t", "et").Edge("f", "ef")
	}
	tests := []struct {
		routing flow.Routing
		input   execution.Batch
		wantT   int
		wantF   int
	}{
		{flow.RoutingExclusive, executiontest.Batch(`{"id":1,"to":["true"]}`, `{"id":2,"to":["false"]}`, `{"id":3,"to":["true"]}`), 2, 1},
		{flow.RoutingInclusive, executiontest.Batch(`{"id":1,"to":["true"]}`, `{"id":2,"to":["false"]}`, `{"id":3,"to":["true","false"]}`), 2, 2},
	}
	for _, tt := range tests {
		t.Run(string(tt.routing), func(t *testing.T) {
			t.Parallel()
			requireStore(t)
			res := executiontest.Run(t, build(tt.routing), execution.Deps{Runner: &executiontest.TraceRunner{}, Router: executiontest.RouterFunc(route)}, tt.input)
			got := map[string]int{}
			for _, end := range res.Ends {
				got[string(end.Node)] += end.Batch.Len()
			}
			if got["et"] != tt.wantT || got["ef"] != tt.wantF {
				t.Errorf("items per port = %v, want true=%d false=%d", got, tt.wantT, tt.wantF)
			}
		})
	}
}

func TestSequentialBranchesRunOneAfterAnother(t *testing.T) {
	t.Parallel()
	requireStore(t)
	b := executiontest.NewFlow("seq").Sequential().
		Start("s").Parallel("p").Task("a1").Task("a2").Task("b1").End("e1").End("e2").
		Chain("s", "p").
		Edge("p", "b1", executiontest.Order(2)).Edge("p", "a1", executiontest.Order(1)).
		Chain("a1", "a2", "e1").Edge("b1", "e2")
	runner := &executiontest.TraceRunner{}
	executiontest.Run(t, b, execution.Deps{Runner: runner}, one())
	var got []string
	for _, id := range runner.Calls() {
		got = append(got, string(id))
	}
	if diff := cmp.Diff([]string{"a1", "a2", "b1"}, got); diff != "" {
		t.Errorf("order mismatch (-want +got): the sibling with order 1 runs to its end first\n%s", diff)
	}
}

func TestLoopLimitAndScopePaths(t *testing.T) {
	t.Parallel()
	build := func(limit int) *executiontest.Builder {
		return executiontest.NewFlow("loop").Start("s").Loop("L", limit).Task("work").If("more", flow.RoutingExclusive).End("e").
			Edge("s", "L").Edge("L", "work", executiontest.Port("loop")).Edge("work", "more").
			Edge("more", "L", executiontest.Port("true"), executiontest.Back()).Edge("more", "e", executiontest.Port("false"))
	}
	deps := func() execution.Deps {
		return execution.Deps{
			Router: executiontest.WhileLess("n", 4),
			Runner: &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"work": executiontest.Increment("n")}},
		}
	}
	t.Run("within the limit", func(t *testing.T) {
		t.Parallel()
		requireStore(t)
		res := executiontest.Run(t, build(4), deps(), executiontest.Batch(`{"n":0}`))
		if res.Status != execution.ExecSucceeded {
			t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
		}
		var scopes []string
		for _, ne := range res.NodeExecutions {
			if ne.Node == "work" {
				scopes = append(scopes, string(ne.Scope))
			}
		}
		want := []string{"L:L#0", "L:L#1", "L:L#2", "L:L#3"}
		if diff := cmp.Diff(want, scopes); diff != "" {
			t.Errorf("iteration scopes mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("beyond the limit", func(t *testing.T) {
		t.Parallel()
		requireStore(t)
		res := executiontest.Run(t, build(3), deps(), executiontest.Batch(`{"n":0}`))
		if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != execution.CodeLoopLimitExceeded {
			t.Fatalf("result = %s %+v, want FAILED with LoopLimitExceeded", res.Status, res.Failure)
		}
	})
}
