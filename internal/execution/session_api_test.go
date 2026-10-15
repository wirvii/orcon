// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestBatchDocumentRoundTrip(t *testing.T) {
	t.Parallel()
	big := jsonvalue.String("x")
	num, err := jsonvalue.Number("1234.5")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := jsonvalue.Object(jsonvalue.Member{Key: "n", Value: num}, jsonvalue.Member{Key: "s", Value: big})
	if err != nil {
		t.Fatal(err)
	}
	batch := execution.NewBatch(
		execution.Item{JSON: obj, Lineage: []execution.LineageRef{{Port: "main", Index: 3}, {Port: "other", Index: 0}}},
		execution.Item{JSON: jsonvalue.Null()},
	)
	doc, err := execution.EncodeBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	got, err := execution.DecodeBatch(doc)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(executiontest.Texts(batch), executiontest.Texts(got)); diff != "" {
		t.Errorf("items (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(batch.Item(0).Lineage, got.Item(0).Lineage); diff != "" {
		t.Errorf("lineage (-want +got):\n%s", diff)
	}
	empty, err := execution.EncodeBatch(execution.Batch{})
	if err != nil {
		t.Fatal(err)
	}
	if b, err := execution.DecodeBatch(empty); err != nil || b.Len() != 0 {
		t.Errorf("empty batch = %d items, %v", b.Len(), err)
	}
	for name, bad := range map[string]string{
		"not json": `{`, "not an object": `[]`, "items not an array": `{"items":1}`, "item without json": `{"items":[{}]}`, "item not an object": `{"items":[1]}`,
	} {
		if _, err := execution.DecodeBatch([]byte(bad)); err == nil {
			t.Errorf("DecodeBatch(%s) accepted %s", name, bad)
		}
	}
}

func TestTokenDocumentRoundTrip(t *testing.T) {
	t.Parallel()
	row := execution.TokenRow{
		Lineage: []execution.ForkRef{{SplitNodeExecution: "ne-1", SplitNode: "p", Branch: 2}, {SplitNodeExecution: "ne-2", SplitNode: "q"}},
		Passed:  true, Merged: true,
	}
	doc, err := execution.EncodeTokenDocument(&row)
	if err != nil {
		t.Fatal(err)
	}
	var got execution.TokenRow
	if err := execution.DecodeTokenDocument(doc, &got); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(row, got); diff != "" {
		t.Errorf("token document (-want +got):\n%s", diff)
	}
	for _, bad := range []string{`{`, `[]`, `{"l":1}`, `{"l":[1]}`, `{"p":"yes"}`} {
		if err := execution.DecodeTokenDocument([]byte(bad), &got); err == nil {
			t.Errorf("DecodeTokenDocument accepted %s", bad)
		}
	}
}

// openWith opens a session on the plan of a fork and join, with a corrupted tracker document.
func TestOpenSessionRejectsCorruptTrackers(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("fj").Start("s").Parallel("p").Task("a").Task("b").Parallel("j").End("e").Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "e")
	plan := executiontest.MustCompile(t, b)
	in := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}})
	cases := map[string]string{
		"not json":                `{`,
		"not an object":           `[]`,
		"a node the flow lacks":   `{"nodes":[{"n":"ghost"}]}`,
		"wrong incoming edges":    `{"nodes":[{"n":"j","g":[true]}]}`,
		"a join that is no join":  `{"joins":[{"n":"a"}]}`,
		"wrong join edges":        `{"joins":[{"n":"j","e":[{}]}]}`,
		"unknown loop node":       `{"loops":[{"n":"ghost"}]}`,
		"unknown live edge":       `{"live":[{"e":"ghost"}]}`,
		"inconsistent group":      `{"groups":[{"x":"g","t":["t1"],"k":[],"c":0}]}`,
		"unknown round join":      `{"rounds":[{"x":"r","j":[{"n":"ghost"}]}]}`,
		"unknown return":          `{"returns":[{"n":"ghost"}]}`,
		"unknown return edge":     `{"returns":[{"n":"j","a":[{"e":"ghost"}]}]}`,
		"unknown signal edge":     `{"signals":[{"e":"ghost"}]}`,
		"a string where a number": `{"nodes":[{"n":"s","i":"x"}]}`,
		"a string where a flag":   `{"nodes":[{"n":"s","c":"x"}]}`,
		"a number where a string": `{"nodes":[{"n":5}]}`,
	}
	for name, doc := range cases {
		_, err := in.OpenSession(plan, execution.Snapshot{Execution: "x", Status: execution.ExecRunning, Trackers: []byte(doc)}, execution.SessionOptions{})
		if err == nil {
			t.Errorf("OpenSession accepted %s: %s", name, doc)
		}
	}
	// A snapshot with a terminal token, a token at a node the flow lacks or an edge it lacks is refused too.
	for name, row := range map[string]execution.TokenRow{
		"a terminal token": {ID: "t", State: execution.TokenCompleted, Node: "s"},
		"a missing node":   {ID: "t", State: execution.TokenRunnable, Node: "ghost"},
		"a missing edge":   {ID: "t", State: execution.TokenRunnable, Node: "a", InEdge: "ghost"},
	} {
		_, err := in.OpenSession(plan, execution.Snapshot{Execution: "x", Status: execution.ExecRunning, Tokens: []execution.TokenRow{row}}, execution.SessionOptions{})
		if err == nil {
			t.Errorf("OpenSession accepted %s", name)
		}
	}
	if _, err := in.OpenSession(nil, execution.Snapshot{Execution: "x"}, execution.SessionOptions{}); err == nil {
		t.Error("OpenSession accepted no plan")
	}
	if _, err := in.OpenSession(plan, execution.Snapshot{}, execution.SessionOptions{}); err == nil {
		t.Error("OpenSession accepted no execution")
	}
}

