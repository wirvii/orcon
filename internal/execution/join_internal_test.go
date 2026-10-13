// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/wirvii/orcon/internal/flow"
)

// step is one event applied to a join in a table test.
type joinStep struct {
	// send registers a live token delivered on the slot before anything else.
	send []int
	// arrive absorbs a token from the slot.
	arrive *int
	// signal completes the slot's edge.
	signal *int
	want   joinOutcome
}

func slot(n int) *int { return &n }

func TestJoinRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		mode  flow.JoinMode
		edges int
		steps []joinStep
	}{
		{
			name: "ALL fires when every edge completes with a live token", mode: flow.JoinAll, edges: 2,
			steps: []joinStep{
				{send: []int{0, 1}},
				{arrive: slot(0), want: joinWaiting},
				{signal: slot(0), want: joinWaiting},
				{arrive: slot(1), want: joinWaiting},
				{signal: slot(1), want: joinFires},
			},
		},
		{
			name: "ALL waits for a token still on its way even after the signal", mode: flow.JoinAll, edges: 2,
			steps: []joinStep{
				{send: []int{0, 1}},
				{signal: slot(0), want: joinWaiting},
				{signal: slot(1), want: joinWaiting},
				{arrive: slot(0), want: joinWaiting},
				{arrive: slot(1), want: joinFires},
			},
		},
		{
			name: "ALL fails as soon as one edge completes with no live token", mode: flow.JoinAll, edges: 3,
			steps: []joinStep{
				{send: []int{0}},
				{signal: slot(1), want: joinFails},
			},
		},
		{
			name: "ALL fails when a live token arrives after an edge completed dead", mode: flow.JoinAll, edges: 2,
			steps: []joinStep{
				{signal: slot(0), want: joinWaiting},
				{send: []int{1}},
				{arrive: slot(1), want: joinFails},
			},
		},
		{
			name: "ALL on a dead path completes empty", mode: flow.JoinAll, edges: 2,
			steps: []joinStep{
				{signal: slot(0), want: joinWaiting},
				{signal: slot(1), want: joinEmpty},
			},
		},
		{
			name: "ALL fires once and stays settled", mode: flow.JoinAll, edges: 1,
			steps: []joinStep{
				{send: []int{0, 0}},
				{arrive: slot(0), want: joinWaiting},
				{signal: slot(0), want: joinWaiting},
				{arrive: slot(0), want: joinFires},
				{signal: slot(0), want: joinWaiting},
			},
		},
		{
			name: "ACTIVATED fires when all edges completed and one was live", mode: flow.JoinActivated, edges: 3,
			steps: []joinStep{
				{send: []int{1}},
				{signal: slot(0), want: joinWaiting},
				{arrive: slot(1), want: joinWaiting},
				{signal: slot(1), want: joinWaiting},
				{signal: slot(2), want: joinFires},
			},
		},
		{
			name: "ACTIVATED completes empty when no edge was live", mode: flow.JoinActivated, edges: 2,
			steps: []joinStep{
				{signal: slot(0), want: joinWaiting},
				{signal: slot(1), want: joinEmpty},
			},
		},
		{
			name: "FIRST fires on the first live arrival", mode: flow.JoinFirst, edges: 3,
			steps: []joinStep{
				{send: []int{1, 2}},
				{arrive: slot(2), want: joinFires},
				{arrive: slot(1), want: joinWaiting},
				{signal: slot(0), want: joinWaiting},
			},
		},
		{
			name: "FIRST completes empty when every edge completes without a token", mode: flow.JoinFirst, edges: 2,
			steps: []joinStep{
				{signal: slot(0), want: joinWaiting},
				{signal: slot(1), want: joinEmpty},
			},
		},
		{
			name: "an edge with a token still on its way is not complete", mode: flow.JoinActivated, edges: 1,
			steps: []joinStep{
				{send: []int{0}},
				{signal: slot(0), want: joinWaiting},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			js := newJoinState(&joinSpec{mode: tt.mode}, tt.edges)
			for i, st := range tt.steps {
				for _, s := range st.send {
					js.send(s)
				}
				var got joinOutcome
				switch {
				case st.arrive != nil:
					got = js.arrive(joinArrival{Slot: *st.arrive, Token: TokenID("t")})
				case st.signal != nil:
					got = js.signal(*st.signal)
				default:
					continue
				}
				if got != st.want {
					t.Fatalf("step %d: outcome = %d, want %d", i, got, st.want)
				}
			}
		})
	}
}

