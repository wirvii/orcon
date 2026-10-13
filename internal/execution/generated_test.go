// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"encoding/binary"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
)

// checkProgram generates a flow from the bytes, runs it twice and checks the
// properties every structured flow must have:
//   - it compiles and ends in SUCCEEDED;
//   - the run invariants hold (completion signals once per edge and scope,
//     no token left active, node executions unique and finished);
//   - two runs of the same flow and input give the same trace and tokens;
//   - without discriminators, the items at the end event are exactly those the
//     reference model predicts.
func checkProgram(t *testing.T, data []byte, opts executiontest.GenOptions) {
	t.Helper()
	prog := executiontest.Generate(data, opts)
	doc := prog.Flow.Doc()
	plan, err := execution.Compile(&doc)
	if err != nil {
		t.Fatalf("input %x: Compile() = %v", data, err)
	}
	run := func() execution.Result {
		res, err := execution.NewInterpreter(prog.Deps()).Run(t.Context(), plan, execution.Request{Execution: "gen", Input: prog.Input})
		if err != nil {
			t.Fatalf("input %x: Run() = %v", data, err)
		}
		return res
	}
	first := run()
	if first.Status != execution.ExecSucceeded {
		t.Fatalf("input %x: status = %s, failure %v\n%s", data, first.Status, first.Failure, executiontest.Trace(first))
	}
	if v := executiontest.Violations(first); len(v) > 0 {
		t.Fatalf("input %x: invariants broken: %v\n%s", data, v, executiontest.Trace(first))
	}
	second := run()
	// DeepEqual first: cmp.Diff is slow on long traces and only needed to explain.
	if !reflect.DeepEqual(first.Events, second.Events) {
		t.Fatalf("input %x: two runs of the same flow differ (-first +second):\n%s", data, cmp.Diff(first.Events, second.Events))
	}
	if !reflect.DeepEqual(first.Tokens, second.Tokens) {
		t.Fatalf("input %x: tokens of two runs differ (-first +second):\n%s", data, cmp.Diff(first.Tokens, second.Tokens))
	}
	if !prog.HasReference {
		return
	}
	if diff := cmp.Diff(prog.Want, executiontest.Paths(first)); diff != "" {
		t.Fatalf("input %x: end items differ from the reference model (-model +interpreter):\n%s\n%s", data, diff, executiontest.Trace(first))
	}
}

func seeded(seed uint64, size int) []byte {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) //nolint:gosec // G404: deterministic test data, not a secret
	data := make([]byte, 0, size+8)
	for len(data) < size {
		data = binary.LittleEndian.AppendUint64(data, r.Uint64())
	}
	return data[:size]
}

func TestGeneratedFlowsMatchTheReferenceModel(t *testing.T) {
	t.Parallel()
	runs := uint64(600)
	if testing.Short() {
		runs = 100
	}
	for seed := range runs {
		checkProgram(t, seeded(seed, 200), executiontest.GenOptions{})
	}
}

func TestGeneratedFlowsWithDiscriminators(t *testing.T) {
	t.Parallel()
	runs := uint64(400)
	if testing.Short() {
		runs = 80
	}
	for seed := range runs {
		checkProgram(t, seeded(1_000_000+seed, 200), executiontest.GenOptions{Discriminators: true})
	}
}

// FuzzInterpreter feeds arbitrary bytes to the flow generator. Whatever the
// bytes, the generated structured flow must run to SUCCEEDED, deterministically
// and, when it has no discriminator, to the result of the reference model.
func FuzzInterpreter(f *testing.F) {
	for seed := range uint64(8) {
		f.Add(seeded(seed, 120), false)
		f.Add(seeded(100+seed, 120), true)
	}
	f.Add([]byte{}, false)
	f.Add([]byte{1, 99, 99, 99, 99, 99, 99, 99}, true)
	f.Fuzz(func(t *testing.T, data []byte, discriminators bool) {
		checkProgram(t, data, executiontest.GenOptions{Discriminators: discriminators, MaxNodes: 25})
	})
}
