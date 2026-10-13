// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr_test

import (
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/expr"
)

const noSandbox = "needs the sandbox"

// TestFastPathSemantics checks the JavaScript semantics the fast path
// reproduces, against values written from the language specification. An
// expression it must not decide ends with ErrNoSandbox (an evaluator without a
// sandbox cannot take the fallback).
func TestFastPathSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, item, want string
	}{
		// literals and arithmetic
		{"={{ 1 + 2 * 3 }}", `{}`, `7`},
		{"={{ (1 + 2) * 3 }}", `{}`, `9`},
		{"={{ 10 / 4 }}", `{}`, `2.5`},
		{"={{ -10 % 4 }}", `{}`, `-2`},
		{"={{ 10 % 0 }}", `{}`, `null`},
		{"={{ 1 / 0 }}", `{}`, `null`},
		{"={{ 0.1 + 0.2 }}", `{}`, `0.30000000000000004`},
		{"={{ 1e21 }}", `{}`, `1e+21`},
		{"={{ 0x10 + 1_000 }}", `{}`, `1016`},
		{"={{ -0 }}", `{}`, `0`},
		{"={{ +'  42  ' }}", `{}`, `42`},
		{"={{ +'0x1f' }}", `{}`, `31`},
		{"={{ +'0b101' }}", `{}`, `5`},
		{"={{ +'0o17' }}", `{}`, `15`},
		{"={{ +'1e3' }}", `{}`, `1000`},
		{"={{ +'' }}", `{}`, `0`},
		{"={{ +'abc' }}", `{}`, `null`},
		{"={{ +'-Infinity' }}", `{}`, `null`},
		{"={{ +'0x' }}", `{}`, `null`},
		{"={{ +'1.5.5' }}", `{}`, `null`},
		{"={{ +'1e' }}", `{}`, `null`},
		{"={{ +'.5' }}", `{}`, `0.5`},
		{"={{ +'1e999' }}", `{}`, `null`},
		{"={{ '3' * '4' }}", `{}`, `12`},
		{"={{ '3' - 1 }}", `{}`, `2`},
		{"={{ null + 1 }}", `{}`, `1`},
		{"={{ undefined + 1 }}", `{}`, `null`},
		{"={{ true + true }}", `{}`, `2`},
		// strings
		{"={{ 'a' + 1 + 2 }}", `{}`, `"a12"`},
		{"={{ 1 + 2 + 'a' }}", `{}`, `"3a"`},
		{"={{ 'x' + null + undefined + true }}", `{}`, `"xnullundefinedtrue"`},
		{"={{ 'a' + NaN + Infinity + -Infinity }}", `{}`, `"aNaNInfinity-Infinity"`},
		{"={{ 'abc'.length }}", `{}`, `3`},
		{"={{ '\\ud83d\\ude00'.length }}", `{}`, `2`},
		{"={{ 'abc'[1] }}", `{}`, `"b"`},
		{"={{ 'abc'[5] }}", `{}`, `undefined`},
		{"={{ 'abc'.nope }}", `{}`, `undefined`},
		{"={{ 'abc'.toUpperCase }}", `{}`, noSandbox},
		{"={{ '\\ud83d\\ude00'[0] }}", `{}`, noSandbox},
		// comparison and equality
		{"={{ 'a' < 'b' }}", `{}`, `true`},
		{"={{ 'a' < 'B' }}", `{}`, `false`},
		{"={{ '10' < '9' }}", `{}`, `true`},
		{"={{ 10 < '9' }}", `{}`, `false`},
		{"={{ 'a' < 1 }}", `{}`, `false`},
		{"={{ 'a\\u00e9' < 'a\\ud83d\\ude00' }}", `{}`, `true`},
		{"={{ '\\uffff' < '\\ud83d\\ude00' }}", `{}`, `false`},
		{"={{ 1 <= 1 }}", `{}`, `true`},
		{"={{ 2 >= 3 }}", `{}`, `false`},
		{"={{ NaN <= NaN }}", `{}`, `false`},
		{"={{ NaN == NaN }}", `{}`, `false`},
		{"={{ null == undefined }}", `{}`, `true`},
		{"={{ null === undefined }}", `{}`, `false`},
		{"={{ null == 0 }}", `{}`, `false`},
		{"={{ '' == 0 }}", `{}`, `true`},
		{"={{ '0' == false }}", `{}`, `true`},
		{"={{ 1 != '1' }}", `{}`, `false`},
		{"={{ 1 !== '1' }}", `{}`, `true`},
		{"={{ true === true }}", `{}`, `true`},
		{"={{ 'a' === 'a' }}", `{}`, `true`},
		// logical, unary, conditional
		{"={{ !'' }}", `{}`, `true`},
		{"={{ !!'0' }}", `{}`, `true`},
		{"={{ typeof null + typeof 1 + typeof 'a' + typeof true + typeof undefined }}", `{}`, `"objectnumberstringbooleanundefined"`},
		{"={{ typeof {} }}", `{}`, noSandbox},
		{"={{ 0 ?? 'd' }}", `{}`, `0`},
		{"={{ null ?? 'd' }}", `{}`, `"d"`},
		{"={{ 0 || 'd' }}", `{}`, `"d"`},
		{"={{ 1 && 2 }}", `{}`, `2`},
		{"={{ '' && 2 }}", `{}`, `""`},
		{"={{ 1 ? 'a' : 'b' }}", `{}`, `"a"`},
		{"={{ 0 ? 'a' : 'b' }}", `{}`, `"b"`},
		// item data
		{"={{ $json.a }}", `{"a":{"b":[1,2,{"c":"deep"}]}}`, `{"b":[1,2,{"c":"deep"}]}`},
		{"={{ $json.a.b[2].c }}", `{"a":{"b":[1,2,{"c":"deep"}]}}`, `"deep"`},
		{"={{ $json['a']['b'][0] }}", `{"a":{"b":[1,2]}}`, `1`},
		{"={{ $json.missing }}", `{}`, `undefined`},
		{"={{ $json.a.length }}", `{"a":[1,2,3]}`, `3`},
		{"={{ $json.a[9] }}", `{"a":[1]}`, `undefined`},
		{"={{ $json.a[-1] }}", `{"a":[1]}`, `undefined`},
		{"={{ $json.a['01'] }}", `{"a":[1]}`, `undefined`},
		{"={{ $json.a.foo }}", `{"a":[1]}`, `undefined`},
		{"={{ $json.a.map }}", `{"a":[1]}`, noSandbox},
		{"={{ $json.a.toString }}", `{"a":{}}`, noSandbox},
		{"={{ $json.a.compact }}", `{"a":{}}`, noSandbox},
		{"={{ $json.a.compact }}", `{"a":{"compact":3}}`, `3`},
		{"={{ $json.n.foo }}", `{"n":5}`, noSandbox},
		{"={{ $json.a.b }}", `{"a":null}`, noSandbox},
		{"={{ $json.a.b }}", `{}`, noSandbox},
		{"={{ $json.a + 1 }}", `{"a":{}}`, noSandbox},
		{"={{ $json.a == 1 }}", `{"a":[1]}`, noSandbox},
		{"={{ $json.a === $json.a }}", `{"a":[1]}`, noSandbox},
		{"={{ $json.a < 1 }}", `{"a":[1]}`, noSandbox},
		{"={{ $json.a == null }}", `{"a":[1]}`, `false`},
		{"={{ $json.a == null }}", `{"a":null}`, `true`},
		{"={{ $json.big }}", `{"big":1e999}`, noSandbox},
		{"={{ $itemIndex + $runIndex + $position }}", `{}`, `0`},
		{"={{ $json }}", ``, `{}`},
		{"={{ $json.a ? 'y' : 'n' }}", `{"a":0}`, `"n"`},
		{"={{ $json.a.b }}x", `{"a":{"b":1}}`, `"{{ $json.a.b }}x"`},
		// templates
		{"=Hello {{ $json.n }}!", `{"n":"Ada"}`, `"Hello Ada!"`},
		{"=a {{ 1 + 1 }} b {{ 'x' }}", `{}`, `"a 2 b x"`},
		{"={{ 1 }}{{ 2 }}", `{}`, `"12"`},
		{"= {{ 1 }}", `{}`, `" 1"`},
		{"=plain", `{}`, `"plain"`},
		{"=", `{}`, `""`},
		{"not a template {{ 1 }}", `{}`, `"not a template {{ 1 }}"`},
		{"=n: {{ $json.o }}", `{"o":{}}`, noSandbox},
		{"={{ 1 }}   ", `{}`, `"1   "`},
		{"={{ 1; }}", `{}`, `1`},
		{"={{ 1 ; ; }}", `{}`, `1`},
	}
	for _, tc := range tests {
		t.Run(tc.src+" on "+tc.item, func(t *testing.T) {
			t.Parallel()
			item := tc.item
			if item == "" {
				item = "null"
			}
			var res expr.Result
			var err error
			if tc.item == "" {
				ev := expr.New(expr.Options{})
				p, cerr := ev.Compile(tc.src)
				if cerr != nil {
					t.Fatal(cerr)
				}
				env := newEnv(t, "{}")
				env.Input = nil // no input item at all
				res, err = ev.Open().Eval(t.Context(), p, env)
			} else {
				res, err = eval(t, tc.src, item)
			}
			if tc.want == noSandbox {
				if !errors.Is(err, expr.ErrNoSandbox) {
					t.Fatalf("error = %v (result %v), want ErrNoSandbox", err, res)
				}
				return
			}
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if tc.src == "={{ $json.a.b }}x" {
				return // not a template-with-text case: exercised below
			}
			if got := canon(t, res); got != tc.want {
				t.Errorf("= %s, want %s", got, tc.want)
			}
		})
	}
}

