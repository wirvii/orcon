// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/connector/limit"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// nullableMicros is a nullable instant column: the zero time is NULL.
func nullableMicros(t time.Time) sql.NullInt64 {
	return sql.NullInt64{Int64: micros(t), Valid: !t.IsZero()}
}

// bucketID is the deterministic identity of a bucket row: the same key of the
// same organization always names the same row.
func bucketID(org identity.OrgID, key string) []byte {
	return id.Derive("limit_bucket", org.String(), key).Bytes()
}

// TakeBucketPermits implements [store.LimitBuckets] in one write transaction.
func (s *Store) TakeBucketPermits(ctx context.Context, org identity.OrgID, r store.BucketRequest) (store.BucketVerdict, error) {
	spec := limit.Spec{Capacity: r.Capacity, RefillPerSecond: r.RefillPerSecond}
	if err := spec.Validate(); err != nil {
		return store.BucketVerdict{}, err
	}
	var verdict store.BucketVerdict
	err := s.write(ctx, "take bucket permits", func(ctx context.Context, q *sqlcgen.Queries) error {
		bid := bucketID(org, r.Key)
		if err := q.InsertLimitBucketIfAbsent(ctx, sqlcgen.InsertLimitBucketIfAbsentParams{
			ID: bid, OrgID: uuidBytes(org), BucketKey: r.Key, Capacity: spec.Capacity, RefillPerSecond: spec.RefillPerSecond,
			Tokens: spec.Capacity, RefilledAt: micros(r.Now), CreatedAt: micros(r.Now), UpdatedAt: micros(r.Now),
		}); err != nil {
			return err
		}
		row, err := q.LockLimitBucket(ctx, uuidBytes(org), r.Key)
		if err != nil {
			return err
		}
		st := limit.State{Tokens: row.Tokens, RefilledAt: time.UnixMicro(row.RefilledAt).UTC()}
		if row.BlockedUntilAt.Valid {
			st.BlockedUntil = time.UnixMicro(row.BlockedUntilAt.Int64).UTC()
		}
		next, granted, retry, err := limit.Take(st, spec, r.Permits, r.Now)
		if err != nil {
			return err
		}
		verdict = store.BucketVerdict{Granted: granted, RetryAt: retry}
		return q.UpdateLimitBucketState(ctx, sqlcgen.UpdateLimitBucketStateParams{
			Tokens: next.Tokens, RefilledAt: micros(next.RefilledAt), BlockedUntilAt: nullableMicros(next.BlockedUntil),
			UpdatedAt: micros(r.Now), OrgID: uuidBytes(org), ID: bid,
		})
	})
	return verdict, err
}

// BlockBucket implements [store.LimitBuckets]: the bucket refuses every take
// until the instant. A bucket that was never taken from has nothing to block.
func (s *Store) BlockBucket(ctx context.Context, org identity.OrgID, key string, until, now time.Time) error {
	return s.write(ctx, "block bucket", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.LockLimitBucket(ctx, uuidBytes(org), key)
		if err != nil {
			return err
		}
		st := limit.State{Tokens: row.Tokens, RefilledAt: time.UnixMicro(row.RefilledAt).UTC()}
		if row.BlockedUntilAt.Valid {
			st.BlockedUntil = time.UnixMicro(row.BlockedUntilAt.Int64).UTC()
		}
		st = limit.Block(st, until)
		return q.UpdateLimitBucketState(ctx, sqlcgen.UpdateLimitBucketStateParams{
			Tokens: st.Tokens, RefilledAt: micros(st.RefilledAt), BlockedUntilAt: nullableMicros(st.BlockedUntil),
			UpdatedAt: micros(now), OrgID: uuidBytes(org), ID: row.ID,
		})
	})
}

var _ store.LimitBuckets = (*Store)(nil)
