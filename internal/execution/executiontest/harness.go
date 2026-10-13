// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// MustCompile compiles the flow and fails the test when it does not compile.
func MustCompile(tb testing.TB, b *Builder) *execution.Plan {
	tb.Helper()
	doc := b.Doc()
	plan, err := execution.Compile(&doc)
	if err != nil {
		tb.Fatalf("Compile() = %v, want a plan", err)
	}
	return plan
}

// Run compiles the flow, runs it once with the given ports and fails the test
// on a run error. A failed execution is not a run error: inspect the result.
func Run(tb testing.TB, b *Builder, deps execution.Deps, input execution.Batch) execution.Result {
	tb.Helper()
	plan := MustCompile(tb, b)
	res, err := execution.NewInterpreter(deps).Run(tb.Context(), plan, execution.Request{Execution: "exec-1", Input: input})
	if err != nil {
		tb.Fatalf("Run() = %v, want no error", err)
	}
	for _, v := range Violations(res) {
		tb.Errorf("run invariant broken: %s", v)
	}
	return res
}

// Paths renders, for every token that reached an end event, the path of every
// item as "a>b>c", prefixed by the end node: "end:a>b>c". The result is in
// arrival order.
func Paths(res execution.Result) []string {
	var out []string
	for _, end := range res.Ends {
		for _, it := range end.Batch.Items() {
			out = append(out, "end:"+pathOf(it))
		}
	}
	return out
}

func pathOf(it execution.Item) string {
	v, ok := it.JSON.Get("path")
	if !ok {
		return ""
	}
	parts := make([]string, 0, v.Len())
	for _, p := range v.Items() {
		s, _ := p.Str()
		parts = append(parts, s)
	}
	return strings.Join(parts, ">")
}

// Executed returns how many times the node ran, counting node executions.
func Executed(res execution.Result, node string) int {
	n := 0
	for _, ne := range res.NodeExecutions {
		if ne.Node == flow.NodeID(node) {
			n++
		}
	}
	return n
}

// EventTypes returns the types of the events in order.
func EventTypes(res execution.Result) []execution.EventType {
	out := make([]execution.EventType, len(res.Events))
	for i := range res.Events {
		out[i] = res.Events[i].Type
	}
	return out
}

// Count returns how many events have the given type.
func Count(res execution.Result, typ execution.EventType) int {
	return len(slices.DeleteFunc(EventTypes(res), func(t execution.EventType) bool { return t != typ }))
}

// TokenStates counts the final states of the tokens of a run.
func TokenStates(res execution.Result) map[execution.TokenState]int {
	out := make(map[execution.TokenState]int)
	for _, t := range res.Tokens {
		out[t.State]++
	}
	return out
}

// Trace renders the whole event trace, one line per event.
func Trace(res execution.Result) string {
	lines := make([]string, len(res.Events))
	for i := range res.Events {
		lines[i] = res.Events[i].String()
	}
	return strings.Join(lines, "\n")
}

// Violations checks the invariants every finished run must satisfy, whatever
// the flow, and describes each one that does not hold:
//
//   - events are numbered 1..n in order;
//   - exactly one terminal execution event, and no token still active;
//   - every edge carries exactly one completion signal per scope (ADR-0010
//     section 2);
//   - every node execution has a unique id and ends in a final state;
//   - the node.started and the terminal node events match one to one.
func Violations(res execution.Result) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }
	closed := make(map[string]bool)
	started, ended := 0, 0
	terminal := 0
	for i := range res.Events {
		e := &res.Events[i]
		if e.Sequence != i+1 {
			add("event %d has sequence %d", i+1, e.Sequence)
		}
		switch e.Type {
		case execution.EventEdgeClosed:
			key := string(e.Edge) + "@" + string(e.Scope)
			if closed[key] {
				add("edge %s closed twice in scope %q", e.Edge, e.Scope)
			}
			closed[key] = true
		case execution.EventNodeStarted:
			started++
		case execution.EventNodeSucceeded, execution.EventNodeFailed:
			ended++
		case execution.EventExecutionSucceeded, execution.EventExecutionFailed:
			terminal++
		case execution.EventExecutionStarted, execution.EventTokenForked, execution.EventTokenDeferred,
			execution.EventTokenConsumed, execution.EventJoinFired, execution.EventJoinFailed,
			execution.EventLoopIterationStarted, execution.EventLoopLimitExceeded:
		}
	}
	if res.Status == execution.ExecSucceeded || res.Status == execution.ExecFailed {
		if terminal != 1 {
			add("%d terminal execution events, want 1", terminal)
		}
		for _, t := range res.Tokens {
			switch t.State {
			case execution.TokenRunnable, execution.TokenLeased, execution.TokenDeferred, execution.TokenSuspended:
				add("token %s ended the run %s", t.ID, t.State)
			case execution.TokenWaiting, execution.TokenConsumed, execution.TokenCompleted,
				execution.TokenCanceled, execution.TokenFailed:
			}
		}
	}
	if started != ended {
		add("%d node.started events but %d node.succeeded or node.failed", started, ended)
	}
	ids := make(map[execution.NodeExecutionID]bool)
	for _, ne := range res.NodeExecutions {
		if ids[ne.ID] {
			add("node execution %s appears twice", ne.ID)
		}
		ids[ne.ID] = true
		if ne.State == execution.NodeRunning {
			add("node execution %s of %s is still RUNNING", ne.ID, ne.Node)
		}
	}
	return out
}

// Field returns the named JSON field of the first item of a batch, rendered as
// canonical JSON, or "" when the batch is empty or the field absent.
func Field(b execution.Batch, field string) string {
	if b.Len() == 0 {
		return ""
	}
	v, ok := b.Item(0).JSON.Get(field)
	if !ok {
		return ""
	}
	raw, err := jsonvalue.Canonical(v)
	if err != nil {
		return ""
	}
	return string(raw)
}
