// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package journal

import (
	"context"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
)

// listPage is the page size of the recovery scan.
const listPage = 512

// Recover settles the operations of one node execution that were left in
// DISPATCHING by a worker that stopped (ADR-0012 section 6). Only items whose
// dispatch deadline plus the grace period has passed are touched, so a live
// worker is never raced.
//
// For each such item, in order:
//   - a provider key younger than its TTL is resent with the same key;
//   - a reconcilable contract is looked up by correlation key;
//   - anything else becomes UNCERTAIN, and nothing is sent.
//
// A resend that fails with a transport error keeps the item in DISPATCHING for
// the next recovery.
func (j *Journal) Recover(ctx context.Context, org identity.OrgID, node store.NodeExecutionID, c Contract) (Summary, error) {
	var sum Summary
	from := int64(0)
	for {
		rows, err := j.db.ListExternalOperations(ctx, org, node, from, listPage)
		if err != nil {
			return sum, fmt.Errorf("journal: list operations: %w", err)
		}
		for i := range rows {
			if rows[i].State != StateDispatching {
				continue
			}
			if err := j.settle(ctx, org, rows[i], c, &sum); err != nil {
				return sum, err
			}
		}
		if len(rows) < listPage {
			return sum, nil
		}
		from = rows[len(rows)-1].ItemIndex + 1
	}
}

// settle decides one stale DISPATCHING item.
func (j *Journal) settle(ctx context.Context, org identity.OrgID, op store.ExternalOperation, c Contract, sum *Summary) error {
	now := j.clock.Now()
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
// age counts from the first dispatch; a TTL of zero means no key ever expires.
func keyAlive(op store.ExternalOperation, c Contract, now time.Time) bool {
	if c.KeyTTL <= 0 {
		return true
	}
	return now.Sub(op.FirstDispatchedAt) < c.KeyTTL
}
