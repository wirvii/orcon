// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
)

// storeFlag selects the stores the engine tests run against, like the -store of the
// other criteria of SPEC-001: sqlite, postgres, both, or auto (SQLite, and
// PostgreSQL when a server is enabled). Memory is not a store: the engine is
// durable by definition.
func storeFlag() string { return executiontest.StoreFlag("auto") }

// quick reports whether the run is the default one of `make test`, with no store
// named: the criteria commands of SPEC-001 name the store (`-args -store=both`) and
// get the full repetitions that E-REC-2 (1000 arrivals) and E-REC-4 (-iterations)
// ask for, while the default run keeps to what -race can do in reasonable time.
func quick() bool { return executiontest.StoreFlag("") == "" }

func iterations() int {
	if quick() {
		return executiontest.Iterations(12)
	}
	return executiontest.Iterations(120)
}

func joinRepeats() int {
	if quick() {
		return executiontest.JoinRepeats(4)
	}
	return executiontest.JoinRepeats(1000)
}

func TestMain(m *testing.M) {
	flag.Parse()
	// An unknown value of -store makes every test fail (eachStore reports it).
	kinds, _ := storetest.KindsFor(storeFlag())
	if slices.Contains(kinds, storetest.PostgreSQL) {
		storetest.EnablePostgres()
		// The container client keeps goroutines of its own for the whole run.
		stop := storetest.StartPostgres()
		defer stop()
		m.Run()
		return
	}
	goleak.VerifyTestMain(m)
}

// eachStore runs fn as a subtest on each selected store, on a fresh database.
func eachStore(t *testing.T, opts storetest.EngineOptions, fn func(t *testing.T, env *storetest.EngineEnv)) {
	t.Helper()
	kinds, err := storetest.KindsFor(storeFlag())
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			fn(t, storetest.NewEngineEnv(t, kind, opts))
		})
	}
}

// faultPlan is the Faults hook of the tests: it counts the points it sees and
// can fail or intercept them.
type faultPlan struct {
	mu   sync.Mutex
	hits map[durable.Point]int
	// failAt fails the nth hit (counted over the points in only) with err.
	failAt int
	only   []durable.Point
	err    error
	seen   int
	// on, when set, runs at every hit before the plan decides.
	on func(ctx context.Context, p durable.Point, token store.TokenID) error
}

func (f *faultPlan) Hit(ctx context.Context, p durable.Point, token store.TokenID) error {
	f.mu.Lock()
	if f.hits == nil {
		f.hits = make(map[durable.Point]int)
	}
	f.hits[p]++
	counted := len(f.only) == 0 || slices.Contains(f.only, p)
	fail := false
	if counted {
		f.seen++
		fail = f.failAt > 0 && f.seen == f.failAt
	}
	hook, err := f.on, f.err
	f.mu.Unlock()
	if hook != nil {
		if e := hook(ctx, p, token); e != nil {
			return e
		}
	}
	if fail {
		return err
	}
	return nil
}

func (f *faultPlan) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seen
}

// runner returns the plain runner of the tests: it passes the items through
// appending the node id to their path.
func runner() execution.Deps { return execution.Deps{Runner: &executiontest.TraceRunner{}} }

// sequence builds a flow of a start event, n tasks one after the other, and an end event.
func sequence(n int) *executiontest.Builder {
	b := executiontest.NewFlow("sequence").Start("s")
	ids := []string{"s"}
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("t%d", i)
		b.Task(id)
		ids = append(ids, id)
	}
	b.End("e")
	ids = append(ids, "e")
	return b.Chain(ids...)
}

// forkJoin builds a flow that splits into n branches and joins them with an AND join.
func forkJoin(n int) *executiontest.Builder {
	b := executiontest.NewFlow("fork join").Start("s").Parallel("p").Parallel("j").Task("after").End("e").Join("j", flow.JoinAll)
	b.Chain("s", "p")
	for i := range n {
		id := fmt.Sprintf("b%d", i)
		b.Task(id).Edge("p", id).Edge(id, "j")
	}
	return b.Chain("j", "after", "e")
}

// waitFor polls until cond holds or the deadline passes.
func waitFor(t *testing.T, what string, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for %s", within, what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// statusOf reads the status of an execution.
func statusOf(env *storetest.EngineEnv, execID store.ExecutionID) execution.State {
	ex, err := env.Store.GetExecution(env.T.Context(), env.Org, execID)
	if err != nil {
		return ""
	}
	return execution.State(ex.Status)
}

// countEvents counts the stored events of a type.
func countEvents(env *storetest.EngineEnv, execID store.ExecutionID, typ string) int {
	n := 0
	events := env.Events(execID)
	for i := range events {
		if events[i].Type == typ {
			n++
		}
	}
	return n
}
