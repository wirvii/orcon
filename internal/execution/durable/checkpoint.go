// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
)

// checkpointOf returns the checkpoint of the segment a worker runs for one token
// (ADR-0013 section 6). A node that acts on an external system calls it before
// the first effect: one transaction locks the execution, checks the lease fence,
// records the node execution as RUNNING, and runs the node's writes (the journal
// intent, ADR-0012 section 4). A crash after the commit leaves the RUNNING row
// and the intent for the next worker to find; the commit of the activation later
// completes the same row.
func (e *Engine) checkpointOf(org identity.OrgID, exec store.ExecutionID, fence store.Fence) execution.ActivationCheckpoint {
	return func(ctx context.Context, call execution.NodeCall, fn func(ctx context.Context, tx any) error) error {
		neID, err := nodeExecutionID(call.NodeExecution)
		if err != nil {
			return err
		}
		tid, err := tokenID(call.Token)
		if err != nil {
			return err
		}
		return e.inTx(ctx, org, func(ctx context.Context, tx store.Tx) error {
			now := e.cfg.Clock.Now()
			if _, err := tx.LockExecution(ctx, org, exec); err != nil {
				return fmt.Errorf("lock execution %s: %w", exec, err)
			}
			if err := tx.AssertFence(ctx, fence, now); err != nil {
				return err
			}
			if err := tx.UpsertNodeExecution(ctx, store.NodeExecution{
				ID: neID, OrgID: org, ExecutionID: exec, TokenID: tid, NodeID: string(call.Node.ID), ScopePath: string(call.Scope),
				Status: string(execution.NodeRunning), StartedAt: now, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("record node execution %s as running: %w", call.NodeExecution, err)
			}
			return fn(ctx, tx)
		})
	}
}
