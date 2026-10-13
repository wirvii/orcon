// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestRuleV14JoinOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		flow   string
		mutate func(*flow.Document)
		// wantPaths are the JSON Pointers of the V14 errors; none means valid.
		wantPaths []string
	}{
		{"LET_FINISH with a corresponding split is valid", "discriminator", func(*flow.Document) {}, nil},
		{"CANCEL explicit is valid", "discriminator", func(d *flow.Document) { node(d, "first").Join.Remaining = flow.RemainingCancel }, nil},
		{"remaining absent is valid", "discriminator", func(d *flow.Document) { node(d, "first").Join.Remaining = "" }, nil},
		{
			"remaining on an ALL join", "parallel-join",
			func(d *flow.Document) { node(d, "join").Join.Remaining = flow.RemainingCancel },
			[]string{"/nodes/4/join/remaining"},
		},
		{
			"remaining on an ACTIVATED join", "exclusive-and-inclusive",
			func(d *flow.Document) { node(d, "after").Join.Remaining = flow.RemainingLetFinish },
			[]string{"/nodes/8/join/remaining"},
		},
		{
			"LET_FINISH without a split that dominates the join", "discriminator",
			func(d *flow.Document) {
				// A second start event feeds the join: no node dominates it.
				d.Nodes = append(d.Nodes, flow.Node{
					ID: "other", Name: "Other start", Element: flow.ElementStartEvent, Type: "orcon.start@1",
					Parameters: node(d, "start").Parameters,
				})
				d.Edges = append(d.Edges, flow.Edge{ID: "e20", From: flow.Endpoint{Node: "other", Port: "main"}, To: "first", Kind: flow.EdgeSequence})
			},
			[]string{"/nodes/5/join/remaining"},
		},
		{
			"MATCH_FIELD with its field", "discriminator",
			func(d *flow.Document) {
				node(d, "first").Join.Data = flow.JoinDataMatchField
				node(d, "first").Join.MatchField = "id"
			},
			[]string{"/nodes/5/join/data", "/nodes/5/join/matchField"},
		},
		{
			"MATCH_POSITION", "discriminator",
			func(d *flow.Document) { node(d, "first").Join.Data = flow.JoinDataMatchPosition },
			[]string{"/nodes/5/join/data"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, tt.flow)
			tt.mutate(&doc)
			var got []string
			for _, issue := range flow.Validate(&doc).Issues {
				if issue.Code == flow.CodeV14 && issue.Severity == flow.SeverityError {
					got = append(got, issue.Path)
				}
			}
			slices.Sort(got)
			if diff := cmp.Diff(tt.wantPaths, got); diff != "" {
				t.Errorf("V14 error paths mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCorrespondingSplitsIsTheNearestDominatingSplit(t *testing.T) {
	t.Parallel()
	doc := load(t, "discriminator")
	got := flow.CorrespondingSplits(&doc)
	want := map[flow.NodeID]flow.NodeID{"first": "split", "canceling": "split"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CorrespondingSplits mismatch (-want +got):\n%s", diff)
	}
	// Joins of every mode have one when a split dominates them.
	pj := load(t, "parallel-join")
	if diff := cmp.Diff(map[flow.NodeID]flow.NodeID{"join": "split"}, flow.CorrespondingSplits(&pj)); diff != "" {
		t.Errorf("CorrespondingSplits(parallel-join) mismatch (-want +got):\n%s", diff)
	}
}

func TestExplicitCancelSurvivesTheRoundTripAndChangesTheHash(t *testing.T) {
	t.Parallel()
	explicit := load(t, "discriminator")
	absent := load(t, "discriminator")
	node(&absent, "canceling").Join.Remaining = ""
	back, err := flow.FromValue(explicit.Value())
	if err != nil {
		t.Fatalf("FromValue() = %v", err)
	}
	if got := node(&back, "canceling").Join.Remaining; got != flow.RemainingCancel {
		t.Errorf("remaining after the round trip = %q, want CANCEL kept as written", got)
	}
	if v, ok := absent.Value().Get("nodes"); ok {
		for _, n := range v.Items() {
			if id, _ := n.Get("id"); id.Kind() == jsonvalue.KindString {
				if s, _ := id.Str(); s == "canceling" {
					j, _ := n.Get("join")
					if _, has := j.Get("remaining"); has {
						t.Error("an absent remaining was written out")
					}
				}
			}
		}
	}
	h1, err1 := flow.ContentHash(&explicit)
	h2, err2 := flow.ContentHash(&absent)
	if err1 != nil || err2 != nil || h1 == h2 {
		t.Errorf("hashes %v %v (errors %v %v), want different hashes for explicit and absent CANCEL", h1, h2, err1, err2)
	}
}
