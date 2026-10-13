// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/expr"
)

func TestAnalyzeNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		code        string
		identifiers []string
		methods     []string
		refs        []expr.NodeRef
		kinds       []expr.FindingKind
	}{
		{name: "context names", code: `$json.a + $itemIndex`, identifiers: []string{"$itemIndex", "$json"}},
		{name: "globals are known", code: `Math.max(1, JSON.parse('2'))`, identifiers: []string{"JSON", "Math"}, methods: []string{"max", "parse"}},
		{name: "arrow parameters are not free", code: `$json.items.map(i => i.price).filter(p => p > limit)`, identifiers: []string{"$json", "limit"}, methods: []string{"filter", "map"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier}},
		{name: "function parameters and locals", code: `(function (a, {b, c = d}, [e], ...rest) { var x = a; let y = b; const z = e; return x + y + z + rest + q; })()`, identifiers: []string{"d", "q"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier}},
		{name: "block scoped declarations", code: `(() => { for (let i = 0; i < 3; i++) { const t = i; } for (const k in $json) {} for (const [a, b] of $json.list) {} try { f(); } catch (err) { return err; } finally {} switch (1) { case 1: let s = 1; } return 0; })()`, identifiers: []string{"$json", "f"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier}},
		{name: "named function and class declarations", code: `(() => { function inner(p) { return p; } class K {} label: while (false) { break label; } do { } while (false); return inner(K); })()`},
		{name: "statements in functions", code: `(() => { if (a) { return b; } else { throw c; } })()`, identifiers: []string{"a", "b", "c"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier}},
		{name: "object and array literals", code: `({a: $json.a, [k]: 1, ...$json.o, s, m() {}, get g() { return 1 }})`, identifiers: []string{"$json", "k", "s"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier}},
		{name: "templates, new, sequence, conditional, unary", code: "`x${y}` + (new Date(z), typeof w === 'undefined' ? -a : !b)", identifiers: []string{"Date", "a", "b", "w", "y", "z"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier}},
		{name: "optional chaining and spread", code: `f(...$json?.list?.items, ...g)`, identifiers: []string{"$json", "f", "g"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier}},
		{name: "method names", code: `$json.a.toUpperCase().trim() + $json['b'].toLowerCase() + $json.c.nopeMethod()`, identifiers: []string{"$json"}, methods: []string{"nopeMethod", "toLowerCase", "toUpperCase", "trim"}, kinds: []expr.FindingKind{expr.FindingUnknownMethod}},
		{name: "extension methods are known", code: `$json.s.toTitleCase() + $json.a.sum() + $now.isWeekend()`, identifiers: []string{"$json", "$now"}, methods: []string{"isWeekend", "sum", "toTitleCase"}},
		{name: "node references", code: `$('Fetch').item.json.a + $("Other").first().json.b + $node["Legacy"].json.c + $items('Old')`, identifiers: []string{"$", "$items", "$node"}, methods: []string{"first"},
			refs: []expr.NodeRef{{Name: "Fetch", Offset: 0}, {Name: "Other", Offset: 25}, {Name: "Legacy", Offset: 53}, {Name: "Old", Offset: 78}}},
		{name: "guarded reference", code: `$('A').isExecuted && $('A').item.json.x`, identifiers: []string{"$"}, refs: []expr.NodeRef{{Name: "A", Guarded: true, Offset: 0}, {Name: "A", Offset: 21}}},
		{name: "guarded legacy reference", code: `$node["A"].isExecuted ? 1 : 2`, identifiers: []string{"$node"}, refs: []expr.NodeRef{{Name: "A", Guarded: true, Offset: 0}}},
		{name: "computed references are reported", code: `$($json.name).item + $items(x) + $node[k].json`, identifiers: []string{"$", "$items", "$json", "$node", "k", "x"},
			kinds: []expr.FindingKind{expr.FindingDynamicReference, expr.FindingDynamicReference, expr.FindingUnknownIdentifier, expr.FindingDynamicReference, expr.FindingUnknownIdentifier}},
		{name: "input without arguments", code: `$items().length`, identifiers: []string{"$items"}},
		{name: "unknown identifier", code: `fetch('x')`, identifiers: []string{"fetch"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier}},
		{name: "syntax error", code: `$json.`, kinds: []expr.FindingKind{expr.FindingSyntax}},
		{name: "trailing semicolon", code: `$json.a;`, identifiers: []string{"$json"}},
		{name: "bound context name is not the context", code: `[1].map($json => $json.x)`, methods: []string{"map"}},
		{name: "assignment", code: `(x = 1, y += 2)`, identifiers: []string{"x", "y"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier, expr.FindingUnknownIdentifier}},
		{name: "class in an expression", code: `(class extends Base {})`, identifiers: []string{"Base"}, kinds: []expr.FindingKind{expr.FindingUnknownIdentifier}},
		{name: "with and eval are dynamic", code: `eval('1')`, identifiers: []string{"eval"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := expr.Analyze(tc.code)
			if diff := cmp.Diff(tc.identifiers, a.Identifiers); diff != "" {
				t.Errorf("identifiers (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.methods, a.Methods); diff != "" {
				t.Errorf("methods (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.refs, a.NodeRefs); diff != "" {
				t.Errorf("node references (-want +got):\n%s", diff)
			}
			var kinds []expr.FindingKind
			for _, f := range a.Findings {
				kinds = append(kinds, f.Kind)
			}
			if diff := cmp.Diff(tc.kinds, kinds); diff != "" {
				t.Errorf("findings (-want +got):\n%s\n%v", diff, a.Findings)
			}
			if a.Supported() != (len(tc.kinds) == 0) {
				t.Errorf("Supported() = %v with %d findings", a.Supported(), len(a.Findings))
			}
			if a.Syntax() != slicesContains(kinds, expr.FindingSyntax) {
				t.Errorf("Syntax() = %v", a.Syntax())
			}
			if a.Size <= 0 && !a.Syntax() {
				t.Errorf("Size = %d, want a positive node count", a.Size)
			}
		})
	}
}

func slicesContains(kinds []expr.FindingKind, k expr.FindingKind) bool {
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}

func TestAnalyzeJSONPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code    string
		paths   [][]string
		whole   bool
		dynamic bool
	}{
		{`$json.a`, [][]string{{"a"}}, false, false},
		{`$json.a.b + $json['c'].d`, [][]string{{"a", "b"}, {"c", "d"}}, false, false},
		{`$json.a.b.toUpperCase()`, [][]string{{"a", "b"}}, false, false},
		{`$json.a['b'].c()`, [][]string{{"a", "b"}}, false, false},
		{`$json.list[0].name`, [][]string{{"list"}}, false, false},
		{`$json.list['0'].name`, [][]string{{"list"}}, false, false},
		{`$json[key]`, [][]string{{}}, false, false},
		{`$json.a[key].b`, [][]string{{"a"}}, false, false},
		{`$json.a + $json.a.b`, [][]string{{"a"}}, false, false},
		{`$json`, nil, true, false},
		{`Object.keys($json)`, nil, true, false},
		{`JSON.stringify($json.a)`, [][]string{{"a"}}, false, false},
		{`{...$json}`, nil, true, false},
		{`eval('$json.a')`, [][]string{}, true, true},
		{`(function () { return arguments[0]; })()`, nil, true, true},
		{`1 +`, nil, true, true},
		{`[1].map($json => $json.x)`, nil, false, false},
		{`$json.a = 1`, [][]string{{"a"}}, false, false},
		{`$json?.a?.b`, nil, true, false},
	}
	for _, tc := range tests {
		a := expr.Analyze(tc.code)
		if a.JSONWhole != tc.whole || a.Dynamic != tc.dynamic {
			t.Errorf("%s: whole=%v dynamic=%v, want %v %v", tc.code, a.JSONWhole, a.Dynamic, tc.whole, tc.dynamic)
		}
		if tc.whole && tc.paths == nil {
			continue
		}
		if !tc.whole {
			if diff := cmp.Diff(tc.paths, a.JSONPaths); diff != "" && tc.paths != nil {
				t.Errorf("%s: paths (-want +got):\n%s", tc.code, diff)
			}
		}
	}
}

