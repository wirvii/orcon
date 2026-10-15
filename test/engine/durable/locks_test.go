// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/store"
)

// TestRecoveryLockOrder (BL-067): a worker commit locks the execution and then its
// tokens; the recovery functions must take the same order. With a token locked
// before the execution the reaper and a commit wait for each other and PostgreSQL
// aborts one of them (SQLSTATE 40P01). The commit below holds the execution lock,
// the recovery call starts and has to wait on the execution, and the commit then
// reaches for the token: both must finish without error.
func TestRecoveryLockOrder(t *testing.T) {
	recoveries := map[string]func(env *storetest.EngineEnv, worker execution.WorkerID) error{
		"reaper": func(env *storetest.EngineEnv, _ execution.WorkerID) error {
			_, err := env.Store.ReleaseExpiredLeases(context.WithoutCancel(env.T.Context()), 10, 3, time.Now())
			return err
		},
		"worker release": func(env *storetest.EngineEnv, worker execution.WorkerID) error {
			_, err := env.Store.ReleaseWorkerLeases(context.WithoutCancel(env.T.Context()), worker, 3, time.Now())
			return err
		},
	}
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		for name, recover := range recoveries {
			t.Run(name, func(t *testing.T) {
				ctx := t.Context()
				pub := env.Publish(docOf(sequence(2)))
				worker := execution.WorkerID(env.IDs.New())
				eng := env.Engine(durable.Config{Worker: worker, Concurrency: 1, LeaseTTL: 100 * time.Millisecond}, runner())
				env.Start(eng, pub, input())
				claimed, _, err := eng.Claim(ctx, 1)
				if err != nil || len(claimed) != 1 {
					t.Fatalf("claim = %d, %v", len(claimed), err)
				}
				c := claimed[0]
				time.Sleep(300 * time.Millisecond) // the lease has expired and nobody reaped it yet
				recovered := make(chan error, 1)
				err = env.Store.InTx(ctx, storetest.ScopeOf(env.Org), func(ctx context.Context, tx store.Tx) error {
					if _, err := tx.LockExecution(ctx, env.Org, c.ExecutionID); err != nil { // what a commit does first
						return err
					}
					go func() { recovered <- recover(env, worker) }()
					time.Sleep(500 * time.Millisecond) // the recovery is running now
					f := store.Fence{OrgID: env.Org, Token: c.TokenID, Epoch: c.LeaseEpoch, Owner: worker}
					return tx.AssertFence(ctx, f, time.Now()) // what a commit does second
				})
				if err != nil {
					t.Errorf("the commit: %v", err)
				}
				if err := <-recovered; err != nil {
					t.Errorf("the recovery: %v", err)
				}
			})
		}
	})
}
