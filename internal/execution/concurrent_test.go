// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"reflect"
	"sync"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// TestOnePlanRunsConcurrently runs the same plan and interpreter from several
// goroutines. A Plan is immutable and an Interpreter holds no run state, so the
// runs are independent: every one produces the trace of a lone run. The race
// detector checks that nothing is shared.
func TestOnePlanRunsConcurrently(t *testing.T) {
	t.Parallel()
	b := executiontest.NewFlow("shared").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
	plan := executiontest.MustCompile(t, b)
	in := execution.NewInterpreter(execution.Deps{Runner: &executiontest.TraceRunner{}})
	input := executiontest.Batch(`{"id":1}`)
	want, err := in.Run(t.Context(), plan, execution.Request{Execution: "same", Input: input})
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}
	const runs = 16
	results := make([]execution.Result, runs)
	errs := make([]error, runs)
	var wg sync.WaitGroup
	for i := range runs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = in.Run(t.Context(), plan, execution.Request{Execution: "same", Input: input})
		}()
	}
	wg.Wait()
	for i := range runs {
		if errs[i] != nil {
			t.Fatalf("run %d: %v", i, errs[i])
		}
		if !reflect.DeepEqual(want.Events, results[i].Events) {
			t.Errorf("run %d produced another trace than a lone run", i)
		}
	}
}
