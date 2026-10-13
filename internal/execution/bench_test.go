// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"context"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

func benchRun(b *testing.B, builder *executiontest.Builder, deps execution.Deps, input execution.Batch) {
	b.Helper()
	doc := builder.Doc()
	plan, err := execution.Compile(&doc)
	if err != nil {
		b.Fatalf("Compile() = %v", err)
	}
	in := execution.NewInterpreter(deps)
	b.ReportAllocs()
	for b.Loop() {
		res, err := in.Run(b.Context(), plan, execution.Request{Execution: "bench", Input: input})
		if err != nil || res.Status != execution.ExecSucceeded {
			b.Fatalf("Run() = %s, %v", res.Status, err)
		}
	}
}

// BenchmarkRunSequence measures the step transition: one token through a chain
// of ten activities.
func BenchmarkRunSequence(b *testing.B) {
	builder := executiontest.NewFlow("seq").Start("s").End("e")
	prev := "s"
	for _, id := range []string{"a", "b", "c", "d", "f", "g", "h", "i", "j", "k"} {
		builder.Task(id).Edge(prev, id)
		prev = id
	}
	builder.Edge(prev, "e")
	benchRun(b, builder, execution.Deps{Runner: noopRunner{}}, executiontest.Batch(`{"id":1}`))
}

// BenchmarkRunForkJoin measures a fork into eight branches and an ALL join.
func BenchmarkRunForkJoin(b *testing.B) {
	builder := executiontest.NewFlow("fork").Start("s").Parallel("p").Parallel("j").End("e").Join("j", flow.JoinAll).Edge("s", "p")
	for _, id := range []string{"a", "b", "c", "d", "f", "g", "h", "i"} {
		builder.Task(id).Edge("p", id).Edge(id, "j")
	}
	builder.Edge("j", "e")
	benchRun(b, builder, execution.Deps{Runner: noopRunner{}}, executiontest.Batch(`{"id":1}`))
}

// BenchmarkRunLoop measures a loop of fifty iterations.
func BenchmarkRunLoop(b *testing.B) {
	benchRun(b, counterLoop(100), counterDeps(50), executiontest.Batch(`{"n":0}`))
}

// noopRunner passes the batch on unchanged.
type noopRunner struct{}

func (noopRunner) Execute(_ context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: call.Input}}}, nil
}