// start runs the first step of a fork and join and returns the plan and the first delta.
func startedFlow(t *testing.T) (*execution.Plan, *execution.MemoryPayloads, *execution.Interpreter) {
	t.Helper()
	sharedPayloads := execution.NewMemoryPayloads()
	b := executiontest.NewFlow("seq").Start("s").Task("a").Task("b").Task("c").End("e").Chain("s", "a", "b", "c", "e")
	return executiontest.MustCompile(t, b), sharedPayloads, execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}, Payloads: sharedPayloads})
}

func TestSessionWorkOptions(t *testing.T) {
	t.Parallel()
	plan, _, _ := startedFlow(t)
	ctx := t.Context()
	newSession := func() (*execution.Session, execution.TokenID) {
		pay := execution.NewMemoryPayloads()
		in := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}, Payloads: pay})
		sess, err := in.NewSession(ctx, plan, execution.Request{Execution: "exec", Input: executiontest.Batch(`{"n":1}`)})
		if err != nil {
			t.Fatal(err)
		}
		d, err := sess.Delta()
		if err != nil || len(d.Tokens) != 1 || len(d.Runnable) != 1 {
			t.Fatalf("first delta = %+v, %v", d, err)
		}
		storePayloads(ctx, t, pay, &d)
		leased := d.Tokens[0]
		leased.State = execution.TokenLeased
		snap := execution.Snapshot{Execution: "exec", Status: execution.ExecQueued, Trackers: d.Trackers, Tokens: []execution.TokenRow{leased}}
		reopened, err := in.OpenSession(plan, snap, execution.SessionOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return reopened, leased.ID
	}

	t.Run("budget ends the segment with the token held", func(t *testing.T) {
		t.Parallel()
		sess, id := newSession()
		res, err := sess.Work(ctx, id, execution.WorkOptions{Budget: 2})
		if err != nil || res.Steps != 2 || res.Held != id || res.Done {
			t.Fatalf("Work = %+v, %v; want two steps with the token held", res, err)
		}
		d, err := sess.Delta()
		if err != nil || d.Held != id || d.State != execution.ExecRunning {
			t.Fatalf("delta = %+v, %v", d, err)
		}
	})
	t.Run("pause is asked after the first step", func(t *testing.T) {
		t.Parallel()
		sess, id := newSession()
		asked := 0
		res, err := sess.Work(ctx, id, execution.WorkOptions{Pause: func() bool { asked++; return true }})
		if err != nil || res.Steps != 1 || asked != 1 || res.Held != id {
			t.Fatalf("Work = %+v, %v (asked %d); want one step", res, err, asked)
		}
	})
	t.Run("release gives the token back", func(t *testing.T) {
		t.Parallel()
		sess, id := newSession()
		res, err := sess.Work(ctx, id, execution.WorkOptions{Budget: 1, Release: true})
		if err != nil || res.Held != "" {
			t.Fatalf("Work = %+v, %v; want no token held", res, err)
		}
		d, err := sess.Delta()
		if err != nil {
			t.Fatal(err)
		}
		var released bool
		for _, c := range d.Changed {
			released = released || (c.Row.ID == id && c.Row.State == execution.TokenRunnable)
		}
		if !released || d.Live != 1 {
			t.Fatalf("delta = %+v; want the token RUNNABLE and the execution alive", d)
		}
	})
	t.Run("a token that is not leased cannot be worked", func(t *testing.T) {
		t.Parallel()
		sess, _ := newSession()
		if _, err := sess.Work(ctx, "nobody", execution.WorkOptions{}); err == nil {
			t.Error("Work accepted a token the session does not hold")
		}
	})
	t.Run("a canceled context stops the work", func(t *testing.T) {
		t.Parallel()
		sess, id := newSession()
		canceled, stop := context.WithCancel(ctx)
		stop()
		if _, err := sess.Work(canceled, id, execution.WorkOptions{}); err == nil {
			t.Error("Work went on with a canceled context")
		}
	})
}

