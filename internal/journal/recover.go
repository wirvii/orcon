// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package journal

import (
	"context"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Recover settles the operations of one node execution that were left in
// DISPATCHING by a worker that stopped (ADR-0012 section 6). Only items whose
// dispatch deadline plus the grace period has passed, by the database clock, are
// touched, so a live worker is never raced.
//
// For each such item, in order:
//   - a provider key younger than its TTL is resent with the same key;
//   - a reconcilable contract is looked up by correlation key;
//   - anything else becomes UNCERTAIN, and nothing is sent.
//
// A resend that fails with a transport error keeps the item in DISPATCHING for
// the next recovery.
//
// The operations are read page by page in (item index, id) order. The cursor is
// the full key of the last row, so operations that share an item index (one per
// retry epoch) are never skipped at a page boundary.
func (j *Journal) Recover(ctx context.Context, org orgscope.Org, node store.NodeExecutionID, c Contract) (Summary, error) {
	var sum Summary
	var fromItem int64
	var fromID store.ExternalOperationID
	for {
		rows, err := j.db.ListExternalOperations(ctx, org, node, fromItem, fromID, j.page)
		if err != nil {
			return sum, fmt.Errorf("journal: list operations: %w", err)
		}
		if len(rows) == 0 {
			return sum, nil
		}
		now, err := j.databaseNow(ctx, org)
		if err != nil {
			return sum, err
		}
		for i := range rows {
			if rows[i].State != StateDispatching {
				continue
			}
			if err := j.settle(ctx, org, rows[i], c, now, &sum); err != nil {
				return sum, err
			}
		}
		if len(rows) < j.page {
			return sum, nil
		}
		last := rows[len(rows)-1]
		fromItem, fromID = last.ItemIndex, last.ID
	}
}

// databaseNow reads the database clock, the one clock that decides recovery
// windows across processes (ADR-0012 section 6, ADR-0013 section 4).
func (j *Journal) databaseNow(ctx context.Context, org orgscope.Org) (time.Time, error) {
	var now time.Time
	err := j.db.InTx(ctx, org, func(ctx context.Context, tx store.Tx) error {
		var err error
		now, err = tx.DatabaseNow(ctx)
		return err
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("journal: database clock: %w", err)
	}
	return now, nil
}

// settle decides one stale DISPATCHING item; now is the database clock.
func (j *Journal) settle(ctx context.Context, org orgscope.Org, op store.ExternalOperation, c Contract, now time.Time, sum *Summary) error {
	if !op.DispatchDeadlineAt.IsZero() && now.Before(op.DispatchDeadlineAt.Add(j.cfg.Grace)) {
		sum.Pending++
		return nil
	}
	attempt := &Attempt{Operation: op, Contract: c}
	var reply Reply
	var err error
	switch {
	case c.Idempotency == IdempotencyProviderKey && keyAlive(op, c, now):
		reply, err = j.provider.Send(ctx, attempt)
	case c.Reconcilable:
		reply = j.reconcile(ctx, attempt)
	default:
		reply = Reply{Outcome: StateUncertain}
	}
	o := classify(op.ID, c, reply, err)
	if o == nil {
		sum.Pending++
		return nil
	}
	applied, err := j.record(ctx, org, []outcome{*o}, now, sum)
	if applied == 0 {
		sum.Pending++
	}
	return err
}

// reconcile asks the provider whether the operation took effect. A failed
// lookup says nothing, so the operation becomes UNCERTAIN.
func (j *Journal) reconcile(ctx context.Context, attempt *Attempt) Reply {
	reply, err := j.provider.Lookup(ctx, attempt)
	if err != nil {
		return Reply{Outcome: StateUncertain}
	}
	return reply
}

// keyAlive reports whether the provider still deduplicates the key of op. The
// age counts from the first dispatch, by the database clock; a TTL of zero
// means no key ever expires.
func keyAlive(op store.ExternalOperation, c Contract, now time.Time) bool {
	if c.KeyTTL <= 0 {
		return true
	}
	return now.Sub(op.FirstDispatchedAt) < c.KeyTTL
}
