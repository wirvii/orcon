// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/expr/bench"
)

func sandboxCases(b *testing.B) []bench.Case {
	b.Helper()
	var out []bench.Case
	for _, name := range []string{"string method", "map and reduce", "date format", "jmespath"} {
		for _, c := range bench.Cases() {
			if c.Name == name {
				out = append(out, c)
			}
		}
	}
	return out
}

// BenchmarkExprSandboxWarm measures an expression evaluated by a runtime that
// has already served requests (performance.md PERF-1).
func BenchmarkExprSandboxWarm(b *testing.B) {
	for _, mode := range []wazero.Mode{wazero.ModeHardLimit, wazero.ModeCooperativeLimit} {
		b.Run(mode.String(), func(b *testing.B) {
			ev := evaluatorIn(b, mode, expr.Options{})
			for _, c := range sandboxCases(b) {
				prog := mustCompile2(b, ev, c.Template)
				env, err := c.Env()
				if err != nil {
					b.Fatal(err)
				}
				b.Run(c.Name, func(b *testing.B) {
					s := ev.Open()
					defer func() { _ = s.Close(b.Context()) }()
					if _, err := s.Eval(b.Context(), prog, env); err != nil { // open the runtime outside the loop
						b.Fatal(err)
					}
					b.ReportAllocs()
					for b.Loop() {
						if _, err := s.Eval(b.Context(), prog, env); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkExprSandboxCold measures the first evaluation of a new session:
// opening a runtime from the snapshot and running one expression.
func BenchmarkExprSandboxCold(b *testing.B) {
	for _, mode := range []wazero.Mode{wazero.ModeHardLimit, wazero.ModeCooperativeLimit} {
		b.Run(mode.String(), func(b *testing.B) {
			ev := evaluatorIn(b, mode, expr.Options{})
			c := sandboxCases(b)[0]
			prog := mustCompile2(b, ev, c.Template)
			env, err := c.Env()
			if err != nil {
				b.Fatal(err)
			}
			warm := ev.Open() // builds the snapshot outside the loop
			if _, err := warm.Eval(b.Context(), prog, env); err != nil {
				b.Fatal(err)
			}
			_ = warm.Close(b.Context())
			b.ReportAllocs()
			for b.Loop() {
				s := ev.Open()
				if _, err := s.Eval(b.Context(), prog, env); err != nil {
					b.Fatal(err)
				}
				_ = s.Close(b.Context())
			}
		})
	}
}

func mustCompile2(tb testing.TB, ev *expr.Evaluator, src string) *expr.Program {
	tb.Helper()
	p, err := ev.Compile(src)
	if err != nil {
		tb.Fatal(err)
	}
	return p
}
