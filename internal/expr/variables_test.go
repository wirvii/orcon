// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr_test

import (
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/expr"
)

// TestAnalyzeExecutionVariables checks SPEC-001 amendment 10, B.3: an
// expression reads an execution variable only as $execution.variables.name or
// $execution.variables["name"]; every other use is a finding.
func TestAnalyzeExecutionVariables(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		code      string
		variables []string
		whole     int // findings of kind FindingExecutionVariables
	}{
		{name: "dot", code: `$execution.variables.total + 1`, variables: []string{"total"}},
		{name: "bracket", code: `$execution.variables["total"]`, variables: []string{"total"}},
		{name: "bracket with single quotes", code: `$execution.variables['a b']`, variables: []string{"a b"}},
		{name: "two names, repeated", code: `$execution.variables.b + $execution.variables.a + $execution.variables.b`, variables: []string{"a", "b"}},
		{name: "method on the value", code: `$execution.variables.label.toUpperCase()`, variables: []string{"label"}},
		{name: "member of the value", code: `$execution.variables.rows.length`, variables: []string{"rows"}},
		{name: "bracket key spelled variables", code: `$execution["variables"].total`, variables: []string{"total"}},
		{name: "optional chaining", code: `$execution?.variables?.total`, variables: []string{"total"}},
		{name: "computed key", code: `$execution.variables[$json.k]`, whole: 1},
		{name: "template key", code: "$execution.variables[`a`]", whole: 1},
		{name: "the object as a whole", code: `JSON.stringify($execution.variables)`, whole: 1},
		{name: "enumerated", code: `Object.keys($execution.variables)`, whole: 1},
		{name: "operator in", code: `'total' in $execution.variables`, whole: 1},
		{name: "assigned to a name", code: `(() => { const v = $execution.variables; return v.total; })()`, whole: 1},
		{name: "destructured", code: `(() => { const { total } = $execution.variables; return total; })()`, whole: 1},
		{name: "other members of $execution", code: `$execution.id + $execution.mode + $execution.resumeUrl + $execution["resumeFormUrl"]`},
		{name: "an unknown member of $execution", code: `$execution.customData`, whole: 1},
		{name: "an alias of $execution", code: `(() => { const e = $execution; return e.variables.total; })()`, whole: 1},
		{name: "destructured $execution", code: `(() => { const { variables } = $execution; return variables.total; })()`, whole: 1},
		{name: "a computed member of $execution", code: `$execution["vari" + "ables"].total`, whole: 1},
		{name: "spread of $execution", code: `({ ...$execution }).variables`, whole: 1},
		{name: "keys of $execution", code: `Object.keys($execution)`, whole: 1},
		{name: "$execution passed to a function", code: `JSON.stringify($execution)`, whole: 1},
		{name: "$execution in a conditional", code: `($json.a ? $execution : {}).variables`, whole: 1},
		{name: "$execution in a template string", code: "`${$execution}`", whole: 1},
		{name: "$execution as a callback argument", code: `[1].map(() => $execution)`, whole: 1},
		{name: "the in operator on $execution", code: `'variables' in $execution`, whole: 1},
		{name: "typeof $execution", code: `typeof $execution`, whole: 1},
		{name: "a local named $execution", code: `(($execution) => $execution.variables.total)({})`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := expr.Analyze(tc.code)
			if !slices.Equal(a.Variables, tc.variables) {
				t.Errorf("Variables = %v, want %v", a.Variables, tc.variables)
			}
			got := 0
			for _, f := range a.Findings {
				if f.Kind == expr.FindingExecutionVariables {
					got++
				}
			}
			if got != tc.whole {
				t.Errorf("%d findings of kind %s, want %d: %+v", got, expr.FindingExecutionVariables, tc.whole, a.Findings)
			}
		})
	}
}

func TestAnalyzeTemplateMergesVariables(t *testing.T) {
	t.Parallel()
	a := expr.AnalyzeTemplate(`=a {{ $execution.variables.x }} b {{ $execution.variables["y"] }} {{ $execution.variables.x }}`)
	if want := []string{"x", "y"}; !slices.Equal(a.Variables, want) {
		t.Errorf("Variables = %v, want %v", a.Variables, want)
	}
}
