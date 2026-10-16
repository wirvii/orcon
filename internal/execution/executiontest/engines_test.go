// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest_test

import (
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

func forkJoinFlow() *executiontest.Builder {
	return executiontest.NewFlow("fork join").Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
}

// TestSessionEngineMatchesMemoryEngine: the session driver over the fake database
// gives the run of the in-memory interpreter, which Diff states.
func TestSessionEngineMatchesMemoryEngine(t *testing.T) {
	b := forkJoinFlow()
	doc := b.Doc()
	plan := executiontest.MustCompile(t, b)
	input := executiontest.Batch(`{"n":1}`)
	deps := func() execution.Deps { return execution.Deps{Runner: &executiontest.TraceRunner{}} }
	want := executiontest.MemoryEngine(t, &doc, plan, deps(), input)
	got := executiontest.SessionEngine(t, &doc, plan, deps(), input)
	if d := executiontest.Diff(want, got); d != "" {
		t.Fatalf("the engines differ:\n%s", d)
	}
	if want.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s", want.Status)
	}
}

func TestDiffDescribesDifferences(t *testing.T) {
	b := forkJoinFlow()
	doc := b.Doc()
	plan := executiontest.MustCompile(t, b)
	run := func(runner execution.NodeRunner, items string) execution.Result {
		return executiontest.MemoryEngine(t, &doc, plan, execution.Deps{Runner: runner}, executiontest.Batch(items))
	}
	base := run(&executiontest.TraceRunner{}, `{"n":1}`)
	if d := executiontest.Diff(base, base); d != "" {
		t.Fatalf("a result differs from itself: %s", d)
	}
	silent := &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){
		"a": func(execution.NodeCall) (execution.NodeOutput, error) { return execution.NodeOutput{}, nil },
	}}
	other := run(silent, `{"n":1}`)
	if d := executiontest.Diff(base, other); !strings.Contains(d, "events differ") {
		t.Errorf("Diff of a run whose branch emitted nothing = %q, want the events to differ", d)
	}
	failed := base
	failed.Status = execution.ExecFailed
	failed.Failure = &execution.Failure{Code: execution.CodeNoPathTaken, Node: "a"}
	d := executiontest.Diff(base, failed)
	for _, want := range []string{"status", "failure"} {
		if !strings.Contains(d, want) {
			t.Errorf("Diff of a failed run = %q, want it to name the %s", d, want)
		}
	}
	trimmed := base
	trimmed.Events = base.Events[:len(base.Events)-1]
	trimmed.Tokens = base.Tokens[:1]
	trimmed.NodeExecutions = nil
	for _, want := range []string{"events differ", "tokens differ", "node executions differ"} {
		if d := executiontest.Diff(base, trimmed); !strings.Contains(d, want) {
			t.Errorf("Diff of a shorter run = %q, want %q", d, want)
		}
	}
}

func TestUseEngineSwitchesAndRestores(t *testing.T) {
	b := forkJoinFlow()
	called := 0
	restore := executiontest.UseEngine(func(tb testing.TB, doc *flow.Document, plan *execution.Plan, deps execution.Deps, input execution.Batch) execution.Result {
		tb.Helper()
		called++
		return executiontest.MemoryEngine(tb, doc, plan, deps, input)
	})
	executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{"n":1}`))
	restore()
	executiontest.Run(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, executiontest.Batch(`{"n":1}`))
	if called != 1 {
		t.Errorf("the engine in use was called %d times, want once (the second run is back on the previous engine)", called)
	}
}

func TestFlagsTakeTheirDefaults(t *testing.T) {
	if got := executiontest.StoreFlag("auto"); got != "auto" {
		t.Errorf("StoreFlag(auto) = %q without -store", got)
	}
	if got := executiontest.Iterations(7); got != 7 {
		t.Errorf("Iterations(7) = %d without -iterations", got)
	}
	if got := executiontest.JoinRepeats(9); got != 9 {
		t.Errorf("JoinRepeats(9) = %d without -join-repeats", got)
	}
}

func TestMaskEntriesRemovesEntryTokens(t *testing.T) {
	scope := "L:loop@01234567-89ab-cdef-0123-456789abcdef#2/L:inner@fedcba98-7654-3210-fedc-ba9876543210#0"
	if got, want := executiontest.MaskEntries(scope), "L:loop#2/L:inner#0"; got != want {
		t.Errorf("MaskEntries(%q) = %q, want %q", scope, got, want)
	}
}