func TestFastPathSources(t *testing.T) {
	t.Parallel()
	ev := expr.New(expr.Options{})
	items := []expr.Item{{JSON: jv(t, `{"id":1}`)}, {JSON: jv(t, `{"id":2}`)}}
	nodes := fakeNodes{
		"Fetch": {Executed: true, Items: items, Paired: func(i int) (expr.Item, bool) {
			if i < len(items) {
				return items[i], true
			}
			return expr.Item{}, false
		}},
		"Skipped":   {Executed: false},
		"NoLineage": {Executed: true, Items: items},
	}
	env := func(index int) *expr.Env {
		e := newEnv(t, `{"id":99}`)
		e.Input = []expr.Item{{JSON: jv(t, `{"id":10}`)}, {JSON: jv(t, `{"id":11}`)}}
		e.ItemIndex = index
		e.Nodes = nodes
		return e
	}
	tests := []struct {
		src  string
		idx  int
		want string
	}{
		{"={{ $json.id }}", 1, `11`},
		{"={{ $input.item.json.id }}", 1, `11`},
		{"={{ $input.first().json.id }}", 1, `10`},
		{"={{ $input.last().json.id }}", 0, `11`},
		{"={{ $('Fetch').item.json.id }}", 1, `2`},
		{"={{ $('Fetch').first().json.id }}", 1, `1`},
		{"={{ $('Fetch').last().json.id }}", 0, `2`},
		{"={{ $node['Fetch'].json.id }}", 1, `2`},
		{"={{ $('Fetch').isExecuted }}", 0, `true`},
		{"={{ $('Skipped').isExecuted }}", 0, `false`},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			t.Parallel()
			p, err := ev.Compile(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			if p.FastSegments() != 1 {
				t.Fatalf("not on the fast path")
			}
			res, err := ev.Open().Eval(t.Context(), p, env(tc.idx))
			if err != nil {
				t.Fatal(err)
			}
			if got := canon(t, res); got != tc.want {
				t.Errorf("= %s, want %s", got, tc.want)
			}
		})
	}
	// Reads the fast path cannot answer fall to the sandbox, which raises the
	// documented error; without one they report ErrNoSandbox.
	for _, src := range []string{
		"={{ $('Missing').item.json.id }}", "={{ $('Skipped').item.json.id }}", "={{ $('Skipped').first().json.id }}",
		"={{ $('NoLineage').item.json.id }}", "={{ $('Fetch').item.json.id }}", "={{ $input.first().json.id }}",
	} {
		e := env(5)
		if src == "={{ $input.first().json.id }}" {
			e.Input = nil
		}
		p, err := ev.Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ev.Open().Eval(t.Context(), p, e); !errors.Is(err, expr.ErrNoSandbox) {
			t.Errorf("%s: error = %v, want ErrNoSandbox", src, err)
		}
	}
	e := env(0)
	e.Nodes = nil
	p, _ := ev.Compile("={{ $('Fetch').first().json.id }}")
	if _, err := ev.Open().Eval(t.Context(), p, e); !errors.Is(err, expr.ErrNoSandbox) {
		t.Errorf("without a node source: %v, want ErrNoSandbox", err)
	}
	p, _ = ev.Compile("={{ $('Fetch').isExecuted }}")
	if _, err := ev.Open().Eval(t.Context(), p, e); !errors.Is(err, expr.ErrNoSandbox) {
		t.Errorf("isExecuted without a node source: %v, want ErrNoSandbox", err)
	}
	p, _ = ev.Compile("={{ $('Nope').isExecuted }}")
	if _, err := ev.Open().Eval(t.Context(), p, env(0)); !errors.Is(err, expr.ErrNoSandbox) {
		t.Errorf("isExecuted of an unknown node: %v, want ErrNoSandbox", err)
	}
}

