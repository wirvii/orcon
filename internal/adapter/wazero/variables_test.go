// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// TestExecutionVariablesAreReadable checks SPEC-001 amendment 10, B.3 in the
// sandbox: a declared variable is read by literal name, its value is the one
// the activation carries, a variable with no value is undefined, and the
// object is read-only.
func TestExecutionVariablesAreReadable(t *testing.T) {
	ev := evaluator(t, expr.Options{DisableFastPath: true})
	s := ev.Open()
	t.Cleanup(func() { _ = s.Close(t.Context()) })
	env := newEnv(t, `{}`)
	env.Execution.Variables = parse(t, `{"total":41,"label":"x","rows":[1,{"a":2}],"unread":"secret"}`)
	for src, want := range map[string]string{
		`={{ $execution.variables.total + 1 }}`:                               `42`,
		`={{ $execution.variables["label"].toUpperCase() }}`:                  `"X"`,
		`={{ $execution.variables.rows[1].a }}`:                               `2`,
		`={{ $execution.variables.missing }}`:                                 `null`,
		`={{ Object.keys($execution.variables).length }}`:                     `0`,
		`={{ ($execution.variables.total = 5, $execution.variables.total) }}`: `41`,
		`={{ (() => { try { $execution.variables.rows.push(3); } catch (e) {} return $execution.variables.rows.length; })() }}`: `2`,
		`={{ $execution.id + $execution.variables.label }}`:                                                                     `"x"`,
	} {
		p, err := ev.Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		res, err := s.Eval(t.Context(), p, env)
		if err != nil {
			t.Errorf("Eval(%s): %v", src, err)
			continue
		}
		got, _ := jsonvalue.Canonical(res.Value)
		if string(got) != want {
			t.Errorf("Eval(%s) = %s, want %s", src, got, want)
		}
	}
}