// TestReplayStopsAtTheFirstActivationItDoesNotHold: a session that replays recorded
// activations applies them in order and stops, without calling a port, at the first
// node whose activation was not recorded (bounded recomputation).
func TestReplayStopsAtTheFirstActivationItDoesNotHold(t *testing.T) {
	t.Parallel()
	plan, sharedPayloads, in := startedFlow(t)
	ctx := t.Context()
	runner := &executiontest.TraceRunner{}
	live := execution.NewInterpreter(execution.Deps{Runner: runner, Payloads: sharedPayloads})
	sess, err := live.NewSession(ctx, plan, execution.Request{Execution: "exec", Input: executiontest.Batch(`{"n":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	first, err := sess.Delta()
	if err != nil {
		t.Fatal(err)
	}
	storePayloads(ctx, t, sharedPayloads, &first)
	leased := first.Tokens[0]
	leased.State = execution.TokenLeased
	snap := execution.Snapshot{Execution: "exec", Status: execution.ExecQueued, Trackers: first.Trackers, Tokens: []execution.TokenRow{leased}}
	ran, err := live.OpenSession(plan, snap, execution.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ran.Work(ctx, leased.ID, execution.WorkOptions{}); err != nil {
		t.Fatal(err)
	}
	done, err := ran.Delta()
	if err != nil {
		t.Fatal(err)
	}
	if len(done.Activations) != 3 {
		t.Fatalf("%d activations recorded, want 3 (a, b, c)", len(done.Activations))
	}
	// Replay with two of the three: the third node is where it stops, and no port is called.
	calls := len(runner.Calls())
	oracle := map[execution.NodeExecutionID]*execution.Activation{}
	for _, a := range done.Activations[:2] {
		oracle[a.NodeExecution] = a
	}
	replay, err := in.OpenSession(plan, snap, execution.SessionOptions{Oracle: oracle})
	if err != nil {
		t.Fatal(err)
	}
	res, err := replay.Work(ctx, leased.ID, execution.WorkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Awaiting || res.Held != leased.ID || res.Done {
		t.Fatalf("Work = %+v; want the replay to wait for an activation with the token held", res)
	}
	if got := len(runner.Calls()); got != calls {
		t.Errorf("the replay called the runner %d times", got-calls)
	}
	partial, err := replay.Delta()
	if err != nil {
		t.Fatal(err)
	}
	if len(partial.NodeExecutions) != 3 { // s, a and b ran; c did not
		t.Errorf("%d node executions in the replay, want 3", len(partial.NodeExecutions))
	}
}

// TestSessionUsesTheRecordedEnvironment: an activation whose environment was
// recorded gets that record, and one that was not gets a new one that the delta
// carries to be recorded.
func TestSessionUsesTheRecordedEnvironment(t *testing.T) {
	t.Parallel()
	plan, sharedPayloads, _ := startedFlow(t)
	ctx := t.Context()
	var seen []execution.NodeEnv
	runner := runnerFunc(func(call execution.NodeCall) {
		seen = append(seen, call.Env)
	})
	in := execution.NewInterpreter(execution.Deps{Runner: runner, Payloads: sharedPayloads})
	sess, err := in.NewSession(ctx, plan, execution.Request{Execution: "exec", Input: executiontest.Batch(`{"n":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	first, err := sess.Delta()
	if err != nil {
		t.Fatal(err)
	}
	storePayloads(ctx, t, sharedPayloads, &first)
	leased := first.Tokens[0]
	leased.State = execution.TokenLeased
	snap := execution.Snapshot{Execution: "exec", Status: execution.ExecQueued, Trackers: first.Trackers, Tokens: []execution.TokenRow{leased}}
	// The first activation (node a) has a record; the others do not.
	probe, err := in.OpenSession(plan, snap, execution.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := probe.Work(ctx, leased.ID, execution.WorkOptions{Budget: 2}); err != nil {
		t.Fatal(err)
	}
	pd, err := probe.Delta()
	if err != nil || len(pd.Envs) != 1 {
		t.Fatalf("delta = %+v, %v; want one environment made by the one activation", pd, err)
	}
	known := map[execution.NodeExecutionID]execution.NodeEnv{pd.Envs[0].NodeExecution: {Seed: 4242}}
	seen = nil
	again, err := in.OpenSession(plan, snap, execution.SessionOptions{Known: known})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := again.Work(ctx, leased.ID, execution.WorkOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 || seen[0].Seed != 4242 {
		t.Fatalf("activations saw %+v; want the first to get the recorded seed 4242", seen)
	}
	if seen[1].Seed == 4242 || seen[1].Seed == seen[2].Seed {
		t.Errorf("activations without a record got %+v", seen[1:])
	}
	ad, err := again.Delta()
	if err != nil || len(ad.Envs) != 2 {
		t.Fatalf("delta = %+v, %v; want the two environments that were made, not the recorded one", ad, err)
	}
	if !strings.Contains(seen[0].String(), "/") {
		t.Errorf("String() = %q", seen[0].String())
	}
}

// runnerFunc observes each call and passes the input through the main port.
type runnerFunc func(execution.NodeCall)

func (f runnerFunc) Execute(_ context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	f(call)
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
}

// storePayloads stores the batches a delta asks to store before it is committed.
func storePayloads(ctx context.Context, t *testing.T, pay *execution.MemoryPayloads, d *execution.Delta) {
	t.Helper()
	for _, w := range d.Payloads {
		if err := pay.Put(ctx, w.Ref, w.Batch); err != nil {
			t.Fatal(err)
		}
	}
}
