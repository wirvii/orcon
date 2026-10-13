// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// letFinishPlan compiles start -> p -> {a, b} -> j (FIRST, LET_FINISH) -> end.
func letFinishPlan(t *testing.T) *Plan {
	t.Helper()
	params, err := jsonvalue.Object()
	if err != nil {
		t.Fatal(err)
	}
	node := func(id string, el flow.Element, typ string) flow.Node {
		return flow.Node{ID: flow.NodeID(id), Name: id, Element: el, Type: typ, Parameters: params}
	}
	j := node("j", flow.ElementParallelGateway, "orcon.merge@1")
	j.Join = &flow.Join{Mode: flow.JoinFirst, Data: flow.JoinDataAppend, Remaining: flow.RemainingLetFinish}
	edge := func(id, from, to string) flow.Edge {
		return flow.Edge{ID: flow.EdgeID(id), From: flow.Endpoint{Node: flow.NodeID(from), Port: flow.PortMain}, To: flow.NodeID(to), Kind: flow.EdgeSequence}
	}
	doc := flow.Document{
		Format: flow.FormatTag, ID: "01930000-0000-7000-8000-0000000000dd", Name: "rounds",
		Nodes: []flow.Node{
			node("start", flow.ElementStartEvent, "orcon.start@1"), node("p", flow.ElementParallelGateway, "orcon.parallel@1"),
			node("a", flow.ElementTask, "orcon.task@1"), node("b", flow.ElementTask, "orcon.task@1"), j,
			node("end", flow.ElementEndEvent, "orcon.end@1"),
		},
		Edges: []flow.Edge{edge("e1", "start", "p"), edge("e2", "p", "a"), edge("e3", "p", "b"), edge("e4", "a", "j"), edge("e5", "b", "j"), edge("e6", "j", "end")},
	}
	plan, err := Compile(&doc)
	if err != nil {
		t.Fatalf("Compile() = %v", err)
	}
	return plan
}

// TestBlockedRoundDoesNotFireInACanceledRegion pins the case where a LET_FINISH
// round waits behind the one before it and an outer FIRST/CANCEL join cancels
// the region the round belongs to: when the earlier round resets, the blocked
// round must not fire.
func TestBlockedRoundDoesNotFireInACanceledRegion(t *testing.T) {
	// The subtests share one run, so they run in order.
	plan := letFinishPlan(t)
	r := newRun(NewInterpreter(Deps{}), plan, Request{Execution: "x"})
	jIdx := plan.index["j"]
	key := nodeKey{node: jIdx}
	n := &plan.nodes[jIdx]
	js := r.joinOf(key)

	tests := []struct {
		name         string
		canceled     bool
		wantContinue bool
	}{
		{"a live region fires", false, true},
		{"a canceled region does not fire", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := PayloadRef("batch-" + tt.name)
			if err := r.payloads.Put(t.Context(), ref, NewBatch()); err != nil {
				t.Fatal(err)
			}
			outer := ForkRef{SplitNodeExecution: NodeExecutionID("outer-" + tt.name), SplitNode: "outer", Branch: 0}
			own := ForkRef{SplitNodeExecution: NodeExecutionID("own-" + tt.name), SplitNode: "p", Branch: 0}
			rd, _ := js.round(own.SplitNodeExecution)
			rd.arrivals = append(rd.arrivals, joinArrival{Slot: 0, Token: TokenID("tok-" + tt.name), Payload: ref, Lineage: []ForkRef{outer, own}})
			if tt.canceled {
				r.canceledSplits[outer.SplitNodeExecution] = true
			}
			queued := len(r.queue)
			if err := r.fireRound(t.Context(), key, n, rd); err != nil {
				t.Fatalf("fireRound() = %v", err)
			}
			if got := len(r.queue) - queued; (got == 1) != tt.wantContinue {
				t.Errorf("continuations queued = %d, want one: %v", got, tt.wantContinue)
			}
			if !rd.fired {
				t.Error("the round is not marked fired; the rounds after it would stay blocked")
			}
			if !tt.wantContinue && !rd.reset {
				t.Error("a round of a canceled region must be reset, so it does not block the next round")
			}
		})
	}
}

// TestIterationStaysOpenForAWaitingTokenInTheBody pins that a token that is
// WAITING in the body of a loop iteration keeps the iteration open, so the
// merged return does not start the next iteration (waits arrive with cut C8;
// the token is put in the state directly). It is unlike the SEQUENTIAL
// counters, which count WAITING as done.
func TestIterationStaysOpenForAWaitingTokenInTheBody(t *testing.T) {
	t.Parallel()
	entry := TokenID("0aa1b2c3-d4e5-4f60-8718-293a4b5c6d7e")
	iteration := ScopePath("").Push(LoopFrame("L", entry, 0))
	r := newRun(NewInterpreter(Deps{}), &Plan{}, Request{Execution: "x"})
	r.returns[iteration] = &loopReturns{}

	tok := r.spawn("body-token", TokenRunnable, 0, -1, iteration, nil, "")
	if err := r.setToken(tok, TokenLeased); err != nil {
		t.Fatal(err)
	}
	if err := r.setToken(tok, TokenWaiting); err != nil {
		t.Fatal(err)
	}
	if got := r.scopeHeld[iteration]; got != 1 {
		t.Fatalf("scope holders = %d with a WAITING token in the body, want 1", got)
	}
	if len(r.dirtyIters) != 0 {
		t.Fatalf("the iteration was marked due while a token waits in it: %v", r.dirtyIters)
	}
	if err := r.startNextIteration(t.Context(), iteration); err != nil {
		t.Fatal(err)
	}
	if r.returns[iteration].started || len(r.queue) != 0 {
		t.Fatal("the next iteration started while a token waits in the current one")
	}
	// The wait resolves, the token runs again and ends: now the iteration is over.
	if err := r.setToken(tok, TokenRunnable); err != nil {
		t.Fatal(err)
	}
	if err := r.setToken(tok, TokenCanceled); err != nil {
		t.Fatal(err)
	}
	if r.scopeHeld[iteration] != 0 || len(r.dirtyIters) != 1 {
		t.Errorf("after the token ended: holders %d, due %v, want 0 and one", r.scopeHeld[iteration], r.dirtyIters)
	}
}
