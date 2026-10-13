// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"slices"
	"testing"
)

func TestScanNodeRefs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		expr string
		want []nodeRef
	}{
		{"single quotes", `$('Fetch').item.json.x`, []nodeRef{{Name: "Fetch"}}},
		{"double quotes", `$("Fetch").first()`, []nodeRef{{Name: "Fetch"}}},
		{"legacy node syntax", `$node["Fetch"].json.x`, []nodeRef{{Name: "Fetch"}}},
		{"legacy node single quotes with spaces", `$node [ 'Fetch' ] . json`, []nodeRef{{Name: "Fetch"}}},
		{"spaces inside the call", `$(  'Fetch'  ).item`, []nodeRef{{Name: "Fetch"}}},
		{"guarded", `$('A').isExecuted ? 1 : 2`, []nodeRef{{Name: "A", Guarded: true}}},
		{"guarded with spaces", `$('A') . isExecuted`, []nodeRef{{Name: "A", Guarded: true}}},
		{"member that only starts like isExecuted", `$('A').isExecutedLater`, []nodeRef{{Name: "A"}}},
		{"escaped quote in a name", `$('It\'s here').item`, []nodeRef{{Name: "It's here"}}},
		{"several references", `$('A').item.x + $("B").item.y + $('A').item.z`, []nodeRef{{Name: "A"}, {Name: "B"}, {Name: "A"}}},
		{"other dollar names are not references", `$json.x + $input.item + $now + $workflow.id`, nil},
		{"call with a variable is not a literal reference", `$(name).item`, nil},
		{"unterminated string", `$('Fetch`, nil},
		{"missing closer", `$('Fetch' .item`, nil},
		{"node without bracket", `$node.Fetch`, nil},
		{"dollar at the end", `a $`, nil},
		{"empty", ``, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := scanNodeRefs(tt.expr); !slices.Equal(got, tt.want) {
				t.Errorf("scanNodeRefs(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestTemplateSegments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want []string
	}{
		{"={{ a }} and {{ b }}", []string{" a ", " b "}},
		{"={{ a }}", []string{" a "}},
		{"=no braces", nil},
		{"={{ unterminated", []string{" unterminated"}},
		{"{{ a }} without the equals sign", nil},
		{"=", nil},
	}
	for _, tt := range tests {
		if got := templateSegments(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("templateSegments(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRefsInExpression(t *testing.T) {
	t.Parallel()
	if got := refsInExpression(`$('A').item`); len(got) != 1 {
		t.Errorf("bare expression: %v", got)
	}
	if got := refsInExpression(`={{ $('A').item }}`); len(got) != 1 {
		t.Errorf("template expression: %v", got)
	}
	if got := refsInExpression(`=text $('A') outside braces`); len(got) != 0 {
		t.Errorf("reference outside braces counted: %v", got)
	}
}

func TestStronglyConnected(t *testing.T) {
	t.Parallel()
	// 0 -> 1 -> 2 -> 0, 2 -> 3, 4 alone, 5 -> 5
	adj := [][]int{{1}, {2}, {0, 3}, {}, {}, {5}}
	var big, single, selfLoops int
	for _, comp := range stronglyConnected(adj) {
		switch {
		case len(comp) == 3:
			big++
		case len(comp) == 1 && comp[0] == 5:
			selfLoops++
		default:
			single++
		}
	}
	if big != 1 || single != 2 || selfLoops != 1 {
		t.Errorf("components: big=%d single=%d selfLoops=%d, want 1, 2, 1", big, single, selfLoops)
	}
}

func TestStronglyConnectedDeepChainDoesNotOverflow(t *testing.T) {
	t.Parallel()
	const n = 200000
	adj := make([][]int, n)
	for i := range n - 1 {
		adj[i] = []int{i + 1}
	}
	adj[n-1] = []int{0}
	comps := stronglyConnected(adj)
	if len(comps) != 1 || len(comps[0]) != n {
		t.Errorf("deep cycle: %d components, first of size %d", len(comps), len(comps[0]))
	}
}
