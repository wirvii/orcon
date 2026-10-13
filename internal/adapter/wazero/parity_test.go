// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// outcome is the observable result of an evaluation: the canonical JSON of the
// value, or the name of the error.
func outcome(res expr.Result, err error) string {
	if err != nil {
		var ee *expr.EvalError
		if errors.As(err, &ee) {
			return "error:" + ee.Name
		}
		return "failure:" + err.Error()
	}
	if res.Undefined {
		return "undefined"
	}
	b, cerr := jsonvalue.Canonical(res.Value)
	if cerr != nil {
		return "unencodable:" + cerr.Error()
	}
	return string(b)
}

// twoPaths evaluates each source against each data set on both paths and
// reports every difference. With fastMustApply it also asserts that the fast
// path takes the expression. It returns the number of evaluations compared.
func twoPaths(t *testing.T, sources, data []string, fastMustApply bool) int {
	t.Helper()
	fast := evaluator(t, expr.Options{})
	slow := evaluator(t, expr.Options{DisableFastPath: true})
	fs, ss := fast.Open(), slow.Open()
	t.Cleanup(func() { _ = fs.Close(t.Context()); _ = ss.Close(t.Context()) })
	reported, evaluated := 0, 0
	for _, src := range sources {
		fp, err := fast.Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		sp, err := slow.Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		if fastMustApply && fp.FastSegments() != fp.CodeSegments() {
			t.Errorf("%s: expected the fast path to take it", src)
			continue
		}
		for _, d := range data {
			fr, ferr := fs.Eval(t.Context(), fp, newEnv(t, d))
			sr, serr := ss.Eval(t.Context(), sp, newEnv(t, d))
			evaluated++
			got, want := outcome(fr, ferr), outcome(sr, serr)
			if got != want && reported < 40 {
				reported++
				t.Errorf("parity: %s on %s: fast path %s, interpreter %s", src, d, got, want)
			}
		}
	}
	return evaluated
}

// values are JSON data covering every type and the awkward members of each.
var parityValues = []string{
	`null`, `true`, `false`, `0`, `-0`, `1`, `-1`, `2`, `1.5`, `100`, `1e21`, `0.1`,
	`""`, `"a"`, `"b"`, `"1"`, `"abc"`, `" 12 "`, `"0x10"`, `"1e3"`, `"Infinity"`, `"-1"`, `"é"`, `"😀"`,
	`[]`, `[1]`, `[1,2]`, `{}`, `{"a":1}`, `{"a":{"b":2}}`,
}