func TestJoinCombinesByEdgeThenArrival(t *testing.T) {
	t.Parallel()
	js := newJoinState(&joinSpec{mode: flow.JoinAll}, 2)
	js.send(0)
	js.send(0)
	js.send(1)
	js.arrive(joinArrival{Slot: 1, Token: "b1"})
	js.arrive(joinArrival{Slot: 0, Token: "a1"})
	js.arrive(joinArrival{Slot: 0, Token: "a2"})
	var got []TokenID
	for _, a := range js.combined() {
		got = append(got, a.Token)
	}
	if diff := cmp.Diff([]TokenID{"a1", "a2", "b1"}, got); diff != "" {
		t.Errorf("combined order mismatch (-want +got):\n%s", diff)
	}
}

func TestContinuationLineage(t *testing.T) {
	t.Parallel()
	fork := func(split string, branch int) ForkRef {
		return ForkRef{SplitNodeExecution: NodeExecutionID("ne-" + split), SplitNode: flow.NodeID(split), Branch: branch}
	}
	arrival := func(lineage ...ForkRef) joinArrival { return joinArrival{Lineage: lineage} }
	tests := []struct {
		name     string
		arrivals []joinArrival
		split    flow.NodeID
		want     []ForkRef
	}{
		{"no arrival", nil, "p", nil},
		{
			"branches of the corresponding split converge",
			[]joinArrival{arrival(fork("p", 0)), arrival(fork("p", 1))}, "p", nil,
		},
		{"a single arrival drops the corresponding split too", []joinArrival{arrival(fork("p", 0))}, "p", nil},
		{
			"the outer split stays",
			[]joinArrival{arrival(fork("o", 0), fork("q", 0)), arrival(fork("o", 0), fork("q", 1))}, "q", []ForkRef{fork("o", 0)},
		},
		{
			// The branches differ on the inner split only: the outer split is
			// found on the full lineages and kept in the prefix.
			"a split that is not the corresponding one stays",
			[]joinArrival{arrival(fork("o", 0), fork("q", 0)), arrival(fork("o", 0), fork("q", 1))}, "other", []ForkRef{fork("o", 0)},
		},
		{"no corresponding split", []joinArrival{arrival(fork("p", 0))}, "", []ForkRef{fork("p", 0)}},
		{
			"the cut also removes what comes after the split",
			[]joinArrival{arrival(fork("p", 0), fork("x", 1))}, "p", nil,
		},
		{
			// The token never forked at the corresponding split (an exclusive
			// choice sent it down one branch): nothing to drop.
			"the lineage has no entry of the split",
			[]joinArrival{arrival(fork("o", 0))}, "q", []ForkRef{fork("o", 0)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := continuationLineage(tt.arrivals, tt.split)
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("lineage mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeterministicIDsFollowTheADRFormulas(t *testing.T) {
	t.Parallel()
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	ids := map[string]string{
		"root":    string(rootTokenID("x1")),
		"fork":    string(forkTokenID("t", "split", "L:a#0", 2)),
		"join":    string(joinTokenID("x1", "j", "")),
		"node":    string(nodeExecutionID("x1", "t", "n", "")),
		"output":  string(outputRef("ne", "main")),
		"input":   string(inputRef("x1")),
		"joined":  string(joinedRef("t")),
		"another": string(rootTokenID("x2")),
	}
	seen := map[string]string{}
	for name, id := range ids {
		if !uuid.MatchString(id) {
			t.Errorf("%s id %q is not UUID text", name, id)
		}
		if other, dup := seen[id]; dup {
			t.Errorf("%s and %s share the id %s", name, other, id)
		}
		seen[id] = name
	}
	if rootTokenID("x1") != TokenID(hashID("x1", "root")) {
		t.Error("root token id is not H(executionId, \"root\")")
	}
	if forkTokenID("t", "split", "L:a#0", 2) != TokenID(hashID("t", "split", "L:a#0", "2")) {
		t.Error("fork token id is not H(parentTokenId, splitNodeId, scopePath, branchIndex)")
	}
	if joinTokenID("x1", "j", "s") != TokenID(hashID("x1", "j", "s")) {
		t.Error("join token id is not H(executionId, joinNodeId, scopePath)")
	}
	if nodeExecutionID("x1", "t", "n", "s") != NodeExecutionID(hashID("x1", "t", "n", "s")) {
		t.Error("node execution id is not H(executionId, tokenId, nodeId, scopePath)")
	}
	// The length prefix keeps field boundaries apart.
	if hashID("ab", "c") == hashID("a", "bc") {
		t.Error("hashID(\"ab\",\"c\") equals hashID(\"a\",\"bc\"): fields are not length-prefixed")
	}
}
