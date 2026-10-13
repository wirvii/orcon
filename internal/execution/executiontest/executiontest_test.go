// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

func TestBuilderProducesValidFlows(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("everything").Sequential().IterationVariable("total").
		Start("s").Task("a").Parallel("p").Parallel("j").Exclusive("x").Inclusive("o").If("i", flow.RoutingExclusive).
		Switch("sw", flow.RoutingInclusive).Loop("L", 3).End("e").
		Join("j", flow.JoinAll).
		Modify("a", func(n *flow.Node) { n.Disabled = nil }).
		Chain("s", "a", "p").Edge("p", "j", executiontest.Order(2), executiontest.Cond("c"), executiontest.Default(), executiontest.Port("main"))
	doc := b.Doc()
	if len(doc.Nodes) != 10 || len(doc.Edges) != 3 {
		t.Fatalf("doc has %d nodes and %d edges, want 10 and 3", len(doc.Nodes), len(doc.Edges))
	}
	if doc.Settings.BranchMode != flow.BranchSequential || len(doc.Variables) != 1 {
		t.Errorf("settings %+v variables %v: Sequential and IterationVariable did not apply", doc.Settings, doc.Variables)
	}
	edge := doc.Edges[2]
	if edge.Order != 2 || edge.Condition != "c" || edge.Default == nil || !*edge.Default || edge.From.Port != "main" {
		t.Errorf("edge options not applied: %+v", edge)
	}
	// Doc returns a copy: changing it does not change the builder.
	doc.Nodes[0].Name = "changed"
	if b.Doc().Nodes[0].Name == "changed" {
		t.Error("Doc() shares its node slice with the builder")
	}
	if got := executiontest.MustCompile(t, executiontest.NewFlow("ok").Start("s").End("e").Chain("s", "e")).FlowID(); got != executiontest.FlowID {
		t.Errorf("FlowID() = %q", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("Modify of an unknown node did not panic")
		}
	}()
	b.Modify("nope", func(*flow.Node) {})
}

func TestFakes(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	env := execution.EvalEnv{}
	if got, err := (executiontest.Conditions{"c": true}).EvalCondition(ctx, "c", env); err != nil || !got {
		t.Errorf("Conditions known = (%v, %v), want true", got, err)
	}
	if _, err := (executiontest.Conditions{}).EvalCondition(ctx, "missing", env); err == nil {
		t.Error("Conditions without the expression succeeded, want an error")
	}
	fn := executiontest.ConditionFunc(func(expr string, _ execution.EvalEnv) (bool, error) { return expr == "yes", nil })
	if got, _ := fn.EvalCondition(ctx, "yes", env); !got {
		t.Error("ConditionFunc did not answer true")
	}

	batch := executiontest.Batch(`{"route":"a"}`, `{"route":7}`, `{}`)
	got, err := executiontest.FieldRouter("route").RouteItems(ctx, execution.RouteCall{Items: batch})
	if err != nil || !cmp.Equal([][]string{{"a"}, nil, nil}, got) {
		t.Errorf("FieldRouter = (%v, %v), want [[a] [] []]", got, err)
	}
	limit := executiontest.WhileLess("n", 2)
	ports := []string{
		limit(nil, executiontest.JSONItem(`{"n":1}`))[0],
		limit(nil, executiontest.JSONItem(`{"n":2}`))[0],
		limit(nil, executiontest.JSONItem(`{}`))[0],
	}
	if diff := cmp.Diff([]string{"true", "false", "true"}, ports); diff != "" {
		t.Errorf("WhileLess ports mismatch (-want +got):\n%s", diff)
	}
}

func TestTraceRunnerAndItemHelpers(t *testing.T) {
	t.Parallel()
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){
		"inc": executiontest.Increment("n"),
		"set": executiontest.SetField("n", 9),
	}}
	node := func(id string) *flow.Node { return &flow.Node{ID: flow.NodeID(id)} }
	in := executiontest.Batch(`{"n":1}`, `[1]`)
	for _, tt := range []struct {
		node string
		want []string
	}{
		{"plain", []string{`{"n":1,"path":["plain"]}`, `[1]`}},
		{"inc", []string{`{"n":2,"path":["inc"]}`, `[1]`}},
		{"set", []string{`{"n":9,"path":["set"]}`, `[1]`}},
	} {
		out, err := runner.Execute(t.Context(), execution.NodeCall{Node: node(tt.node), Input: in})
		if err != nil {
			t.Fatalf("%s: %v", tt.node, err)
		}
		if diff := cmp.Diff(tt.want, executiontest.Texts(out.Ports[0].Batch)); diff != "" {
			t.Errorf("%s output mismatch (-want +got):\n%s", tt.node, diff)
		}
	}
	if diff := cmp.Diff([]flow.NodeID{"plain", "inc", "set"}, runner.Calls()); diff != "" {
		t.Errorf("Calls() mismatch (-want +got):\n%s", diff)
	}
	if got := executiontest.Field(executiontest.Batch(`{"n":5}`), "n"); got != "5" {
		t.Errorf("Field() = %q, want 5", got)
	}
	if executiontest.Field(execution.Batch{}, "n") != "" || executiontest.Field(executiontest.Batch(`{}`), "n") != "" {
		t.Error("Field() of an empty batch or a missing field is not empty")
	}
	defer func() {
		if recover() == nil {
			t.Error("JSONItem of invalid JSON did not panic")
		}
	}()
	executiontest.JSONItem(`{`)
}

