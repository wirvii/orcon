// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// TestJMESPath runs examples from the JMESPath specification through the
// $jmespath function of the expression runtime.
func TestJMESPath(t *testing.T) {
	ev := evaluator(t, expr.Options{DisableFastPath: true})
	s := ev.Open()
	t.Cleanup(func() { _ = s.Close(t.Context()) })

	people := `{"people":[{"name":"a","age":20,"tags":["x","y"]},{"name":"b","age":35,"tags":[]},{"name":"c","age":50,"tags":["z"]}],"a":{"b":{"c":"deep"}},"n":[1,2,3,4,5],"o":{"x":1,"y":2},"m":[[1,2],[3,[4]]],"s":"héllo"}`
	tests := []struct{ query, want string }{
		{`a.b.c`, `"deep"`},
		{`a.nope.c`, `null`},
		{`people[0].name`, `"a"`},
		{`people[-1].name`, `"c"`},
		{`people[*].name`, `["a","b","c"]`},
		{`people[?age > ` + "`30`" + `].name`, `["b","c"]`},
		{`people[?age >= ` + "`30`" + ` && age < ` + "`50`" + `].name`, `["b"]`},
		{`people[?tags[0]=='x'].name`, `["a"]`},
		{`people[?age > ` + "`30`" + `] | [0].name`, `"b"`},
		{`n[1:3]`, `[2,3]`},
		{`n[::2]`, `[1,3,5]`},
		{`n[::-1]`, `[5,4,3,2,1]`},
		{`n[-2:]`, `[4,5]`},
		{`m[]`, `[1,2,3,[4]]`},
		{`m[][]`, `[1,2,3,4]`},
		{`o.*`, `[1,2]`},
		{`keys(o)`, `["x","y"]`},
		{`values(o)`, `[1,2]`},
		{`[people[0].name, people[1].age]`, `["a",35]`},
		{`people[*].{n: name, a: age}`, `[{"a":20,"n":"a"},{"a":35,"n":"b"},{"a":50,"n":"c"}]`},
		{`length(people)`, `3`},
		{`length(s)`, `5`},
		{`sort_by(people, &age)[-1].name`, `"c"`},
		{`max_by(people, &age).name`, `"c"`},
		{`min_by(people, &age).name`, `"a"`},
		{`sum(n)`, `15`},
		{`avg(n)`, `3`},
		{`max(n)`, `5`},
		{`contains(n, ` + "`3`" + `)`, `true`},
		{`contains(s, 'llo')`, `true`},
		{`join(', ', people[*].name)`, `"a, b, c"`},
		{`map(&age, people)`, `[20,35,50]`},
		{`not_null(missing, a.b.c)`, `"deep"`},
		{`to_string(n)`, `"[1,2,3,4,5]"`},
		{`to_number('42')`, `42`},
		{`type(people)`, `"array"`},
		{`reverse(n)`, `[5,4,3,2,1]`},
		{`sort(n)`, `[1,2,3,4,5]`},
		{`merge(o, ` + "`{\"z\": 3}`" + `)`, `{"x":1,"y":2,"z":3}`},
		{`!(people[0].tags)`, `false`},
		{`people[1].tags || 'none'`, `"none"`},
		{`@.n[0]`, `1`},
		{`starts_with(s, 'h')`, `true`},
		{`"zz".b`, `null`},
	}
	for _, tc := range tests {
		p, err := ev.Compile("={{ $jmespath($json, " + strings.ReplaceAll(jsString(tc.query), "\n", "") + ") }}")
		if err != nil {
			t.Fatal(err)
		}
		res, err := s.Eval(t.Context(), p, newEnv(t, people))
		if err != nil {
			t.Errorf("%s: %v", tc.query, err)
			continue
		}
		got, _ := jsonvalue.Canonical(res.Value)
		want, _ := jsonvalue.Parse([]byte(tc.want))
		wantC, _ := jsonvalue.Canonical(want)
		if !bytes.Equal(got, wantC) {
			t.Errorf("%s = %s, want %s", tc.query, got, wantC)
		}
	}

	// An invalid query fails with a JMESPathError, naming the problem.
	p, _ := ev.Compile(`={{ $jmespath($json, 'people[') }}`)
	_, err := s.Eval(t.Context(), p, newEnv(t, people))
	var ee *expr.EvalError
	if !asEvalError(err, &ee) || ee.Name != "JMESPathError" {
		t.Errorf("invalid query error = %v, want a JMESPathError", err)
	}
}

func jsString(s string) string {
	b, _ := jsonvalue.Marshal(jsonvalue.String(s))
	return string(b)
}