func TestFastPathParity(t *testing.T) {
	// Binary and logical operators over every pair of values.
	ops := []string{"+", "-", "*", "/", "%", "==", "!=", "===", "!==", "<", ">", "<=", ">=", "&&", "||", "??"}
	var pairs []string
	for _, op := range ops {
		pairs = append(pairs, "={{ $json.x "+op+" $json.y }}")
	}
	var data []string
	for _, x := range parityValues {
		for _, y := range parityValues {
			data = append(data, fmt.Sprintf(`{"x":%s,"y":%s}`, x, y))
		}
	}
	n := twoPaths(t, pairs, data, true)
	t.Logf("binary operators: %d evaluations compared", n)

	// Unary operators, conditionals and member access over every value.
	var unary = []string{
		"={{ !$json.x }}", "={{ -$json.x }}", "={{ +$json.x }}", "={{ typeof $json.x }}",
		"={{ $json.x ? 'yes' : 'no' }}", "={{ $json.x ? $json.x : 'no' }}",
		"={{ $json.x.length }}", "={{ $json.x[0] }}", "={{ $json.x['a'] }}", "={{ $json.x.a }}", "={{ $json.x.a.b }}",
		"={{ $json.x.constructor }}", "={{ $json.x.toString }}", "={{ $json.x.hasOwnProperty }}", "={{ $json.x.isEmpty }}",
		"={{ $json.x[1] }}", "={{ $json.x[-1] }}", "={{ $json.x['length'] }}", "={{ $json.x['0'] }}", "={{ $json.x['01'] }}",
		"={{ $json.x + '' }}", "={{ '' + $json.x }}", "={{ $json.x == null }}", "={{ $json.x === undefined }}",
		"={{ 'v=' + $json.x }}", "={{ $json.x }}", "={{ $json.missing }}", "={{ $json.x.a ?? 'd' }}",
	}
	var single []string
	for _, x := range parityValues {
		single = append(single, fmt.Sprintf(`{"x":%s}`, x))
	}
	n = twoPaths(t, unary, single, false)
	t.Logf("unary, conditional and member access: %d evaluations compared", n)

	// Expressions made of literals and arithmetic corner cases.
	literals := []string{
		"={{ 1 + 2 * 3 }}", "={{ (1 + 2) * 3 }}", "={{ 10 / 4 }}", "={{ 10 % 4 }}", "={{ -10 % 4 }}", "={{ 10 % 0 }}", "={{ 1 / 0 }}",
		"={{ -1 / 0 }}", "={{ 0 / 0 }}", "={{ 0.1 + 0.2 }}", "={{ 9007199254740993 }}", "={{ 1e21 }}", "={{ 1e-7 }}", "={{ 123456789012345680000 }}",
		"={{ 'a' + 1 + 2 }}", "={{ 1 + 2 + 'a' }}", "={{ '3' * '4' }}", "={{ '3' - 1 }}", "={{ null + 1 }}", "={{ undefined + 1 }}", "={{ true + true }}",
		"={{ NaN }}", "={{ Infinity }}", "={{ -Infinity }}", "={{ NaN === NaN }}", "={{ NaN == NaN }}", "={{ null == undefined }}", "={{ null === undefined }}",
		"={{ '' == 0 }}", "={{ '0' == false }}", "={{ 'a' < 'b' }}", "={{ 'a' < 'B' }}", "={{ '10' < '9' }}", "={{ 10 < '9' }}", "={{ 'a' < 1 }}",
		"={{ typeof null }}", "={{ typeof undefined }}", "={{ typeof 1 }}", "={{ typeof 'a' }}", "={{ typeof true }}", "={{ !'' }}", "={{ !!'0' }}",
		"={{ 1 ? 'a' : 'b' }}", "={{ 0 ? 'a' : 'b' }}", "={{ null ?? 'd' }}", "={{ 0 ?? 'd' }}", "={{ 0 || 'd' }}", "={{ 1 && 2 }}", "={{ '' && 2 }}",
		"={{ 'x\\u00e9y' }}", "={{ \"it's\" }}", "={{ -0 }}", "={{ 0 * -1 }}", "={{ '\\ud83d\\ude00'.length }}", "={{ 'abc'.length }}", "={{ 'abc'[1] }}",
		"={{ 'a\\u00e9' < 'a\\ud83d\\ude00' }}", "={{ '\\uffff' < '\\ud83d\\ude00' }}", "={{ 0x10 }}", "={{ 1_000 }}", "={{ .5 + 5. }}",
		"={{ $itemIndex }}", "={{ $runIndex }}", "={{ $itemIndex + 1 }}",
		"={{ 'Total: ' + 1 + ' items' }}", "={{ 'a' }} and {{ 1 + 1 }}", "text {{ $json.x }} text", "={{ $json.x }}{{ $json.x }}", "= {{ 1 }}", "=plain",
	}
	n = twoPaths(t, literals, single, false)
	t.Logf("literals and arithmetic: %d evaluations compared", n)
}

func TestFastPathCoverageOfShapes(t *testing.T) {
	// The expressions that must stay on the fast path: they are the bulk of
	// real flows (ADR-0018 section 2).
	ev := evaluator(t, expr.Options{})
	for _, src := range []string{
		"={{ $json.a }}", "={{ $json.a.b.c }}", "={{ $json['a'] }}", "={{ $json.a[0].b }}", "={{ $json.a + 1 }}", "={{ $json.a * $json.b }}",
		"={{ $json.a > 1 ? 'big' : 'small' }}", "={{ $json.a && $json.b }}", "={{ $json.a ?? 'default' }}", "={{ $input.item.json.a }}",
		"={{ $input.first().json.a }}", "={{ $input.last().json.a }}", "={{ $('Node').item.json.a }}", "={{ $('Node').first().json.a }}",
		"={{ $('Node').last().json.a }}", "={{ $node['Node'].json.a }}", "={{ $('Node').isExecuted }}", "=Hello {{ $json.name }}!",
		"={{ $itemIndex }}", "={{ $json.a.length }}",
	} {
		p, err := ev.Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		if p.FastSegments() != p.CodeSegments() || p.CodeSegments() == 0 {
			t.Errorf("%s: the fast path should take it (%d of %d segments)", src, p.FastSegments(), p.CodeSegments())
		}
	}
	for _, src := range []string{
		"={{ $json.a.toUpperCase() }}", "={{ $now }}", "={{ [1,2].map(x => x) }}", "={{ JSON.stringify($json) }}", "={{ $json.a ** 2 }}",
		"={{ `t${$json.a}` }}", "={{ $json.a?.b }}", "={{ new Date() }}",
	} {
		p, err := ev.Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		if p.FastSegments() != 0 {
			t.Errorf("%s: the fast path must not claim it", src)
		}
	}
}