func TestRecorder(t *testing.T) {
	t.Parallel()
	rec := &executiontest.Recorder{FailAt: 2, Err: executiontest.ErrBoom}
	if err := rec.Emit(t.Context(), execution.Event{Sequence: 1}); err != nil {
		t.Fatalf("Emit(1) = %v", err)
	}
	if err := rec.Emit(t.Context(), execution.Event{Sequence: 2}); !errors.Is(err, executiontest.ErrBoom) {
		t.Errorf("Emit(2) = %v, want the configured error", err)
	}
	if got := rec.Events(); len(got) != 1 {
		t.Errorf("Events() = %v, want only the event before the failure", got)
	}
}

func TestHarnessHelpers(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("h").Start("s").Parallel("p").Task("a").Task("b").End("ea").End("eb").
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "ea").Edge("b", "eb")
	res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{"id":1}`))
	if diff := cmp.Diff([]string{"end:a", "end:b"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("Paths mismatch (-want +got):\n%s", diff)
	}
	if executiontest.Executed(res, "a") != 1 || executiontest.Executed(res, "zzz") != 0 {
		t.Error("Executed counts are wrong")
	}
	if executiontest.Count(res, execution.EventTokenForked) != 1 {
		t.Error("Count(token.forked) != 1")
	}
	types := executiontest.EventTypes(res)
	if len(types) != len(res.Events) || types[0] != execution.EventExecutionStarted {
		t.Errorf("EventTypes() = %v", types)
	}
	if states := executiontest.TokenStates(res); states[execution.TokenCompleted] != 2 || states[execution.TokenConsumed] != 1 {
		t.Errorf("TokenStates() = %v", states)
	}
	if trace := executiontest.Trace(res); !strings.HasPrefix(trace, "1 execution.started\n2 node.started node=s") {
		t.Errorf("Trace() starts %q", trace[:min(len(trace), 60)])
	}
}

func TestViolationsReportsBrokenRuns(t *testing.T) {
	t.Parallel()
	broken := execution.Result{
		Status: execution.ExecSucceeded,
		Events: []execution.Event{
			{Sequence: 1, Type: execution.EventNodeStarted},
			{Sequence: 3, Type: execution.EventEdgeClosed, Edge: "e1", Scope: "s"},
			{Sequence: 4, Type: execution.EventEdgeClosed, Edge: "e1", Scope: "s"},
		},
		Tokens: []execution.TokenRecord{{ID: "t1", State: execution.TokenRunnable}},
		NodeExecutions: []execution.NodeExecutionRecord{
			{ID: "n1", State: execution.NodeRunning}, {ID: "n1", State: execution.NodeSucceeded},
		},
	}
	want := []string{
		"event 2 has sequence 3",
		"edge e1 closed twice in scope \"s\"",
		"0 terminal execution events, want 1",
		"token t1 ended the run RUNNABLE",
		"1 node.started events but 0 node.succeeded or node.failed",
		"node execution n1 appears twice",
		"node execution n1 of  is still RUNNING",
	}
	got := executiontest.Violations(broken)
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("Violations() lacks %q; got %q", w, got)
		}
	}
	if v := executiontest.Violations(execution.Result{Status: execution.ExecSucceeded, Events: []execution.Event{
		{Sequence: 1, Type: execution.EventExecutionSucceeded},
	}}); len(v) != 0 {
		t.Errorf("Violations() of a clean run = %q, want none", v)
	}
}

func TestGenerateIsDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	for seed := range byte(40) {
		data := []byte{seed, seed * 7, seed * 13, seed * 31, 200, 100, 50, 25, 12, 6, 3, 1}
		first := executiontest.Generate(data, executiontest.GenOptions{})
		second := executiontest.Generate(data, executiontest.GenOptions{})
		if diff := cmp.Diff(first.Want, second.Want); diff != "" {
			t.Fatalf("seed %d: Generate is not deterministic (-first +second):\n%s", seed, diff)
		}
		if !first.HasReference {
			t.Errorf("seed %d: a program without discriminators has no reference result", seed)
		}
		if len(first.Want) > 200 {
			t.Errorf("seed %d: the reference result has %d items, want a bounded program", seed, len(first.Want))
		}
	}
	empty := executiontest.Generate(nil, executiontest.GenOptions{Discriminators: true, MaxNodes: 3})
	if empty.Flow == nil || empty.Input.Len() == 0 {
		t.Error("Generate(nil) did not produce a program")
	}
}

func TestRandomGraphsAreValidAndRunnable(t *testing.T) {
	t.Parallel()
	failed := 0
	const graphs = 60
	for seed := range byte(graphs) {
		data := []byte{seed, seed * 3, seed * 5, seed * 7, seed * 11, seed * 13, seed * 17, seed * 19, 255 - seed, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		doc := executiontest.RandomGraph(data).Doc()
		if err := flow.Validate(&doc).Err(); err != nil {
			t.Fatalf("seed %d: invalid flow: %v", seed, err)
		}
		plan, err := execution.Compile(&doc)
		if err != nil {
			t.Fatalf("seed %d: Compile() = %v", seed, err)
		}
		res, err := execution.NewInterpreter(executiontest.ChaosDeps(&doc)).
			Run(t.Context(), plan, execution.Request{Execution: "x", Input: executiontest.Batch(`{"a":1}`)})
		if err != nil {
			t.Fatalf("seed %d: Run() = %v", seed, err)
		}
		if v := executiontest.Violations(res); len(v) > 0 {
			t.Errorf("seed %d: %v", seed, v)
		}
		if res.Status == execution.ExecFailed {
			failed++
		}
	}
	if failed == graphs {
		t.Error("every random graph failed; the generator no longer makes runnable flows")
	}
}
