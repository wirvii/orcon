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

func linearFlow() *executiontest.Builder {
	return executiontest.NewFlow("linear").Start("s").Task("a").Task("b").End("e").Chain("s", "a", "b", "e")
}

func forkFlow() *executiontest.Builder {
	return executiontest.NewFlow("fork").
		Start("s").Parallel("p").Task("a").Task("b").End("ea").End("eb").
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "ea").Edge("b", "eb")
}

func TestRunReportsAMissingPortBeforeStarting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		b    *executiontest.Builder
	}{
		{"activity nodes need a NodeRunner", linearFlow()},
		{"edge conditions need a ConditionEvaluator", executiontest.NewFlow("cond").
			Start("s").Exclusive("x").End("e").Edge("s", "x").Edge("x", "e", executiontest.Cond("c"))},
		{"item routing needs an ItemRouter", executiontest.NewFlow("route").
			Start("s").If("r", flow.RoutingExclusive).End("e").Edge("s", "r").Edge("r", "e", executiontest.Port("true"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan := executiontest.MustCompile(t, tt.b)
			_, err := execution.NewInterpreter(execution.Deps{}).Run(t.Context(), plan, execution.Request{Execution: "x"})
			if !errors.Is(err, execution.ErrMissingPort) {
				t.Errorf("Run() error = %v, want ErrMissingPort", err)
			}
		})
	}
}

func TestRunRejectsAnInvalidRequest(t *testing.T) {
	t.Parallel()
	plan := executiontest.MustCompile(t, linearFlow())
	noStart := executiontest.MustCompile(t, executiontest.NewFlow("empty"))
	tests := []struct {
		name string
		plan *execution.Plan
		req  execution.Request
	}{
		{"no plan", nil, execution.Request{Execution: "x"}},
		{"no execution id", plan, execution.Request{}},
		{"unknown start node", plan, execution.Request{Execution: "x", Start: "nope"}},
		{"start node is not a start event", plan, execution.Request{Execution: "x", Start: "a"}},
		{"the flow has no start event", noStart, execution.Request{Execution: "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}}).Run(t.Context(), tt.plan, tt.req)
			if !errors.Is(err, execution.ErrInvalidRequest) {
				t.Errorf("Run() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestEmptyInputEmitsNothing(t *testing.T) {
	t.Parallel()
	runner := &executiontest.TraceRunner{}
	res := executiontest.Run(t, linearFlow(), execution.Deps{Runner: runner}, execution.Batch{})
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	if len(runner.Calls()) != 0 || len(res.Ends) != 0 {
		t.Errorf("calls %v, ends %d; an empty batch is not live data and nothing runs", runner.Calls(), len(res.Ends))
	}
}

func TestNodeErrorStopsTheExecution(t *testing.T) {
	t.Parallel()
	fail := func(execution.NodeCall) (execution.NodeOutput, error) {
		return execution.NodeOutput{}, executiontest.ErrBoom
	}
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"a": fail}}
	res := executiontest.Run(t, forkFlow(), execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecFailed {
		t.Fatalf("status = %s, want FAILED", res.Status)
	}
	f := res.Failure
	if f == nil || f.Code != execution.CodeNodeError || f.Node != "a" || !errors.Is(f.Cause, executiontest.ErrBoom) {
		t.Fatalf("failure = %+v, want NodeError at a wrapping the node's error", f)
	}
	states := executiontest.TokenStates(res)
	if states[execution.TokenFailed] != 1 || states[execution.TokenCanceled] != 1 {
		t.Errorf("token states = %v, want the failing token FAILED and its sibling CANCELED", states)
	}
	for _, ne := range res.NodeExecutions {
		if ne.Node == "a" && ne.State != execution.NodeFailed {
			t.Errorf("node execution of a is %s, want FAILED", ne.State)
		}
	}
	if got := executiontest.Executed(res, "b"); got != 0 {
		t.Errorf("sibling b ran %d times after the failure, want 0", got)
	}
}

func TestPanickingPortsFailTheExecutionAndTheProcessSurvives(t *testing.T) {
	t.Parallel()
	boom := func(execution.NodeCall) (execution.NodeOutput, error) { panic("connector bug") }
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"a": boom}}
	res := executiontest.Run(t, linearFlow(), execution.Deps{Runner: runner}, executiontest.Batch(`{}`))
	if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != execution.CodePanic {
		t.Fatalf("result = %s %+v, want FAILED with code Panic", res.Status, res.Failure)
	}
	var pe *execution.PanicError
	if !errors.As(res.Failure.Cause, &pe) || pe.Value != "connector bug" || len(pe.Stack) == 0 {
		t.Errorf("failure cause = %v, want a PanicError carrying the value and the stack", res.Failure.Cause)
	}
}

func TestConditionEvaluatorFailures(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("cond").Start("s").Exclusive("x").Task("a").End("e").
		Edge("s", "x").Edge("x", "a", executiontest.Cond("c")).Edge("a", "e")
	tests := []struct {
		name     string
		conds    execution.ConditionEvaluator
		wantCode string
	}{
		{"an evaluation error", executiontest.Conditions{}, execution.CodeExpressionError},
		{"a panic", executiontest.ConditionFunc(func(string, execution.EvalEnv) (bool, error) { panic("sandbox bug") }), execution.CodePanic},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: tt.conds}, executiontest.Batch(`{}`))
			if res.Status != execution.ExecFailed || res.Failure == nil || res.Failure.Code != tt.wantCode || res.Failure.Node != "x" {
				t.Errorf("result = %s %+v, want FAILED with code %s at node x", res.Status, res.Failure, tt.wantCode)
			}
		})
	}
}

