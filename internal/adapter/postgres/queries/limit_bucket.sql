-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the shared rate buckets (limit_bucket, SPEC-001 migration 0006). The PostgreSQL file of the same name
-- holds the same query names. A take locks the bucket row, computes the new state and writes it in one transaction.

-- name: InsertLimitBucketIfAbsent :exec
-- orcon:scope org
INSERT INTO limit_bucket (id, org_id, bucket_key, capacity, refill_per_second, tokens, refilled_at, blocked_until_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(bucket_key), sqlc.arg(capacity), sqlc.arg(refill_per_second), sqlc.arg(tokens), sqlc.arg(refilled_at), NULL, sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (org_id, bucket_key) DO NOTHING;

-- name: LockLimitBucket :one
-- orcon:scope org
SELECT id, bucket_key, capacity, refill_per_second, tokens, refilled_at, blocked_until_at
FROM limit_bucket
WHERE org_id = sqlc.arg(org_id) AND bucket_key = sqlc.arg(bucket_key)
FOR UPDATE;

-- name: UpdateLimitBucketState :exec
-- orcon:scope org
UPDATE limit_bucket
SET tokens = sqlc.arg(tokens), refilled_at = sqlc.arg(refilled_at), blocked_until_at = sqlc.arg(blocked_until_at), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);
