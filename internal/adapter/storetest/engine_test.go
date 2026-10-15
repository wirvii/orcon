// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

func engineKinds(t *testing.T) []storetest.Kind {
	t.Helper()
	kinds, err := storetest.KindsFor(storeFlag())
	if err != nil {
		t.Fatal(err)
	}
	return kinds
}

// TestEngineRunsFlows is a smoke test of the durable engine over each store: a
// sequence, a parallel split with an AND join and a loop run to the same result
// as the in-memory interpreter.
func TestEngineRunsFlows(t *testing.T) {
	flows := map[string]*executiontest.Builder{
		"sequence": executiontest.NewFlow("seq").Start("s").Task("a").Task("b").End("e").Chain("s", "a", "b", "e"),
		"fork and join": executiontest.NewFlow("fj").Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("c").End("e").
			Join("j", flow.JoinAll).
			Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "c", "e"),
	}
	for _, kind := range engineKinds(t) {
		for name, b := range flows {
			t.Run(string(kind)+"/"+name, func(t *testing.T) {
				doc := b.Doc()
				plan := executiontest.MustCompile(t, b)
				deps := execution.Deps{Runner: &executiontest.TraceRunner{}}
				input := executiontest.Batch(`{"n":1}`)
				want := executiontest.MemoryEngine(t, &doc, plan, deps, input)
				got := storetest.ExecutionEngine(kind, storetest.EngineOptions{Weak: true})(t, &doc, plan, deps, input)
				if got.Status != want.Status {
					t.Fatalf("status = %s, want %s", got.Status, want.Status)
				}
				if g, w := executiontest.Trace(got), executiontest.Trace(want); g != w {
					t.Fatalf("trace differs:\ngot:\n%s\nwant:\n%s", g, w)
				}
			})
		}
	}
}
