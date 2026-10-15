// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
)

func compareEngines(t *testing.T, label string, plan *execution.Plan, deps execution.Deps, input execution.Batch) {
	t.Helper()
	want := executiontest.MemoryEngine(t, nil, plan, deps, input)
	got := executiontest.SessionEngine(t, nil, plan, deps, input)
	if d := executiontest.Diff(want, got); d != "" {
		t.Fatalf("%s: the session engine differs from the in-memory interpreter:\n%s\nmemory:\n%s\nsession:\n%s", label, d, executiontest.Trace(want), executiontest.Trace(got))
	}
}

// TestSessionEngineMatchesMemory: a run split into sessions that each open from
// the rows the previous one committed gives exactly the run of the in-memory
// interpreter, on generated structured flows (with and without discriminators).
func TestSessionEngineMatchesMemory(t *testing.T) {
	t.Parallel()
	runs := uint64(300)
	if testing.Short() {
		runs = 60
	}
	for seed := range runs {
		for _, opts := range []executiontest.GenOptions{{}, {Discriminators: true}} {
			prog := executiontest.Generate(seeded(5_000_000+seed, 200), opts)
			doc := prog.Flow.Doc()
			plan, err := execution.Compile(&doc)
			if err != nil {
				t.Fatalf("seed %d: Compile() = %v", seed, err)
			}
			compareEngines(t, "generated", plan, prog.Deps(), prog.Input)
		}
	}
}

// TestSessionEngineMatchesMemoryOnRandomGraphs does the same on unstructured
// graphs, where flows may fail halfway through: the failure path cancels
// tokens that other workers may hold, which the session must express in its
// delta as well.
func TestSessionEngineMatchesMemoryOnRandomGraphs(t *testing.T) {
	t.Parallel()
	const graphs = 150
	for seed := range byte(graphs) {
		data := []byte{seed, seed * 3, seed * 5, seed * 7, seed * 11, seed * 13, seed * 17, seed * 19, 255 - seed, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		doc := executiontest.RandomGraph(data).Doc()
		plan, err := execution.Compile(&doc)
		if err != nil {
			t.Fatalf("seed %d: Compile() = %v", seed, err)
		}
		compareEngines(t, "random graph", plan, executiontest.ChaosDeps(&doc), executiontest.Batch(`{"a":1}`, `{"a":2}`))
	}
}
