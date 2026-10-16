// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/connector/limit"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// bucketID is the deterministic identity of a bucket row.
func bucketID(org identity.OrgID, key string) id.UUID {
	return id.Derive("limit_bucket", org.String(), key)
}

// nullableTimestamp is a nullable instant column: the zero time is NULL.
func nullableTimestamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return timestamp(t)
}

// TakeBucketPermits implements [store.LimitBuckets] in one transaction.
func (s *Store) TakeBucketPermits(ctx context.Context, org identity.OrgID, r store.BucketRequest) (store.BucketVerdict, error) {
	spec := limit.Spec{Capacity: r.Capacity, RefillPerSecond: r.RefillPerSecond}
	if err := spec.Validate(); err != nil {
		return store.BucketVerdict{}, err
	}
	var verdict store.BucketVerdict
	err := s.inOrg(ctx, "take bucket permits", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		bid := bucketID(org, r.Key)
		if err := q.InsertLimitBucketIfAbsent(ctx, sqlcgen.InsertLimitBucketIfAbsentParams{
			ID: pgUUID(bid), OrgID: pgUUID(org), BucketKey: r.Key, Capacity: spec.Capacity, RefillPerSecond: spec.RefillPerSecond,
			Tokens: spec.Capacity, RefilledAt: timestamp(r.Now), CreatedAt: timestamp(r.Now), UpdatedAt: timestamp(r.Now),
		}); err != nil {
			return err
		}
		row, err := q.LockLimitBucket(ctx, pgUUID(org), r.Key)
		if err != nil {
			return err
		}
		st := limit.State{Tokens: row.Tokens, RefilledAt: row.RefilledAt.Time.UTC()}
		if row.BlockedUntilAt.Valid {
			st.BlockedUntil = row.BlockedUntilAt.Time.UTC()
		}
		next, granted, retry, err := limit.Take(st, spec, r.Permits, r.Now)
		if err != nil {
			return err
		}
		verdict = store.BucketVerdict{Granted: granted, RetryAt: retry}
		return q.UpdateLimitBucketState(ctx, sqlcgen.UpdateLimitBucketStateParams{
			Tokens: next.Tokens, RefilledAt: timestamp(next.RefilledAt), BlockedUntilAt: nullableTimestamp(next.BlockedUntil),
			UpdatedAt: timestamp(r.Now), OrgID: pgUUID(org), ID: pgUUID(bid),
		})
	})
	return verdict, err
}

// BlockBucket implements [store.LimitBuckets]. A bucket never taken from has
// nothing to block; the error says so.
func (s *Store) BlockBucket(ctx context.Context, org identity.OrgID, key string, until, now time.Time) error {
	return s.inOrg(ctx, "block bucket", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.LockLimitBucket(ctx, pgUUID(org), key)
		if err != nil {
			return err
		}
		st := limit.State{Tokens: row.Tokens, RefilledAt: row.RefilledAt.Time.UTC()}
		if row.BlockedUntilAt.Valid {
			st.BlockedUntil = row.BlockedUntilAt.Time.UTC()
		}
		st = limit.Block(st, until)
		return q.UpdateLimitBucketState(ctx, sqlcgen.UpdateLimitBucketStateParams{
			Tokens: st.Tokens, RefilledAt: timestamp(st.RefilledAt), BlockedUntilAt: nullableTimestamp(st.BlockedUntil),
			UpdatedAt: timestamp(now), OrgID: pgUUID(org), ID: row.ID,
		})
	})
}

var _ store.LimitBuckets = (*Store)(nil)
