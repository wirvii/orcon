// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/expr/bench"
)

// BenchmarkExprFastPath measures the evaluation of simple expressions without
// the sandbox, the hot path of performance.md PERF-1.
func BenchmarkExprFastPath(b *testing.B) {
	ev := expr.New(expr.Options{})
	for _, c := range bench.Cases()[:9] { // the cases the fast path takes
		prog, err := ev.Compile(c.Template)
		if err != nil {
			b.Fatal(err)
		}
		if prog.FastSegments() != prog.CodeSegments() {
			b.Fatalf("%s is not a fast-path case", c.Name)
		}
		env, err := c.Env()
		if err != nil {
			b.Fatal(err)
		}
		b.Run(c.Name, func(b *testing.B) {
			s := ev.Open()
			b.ReportAllocs()
			for b.Loop() {
				if _, err := s.Eval(b.Context(), prog, env); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