func TestRunnerThatEmitsAPortTwiceBreaksItsContract(t *testing.T) {
	t.Parallel()
	dup := func(call execution.NodeCall) (execution.NodeOutput, error) {
		pb := execution.PortBatch{Port: flow.PortMain, Batch: call.Input}
		return execution.NodeOutput{Ports: []execution.PortBatch{pb, pb}}, nil
	}
	runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"a": dup}}
	plan := executiontest.MustCompile(t, linearFlow())
	_, err := execution.NewInterpreter(execution.Deps{Runner: runner}).Run(t.Context(), plan, execution.Request{Execution: "x", Input: executiontest.Batch(`{}`)})
	if !errors.Is(err, execution.ErrPort) {
		t.Errorf("Run() error = %v, want ErrPort", err)
	}
}

func TestCancellationStopsTheRun(t *testing.T) {
	t.Parallel()
	plan := executiontest.MustCompile(t, linearFlow())
	t.Run("before the start", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}}).Run(ctx, plan, execution.Request{Execution: "x"})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() error = %v, want context.Canceled", err)
		}
	})
	t.Run("while a node runs", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		canceling := func(call execution.NodeCall) (execution.NodeOutput, error) {
			cancel()
			return execution.NodeOutput{}, ctx.Err()
		}
		runner := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"a": canceling}}
		res, err := execution.NewInterpreter(execution.Deps{Runner: runner}).Run(ctx, plan, execution.Request{Execution: "x", Input: executiontest.Batch(`{}`)})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled: a canceled run is not a failed execution", err)
		}
		if res.Status != execution.ExecRunning || res.Failure != nil {
			t.Errorf("result = %s %+v, want an interrupted RUNNING execution without a failure", res.Status, res.Failure)
		}
	})
}

func TestEventSinkReceivesTheTraceInOrder(t *testing.T) {
	t.Parallel()
	rec := &executiontest.Recorder{}
	res := executiontest.Run(t, forkFlow(), execution.Deps{Runner: &executiontest.TraceRunner{}, Events: rec}, executiontest.Batch(`{}`))
	if diff := cmp.Diff(res.Events, rec.Events()); diff != "" {
		t.Errorf("sink events differ from the result trace (-result +sink):\n%s", diff)
	}
}

func TestEventSinkErrorAbortsTheRun(t *testing.T) {
	t.Parallel()
	rec := &executiontest.Recorder{FailAt: 3, Err: executiontest.ErrBoom}
	plan := executiontest.MustCompile(t, linearFlow())
	_, err := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}, Events: rec}).
		Run(t.Context(), plan, execution.Request{Execution: "x", Input: executiontest.Batch(`{}`)})
	if !errors.Is(err, executiontest.ErrBoom) {
		t.Errorf("Run() error = %v, want the sink error", err)
	}
}

