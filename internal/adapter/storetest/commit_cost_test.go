// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"os"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
)

// TestCommitCost measures the cost of the commit of the engine state of an execution: SaveEngineState in
// the transaction of the engine. It is a measurement, not a check, so it is off unless
// ORCON_COMMIT_COST_N is set to the number of commits per case (for example 2000), and it logs the
// median and the 95th percentile of one commit and the throughput of eight concurrent writers whose
// executions were accepted in the same hour (the case that makes the commits share one counter row).
// Run it on each store of -args -store=both and compare the numbers with and without the hourly counters.
func TestCommitCost(t *testing.T) {
	raw := os.Getenv("ORCON_COMMIT_COST_N")
	if raw == "" {
		t.Skip("set ORCON_COMMIT_COST_N to measure the cost of a commit")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 10 {
		t.Fatalf("ORCON_COMMIT_COST_N = %q, want a number of at least 10", raw)
	}
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		rec := f.flow(f.orgA, f.projA, "commit-cost")
		ver := flow.VersionRecord{
			ID: next[flow.FlowVersionID](f), OrgID: rec.OrgID, FlowID: rec.ID, Number: 1,
			Document: []byte(`{"version":1}`), ContentHash: "0000000000000000000000000000000000000000000000000000000000000001", PublishedAt: base, CreatedAt: base,
		}
		if err := f.s.PublishVersion(f.ctx(), scopeOf(rec.OrgID), ver); err != nil {
			t.Fatalf("publish version: %v", err)
		}
		accepted := base.Add(-time.Hour)
		running := func(count int) []store.ExecutionID {
			ids := make([]store.ExecutionID, 0, count)
			for range count {
				ex := store.Execution{
					ID: next[store.ExecutionID](f), OrgID: rec.OrgID, ProjectID: rec.ProjectID, FlowID: rec.ID, FlowVersionID: ver.ID,
					Status: "RUNNING", Mode: "TRIGGER", CreatedAt: accepted, UpdatedAt: accepted,
				}
				ex.RootExecutionID = ex.ID
				if err := f.s.InTx(f.ctx(), scopeOf(rec.OrgID), func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &ex) }); err != nil {
					t.Fatalf("insert execution: %v", err)
				}
				ids = append(ids, ex.ID)
			}
			return ids
		}
		commit := func(exec store.ExecutionID, status string) error {
			return f.s.InTx(f.ctx(), scopeOf(rec.OrgID), func(ctx context.Context, tx store.Tx) error {
				st, err := tx.LockExecution(ctx, rec.OrgID, exec)
				if err != nil {
					return err
				}
				return tx.SaveEngineState(ctx, store.EngineStateUpdate{
					OrgID: rec.OrgID, Execution: exec, ExpectedRevision: st.Revision, Status: status, FinishedAt: base, Now: base,
				})
			})
		}
		measure := func(name string, ids []store.ExecutionID, status string) {
			durations := make([]time.Duration, 0, len(ids))
			for _, exec := range ids {
				start := time.Now()
				if err := commit(exec, status); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				durations = append(durations, time.Since(start))
			}
			slices.Sort(durations)
			t.Logf("%-26s n=%d median=%v p95=%v", name, len(durations), durations[len(durations)/2], durations[len(durations)*95/100])
		}

		ids := running(n)
		measure("commit without a status move", ids, "RUNNING")
		measure("commit to a final status", ids, "SUCCEEDED")

		const writers = 8
		ids = running(n)
		var wg sync.WaitGroup
		start := time.Now()
		for w := range writers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := w; i < len(ids); i += writers {
					if err := commit(ids[i], "SUCCEEDED"); err != nil {
						t.Errorf("concurrent commit: %v", err)
						return
					}
				}
			}()
		}
		wg.Wait()
		elapsed := time.Since(start)
		t.Logf("%-26s n=%d writers=%d %v (%.0f commits/s)", "concurrent final commits", len(ids), writers, elapsed, float64(len(ids))/elapsed.Seconds())
	})
}