func TestAnalyzeTemplate(t *testing.T) {
	t.Parallel()
	a := expr.AnalyzeTemplate(`=Hi {{ $json.name.toUpperCase() }}, {{ $('Fetch').item.json.id }} {{ nope }}`)
	if diff := cmp.Diff([]string{"$", "$json", "nope"}, a.Identifiers); diff != "" {
		t.Errorf("identifiers (-want +got):\n%s", diff)
	}
	if len(a.NodeRefs) != 1 || a.NodeRefs[0].Name != "Fetch" || a.NodeRefs[0].Offset < 20 {
		t.Errorf("node references = %+v, want Fetch at an offset inside the template body", a.NodeRefs)
	}
	if len(a.Findings) != 1 || a.Findings[0].Name != "nope" || a.Findings[0].Offset < 40 {
		t.Errorf("findings = %+v, want one for nope at its offset in the template", a.Findings)
	}
	if a.Size == 0 || len(a.JSONPaths) == 0 {
		t.Errorf("Size = %d, JSONPaths = %v", a.Size, a.JSONPaths)
	}
	if got := expr.AnalyzeTemplate("not a template {{ x }}"); len(got.Identifiers) != 0 || len(got.Findings) != 0 {
		t.Errorf("a string without the leading = holds no code, got %+v", got)
	}
	whole := expr.AnalyzeTemplate(`={{ $json }}{{ eval('1') }}`)
	if !whole.JSONWhole || !whole.Dynamic {
		t.Errorf("merged analysis lost JSONWhole/Dynamic: %+v", whole)
	}
}

func TestNameLists(t *testing.T) {
	t.Parallel()
	ctx, glob := expr.ContextNames(), expr.GlobalNames()
	if len(ctx) < 20 || len(glob) < 40 {
		t.Errorf("name lists look truncated: %d context names, %d globals", len(ctx), len(glob))
	}
	for _, list := range [][]string{ctx, glob} {
		for i := 1; i < len(list); i++ {
			if list[i-1] >= list[i] {
				t.Errorf("list not sorted without repeats at %q, %q", list[i-1], list[i])
			}
		}
	}
}

// FuzzAnalyze checks that analysis never panics and stays consistent.
func FuzzAnalyze(f *testing.F) {
	for _, s := range []string{"", "$json.a", "$json.a.b[0]['c']", "(a, b) => a + b", "try { x } catch (e) {} finally {}", "`${", "class A { static { } }", "{{", "with (a) b", "$('x').item", "\"\\ud800\"", "a ? b : c ?? d", "for (const k of $json) {}", "async () => await x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, code string) {
		a := expr.Analyze(code)
		if a.Syntax() && (len(a.Identifiers) != 0 || !a.JSONWhole || !a.Dynamic) {
			t.Fatalf("a syntax error must be conservative: %+v", a)
		}
		for i := 1; i < len(a.Identifiers); i++ {
			if a.Identifiers[i-1] >= a.Identifiers[i] {
				t.Fatalf("identifiers not sorted: %v", a.Identifiers)
			}
		}
		_ = expr.AnalyzeTemplate("=" + code)
	})
}