// brokenPayloads fails the operations selected by its fields.
type brokenPayloads struct {
	*execution.MemoryPayloads
	failPut, failGet bool
}

func (b brokenPayloads) Put(ctx context.Context, ref execution.PayloadRef, batch execution.Batch) error {
	if b.failPut {
		return executiontest.ErrBoom
	}
	return b.MemoryPayloads.Put(ctx, ref, batch)
}

func (b brokenPayloads) Get(ctx context.Context, ref execution.PayloadRef) (execution.Batch, error) {
	if b.failGet {
		return execution.Batch{}, executiontest.ErrBoom
	}
	return b.MemoryPayloads.Get(ctx, ref)
}

func TestPayloadStoreErrorsAbortTheRun(t *testing.T) {
	t.Parallel()
	plan := executiontest.MustCompile(t, linearFlow())
	tests := []struct {
		name   string
		stored brokenPayloads
	}{
		{"writing fails", brokenPayloads{MemoryPayloads: execution.NewMemoryPayloads(), failPut: true}},
		{"reading fails", brokenPayloads{MemoryPayloads: execution.NewMemoryPayloads(), failGet: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}, Payloads: tt.stored})
			_, err := in.Run(t.Context(), plan, execution.Request{Execution: "x", Input: executiontest.Batch(`{}`)})
			if !errors.Is(err, executiontest.ErrBoom) {
				t.Errorf("Run() error = %v, want the store error", err)
			}
		})
	}
}

func TestTokensReferencePayloadsInTheStore(t *testing.T) {
	t.Parallel()
	store := execution.NewMemoryPayloads()
	b := executiontest.NewFlow("share").Start("s").Parallel("p").End("ea").End("eb").Chain("s", "p").Edge("p", "ea").Edge("p", "eb")
	res := executiontest.Run(t, b, execution.Deps{Payloads: store}, executiontest.Batch(`{"id":1}`))
	if store.Len() == 0 {
		t.Fatal("the payload store is empty after a run")
	}
	for _, tok := range res.Tokens {
		if _, err := store.Get(t.Context(), tok.Payload); err != nil {
			t.Errorf("payload %s of token %s: %v", tok.Payload, tok.ID, err)
		}
	}
	// The fork hands the same payload reference to both children: tokens carry
	// references to immutable batches, never copies.
	children := map[execution.PayloadRef]int{}
	for _, tok := range res.Tokens {
		children[tok.Payload]++
	}
	shared := 0
	for _, n := range children {
		if n > 1 {
			shared++
		}
	}
	if shared == 0 {
		t.Errorf("no payload reference is shared between tokens: %v", children)
	}
}

func TestSameFlowAndInputGiveTheSameTrace(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("determinism").
		Start("s").Parallel("p").Task("a").Task("b").Task("c").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinActivated).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("p", "c").
		Edge("a", "j").Edge("b", "j").Edge("c", "j").Chain("j", "after", "e")
	run := func(id execution.ID) execution.Result {
		plan := executiontest.MustCompile(t, b)
		res, err := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}}).
			Run(t.Context(), plan, execution.Request{Execution: id, Input: executiontest.Batch(`{"id":1}`)})
		if err != nil {
			t.Fatalf("Run() = %v", err)
		}
		return res
	}
	first, second := run("exec-1"), run("exec-1")
	if diff := cmp.Diff(first.Events, second.Events); diff != "" {
		t.Errorf("event traces differ between identical runs (-first +second):\n%s", diff)
	}
	if diff := cmp.Diff(first.Tokens, second.Tokens); diff != "" {
		t.Errorf("tokens differ between identical runs (-first +second):\n%s", diff)
	}
	if diff := cmp.Diff(first.NodeExecutions, second.NodeExecutions); diff != "" {
		t.Errorf("node executions differ between identical runs (-first +second):\n%s", diff)
	}
	other := run("exec-2")
	if first.Tokens[0].ID == other.Tokens[0].ID {
		t.Error("two executions share a root token id; ids must depend on the execution id")
	}
	if diff := cmp.Diff(executiontest.EventTypes(first), executiontest.EventTypes(other)); diff != "" {
		t.Errorf("the shape of the trace depends on the execution id (-first +other):\n%s", diff)
	}
}