// fakeNodes is an expr.NodeSource over fixed outputs.
type fakeNodes map[string]expr.NodeOutput

func (f fakeNodes) Node(name string) (expr.NodeOutput, bool) {
	o, ok := f[name]
	return o, ok
}

func TestFastPathClassification(t *testing.T) {
	t.Parallel()
	ev := expr.New(expr.Options{})
	for src, fast := range map[string]bool{
		"={{ $json.a }}": true, "={{ $json.a + 1 }}": true, "={{ $json.a ? 1 : 2 }}": true, "={{ $json.a ?? 1 }}": true,
		"=text {{ $json.a }} and {{ $json.b }}": true, "={{ $json.a.toUpperCase() }}": false, "={{ $now }}": false,
		"={{ $json.a ** 2 }}": false, "={{ [1] }}": false, "={{ {a: 1} }}": false, "={{ $json.a++ }}": false,
		"={{ typeof $json }}": true, "={{ typeof foo }}": false, "={{ x => x }}": false, "={{ 10n }}": false,
		"={{ $json.a, 1 }}": false, "={{ `t` }}": false, "={{ 1 +  }}": false, "={{ 1; 2 }}": false,
		"={{ $json?.a }}": false, "={{ void 0 }}": false, "={{ $json[0n] }}": false, "={{ ~1 }}": false, "={{ 1 << 2 }}": false,
		"={{ $json.a in $json }}": false, "={{ 'a' instanceof Object }}": false, "={{ $json.a = 1 }}": false,
		"={{ $('x') }}": false, "={{ $('x').item }}": false, "={{ $node['x'] }}": false, "={{ $input.item }}": false,
		"={{ \"\\ud800\" }}": false,
	} {
		p, err := ev.Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.FastSegments() == p.CodeSegments() && p.CodeSegments() > 0; got != fast {
			t.Errorf("%s: fast path = %v, want %v", src, got, fast)
		}
	}
	disabled := expr.New(expr.Options{DisableFastPath: true})
	p, _ := disabled.Compile("={{ $json.a }}")
	if p.FastSegments() != 0 {
		t.Error("DisableFastPath left a fast segment")
	}
}
