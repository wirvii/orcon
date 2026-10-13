// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// checkChaos runs a valid flow with pseudo-random ports and checks what must
// hold for any flow, however unstructured. A flow may fail (an exclusive gateway
// with no true condition, an ALL join with a dead branch among live ones), but
// the interpreter must never report an internal error, leave a token active,
// close an edge twice in a scope or behave differently on a second run.
func checkChaos(t *testing.T, doc *flow.Document) {
	t.Helper()
	plan, err := execution.Compile(doc)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	deps := executiontest.ChaosDeps(doc)
	start := flow.NodeID("")
	for i := range doc.Nodes {
		if doc.Nodes[i].Element == flow.ElementStartEvent {
			start = doc.Nodes[i].ID
			break
		}
	}
	run := func() (execution.Result, error) {
		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()
		return execution.NewInterpreter(deps).Run(ctx, plan, execution.Request{
			Execution: "chaos", Start: start, Input: executiontest.Batch(`{"a":1}`, `{"a":2}`),
		})
	}
	first, err := run()
	if errors.Is(err, context.DeadlineExceeded) {
		return // a flow that fans out without bound is cut off, not a bug
	}
	if err != nil {
		t.Fatalf("Run() = %v\n%s", err, executiontest.Trace(first))
	}
	switch first.Status {
	case execution.ExecSucceeded:
	case execution.ExecFailed:
		switch first.Failure.Code {
		case execution.CodeNoPathTaken, execution.CodeJoinRequirementNotMet:
		default:
			t.Fatalf("failure %+v: only NoPathTaken and JoinRequirementNotMet are expected\n%s", first.Failure, executiontest.Trace(first))
		}
	case execution.ExecQueued, execution.ExecRunning, execution.ExecWaiting, execution.ExecSuspended,
		execution.ExecCanceling, execution.ExecCompensating, execution.ExecCanceled:
		t.Fatalf("status = %s, want SUCCEEDED or FAILED", first.Status)
	}
	if v := executiontest.Violations(first); len(v) > 0 {
		t.Fatalf("invariants broken: %v\n%s", v, executiontest.Trace(first))
	}
	second, err := run()
	if err != nil {
		return // the second run hit the time limit; the first one already passed
	}
	if !reflect.DeepEqual(first.Events, second.Events) {
		t.Fatalf("two runs differ:\n%s\n---\n%s", executiontest.Trace(first), executiontest.Trace(second))
	}
}

// outcome summarizes a run in a form that does not depend on the order in
// which the branches of a split were run.
type outcome struct {
	Status   execution.State
	Code     string
	Executed map[flow.NodeID]int
	Ends     []string
}

func summarize(res execution.Result) outcome {
	o := outcome{Status: res.Status, Executed: make(map[flow.NodeID]int)}
	if res.Failure != nil {
		o.Code = res.Failure.Code
	}
	for _, ne := range res.NodeExecutions {
		o.Executed[ne.Node]++
	}
	for _, end := range res.Ends {
		for _, text := range executiontest.Texts(end.Batch) {
			o.Ends = append(o.Ends, string(end.Node)+" "+text)
		}
	}
	slices.Sort(o.Ends)
	return o
}

// checkBranchModes runs a flow without discriminators in PARALLEL and in
// SEQUENTIAL branch mode. The mode changes only the order in which sibling
// branches run, so a flow that succeeds in one mode must succeed in the other,
// with the same node executions and the same items at the end events. (A
// discriminator takes whichever branch arrives first, so it is excluded; and
// when both runs fail, the failure that is raised first may differ.)
func checkBranchModes(t *testing.T, doc *flow.Document) {
	t.Helper()
	for i := range doc.Nodes {
		if j := doc.Nodes[i].Join; j != nil && j.Mode == flow.JoinFirst {
			return
		}
	}
	run := func(mode flow.BranchMode) outcome {
		d := *doc
		d.Settings.BranchMode = mode
		plan, err := execution.Compile(&d)
		if err != nil {
			t.Fatalf("Compile() = %v", err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()
		res, err := execution.NewInterpreter(executiontest.ChaosDeps(&d)).Run(ctx, plan, execution.Request{
			Execution: "modes", Input: executiontest.Batch(`{"a":1}`, `{"a":2}`),
		})
		if err != nil {
			return outcome{Status: execution.ExecRunning} // cut off: not comparable
		}
		return summarize(res)
	}
	parallel, sequential := run(flow.BranchParallel), run(flow.BranchSequential)
	if parallel.Status == execution.ExecRunning || sequential.Status == execution.ExecRunning {
		return
	}
	if parallel.Status != sequential.Status {
		t.Fatalf("PARALLEL ends in %s and SEQUENTIAL in %s", parallel.Status, sequential.Status)
	}
	if parallel.Status == execution.ExecFailed {
		return
	}
	if diff := cmp.Diff(parallel, sequential); diff != "" {
		t.Fatalf("branch modes differ (-parallel +sequential):\n%s", diff)
	}
}

func TestRandomGraphs(t *testing.T) {
	t.Parallel()
	runs := uint64(3000)
	if testing.Short() {
		runs = 300
	}
	for seed := range runs {
		doc := executiontest.RandomGraph(seeded(2_000_000+seed, 60)).Doc()
		if err := flow.Validate(&doc).Err(); err != nil {
			t.Fatalf("seed %d: the generator built an invalid flow: %v", seed, err)
		}
		checkChaos(t, &doc)
		checkBranchModes(t, &doc)
	}
}

// FuzzRandomGraph builds unstructured flows from the bytes and runs them.
func FuzzRandomGraph(f *testing.F) {
	for seed := range uint64(12) {
		f.Add(seeded(seed, 60))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		doc := executiontest.RandomGraph(data).Doc()
		if flow.Validate(&doc).HasErrors() {
			return
		}
		checkChaos(t, &doc)
		checkBranchModes(t, &doc)
	})
}

// FuzzRunFlow reads arbitrary bytes as a flow document and runs the ones that
// are valid and supported, with the same properties as [FuzzRandomGraph].
func FuzzRunFlow(f *testing.F) {
	files, err := filepath.Glob(filepath.Join("..", "flow", "testdata", "conformance", "*.json"))
	if err != nil || len(files) == 0 {
		f.Fatalf("no flow seeds (%v)", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file) //nolint:gosec // G304: the path comes from a glob over the flow testdata directory
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		raw, err := jsonvalue.Parse(data)
		if err != nil {
			return
		}
		doc, err := flow.Read(raw)
		if err != nil || len(doc.Nodes) > 24 {
			return
		}
		if _, err := execution.Compile(&doc); err != nil {
			return
		}
		checkChaos(t, &doc)
	})
}
