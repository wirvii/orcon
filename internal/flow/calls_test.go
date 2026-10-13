// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
)

func TestCallEdgesOfADocument(t *testing.T) {
	t.Parallel()
	doc := load(t, "effects-calls-and-settings")
	got := doc.CallEdges()
	want := []flow.CallEdge{
		{From: "effects-calls-and-settings", To: "ship-order"},
		{From: "effects-calls-and-settings", To: "partner-a"},
		{From: "effects-calls-and-settings", To: "partner-b"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("CallEdges() = %v, want %v", got, want)
	}
	linear := load(t, "linear")
	if edges := linear.CallEdges(); len(edges) != 0 {
		t.Errorf("a flow with no calls has edges %v", edges)
	}
}

func TestCheckCallGraph(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		edges    []flow.CallEdge
		wantErrs int
		wantPath string // fragment of the first message
	}{
		{"no calls", nil, 0, ""},
		{"acyclic", []flow.CallEdge{{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "a", To: "c"}}, 0, ""},
		{"direct cycle", []flow.CallEdge{{From: "a", To: "b"}, {From: "b", To: "a"}}, 2, "a -> b -> a"},
		{"self call", []flow.CallEdge{{From: "a", To: "a"}}, 1, "a -> a"},
		{"indirect cycle of three", []flow.CallEdge{{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "a"}}, 3, "-> a"},
		{"cycle closed by the candidate", []flow.CallEdge{{From: "b", To: "c"}, {From: "c", To: "d"}, {From: "d", To: "b"}, {From: "a", To: "b"}}, 3, ""},
		{"intentional recursion is allowed", []flow.CallEdge{{From: "a", To: "a", Intentional: true}}, 0, ""},
		{"cycle of intentional edges is allowed", []flow.CallEdge{{From: "a", To: "b", Intentional: true}, {From: "b", To: "a", Intentional: true}}, 0, ""},
		{"an unmarked edge on a mixed cycle blocks", []flow.CallEdge{{From: "a", To: "b", Intentional: true}, {From: "b", To: "a"}}, 1, "b -> a -> b"},
		{"tail into a cycle is not part of it", []flow.CallEdge{{From: "x", To: "a"}, {From: "a", To: "b"}, {From: "b", To: "a"}}, 2, ""},
		{"duplicate edges are reported once", []flow.CallEdge{{From: "a", To: "a"}, {From: "a", To: "a"}}, 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := flow.CheckCallGraph(tt.edges)
			errs := r.Errors()
			if len(errs) != tt.wantErrs {
				t.Fatalf("CheckCallGraph = %d errors %v, want %d", len(errs), errs, tt.wantErrs)
			}
			for _, e := range errs {
				if e.Code != flow.CodeV11 {
					t.Errorf("code = %s, want V11", e.Code)
				}
			}
			if tt.wantPath != "" && !strings.Contains(errs[0].Message, tt.wantPath) {
				t.Errorf("message %q does not contain the path %q", errs[0].Message, tt.wantPath)
			}
		})
	}
}
