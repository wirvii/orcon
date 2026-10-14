-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Work that crosses organizations. On PostgreSQL these queries call the
-- orcon_engine functions; on SQLite the single writer performs the same
-- operation in one transaction, and the caller supplies the instants.

-- name: ClaimTokens :many
-- orcon:scope engine
-- orcon:hot
UPDATE token
SET status = 'LEASED', lease_owner = sqlc.arg(worker_id), lease_epoch = lease_epoch + 1, lease_expires_at = sqlc.arg(lease_expires_at),
    claim_count = claim_count + 1, updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT t.id FROM token t
  WHERE t.status = 'RUNNABLE' AND t.available_at <= sqlc.arg(now)
    AND (sqlc.arg(any_fairness_key) = 1 OR t.fairness_key IN (sqlc.slice(fairness_keys)))
  ORDER BY t.effective_priority, t.available_at, t.id
  LIMIT sqlc.arg(page_size))
RETURNING org_id, id, execution_id, node_id, edge_id, scope_path, fork_lineage_document, payload_reference, fairness_key,
          lease_epoch, lease_expires_at;

-- name: RenewLeases :many
-- orcon:scope engine
-- orcon:hot
UPDATE token
SET lease_expires_at = sqlc.arg(lease_expires_at), updated_at = sqlc.arg(now)
WHERE lease_owner = sqlc.arg(worker_id) AND status = 'LEASED' AND lease_expires_at > sqlc.arg(now)
RETURNING org_id, id, lease_epoch,
          CAST(EXISTS (SELECT 1 FROM execution e WHERE e.org_id = token.org_id AND e.id = token.execution_id AND e.cancel_requested_at IS NOT NULL) AS INTEGER) AS is_cancel_requested;

-- name: ReleaseExpiredLeases :many
-- orcon:scope engine
-- orcon:hot
UPDATE token
SET status = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN 'SUSPENDED' ELSE 'RUNNABLE' END,
    suspend_reason = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN 'POISON' ELSE NULL END,
    abandoned_count = token.abandoned_count + 1, lease_owner = NULL, lease_expires_at = NULL, updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT t.id FROM token t
  WHERE t.status = 'LEASED' AND t.lease_expires_at < sqlc.arg(now)
  ORDER BY t.lease_expires_at
  LIMIT sqlc.arg(page_size))
RETURNING token.org_id, token.id, token.execution_id, token.abandoned_count, CAST(token.status = 'SUSPENDED' AS INTEGER) AS is_poisoned;

-- name: MarkTokenAttemptsAbandoned :execrows
-- orcon:scope engine
UPDATE task_attempt
SET status = 'ABANDONED', finished_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE task_attempt.org_id = sqlc.arg(org_id) AND task_attempt.status = 'STARTED'
  AND task_attempt.node_execution_id IN (SELECT n.id FROM node_execution n WHERE n.org_id = sqlc.arg(org_id) AND n.token_id = sqlc.arg(token_id));

-- name: ReleaseWorkerLeases :many
-- orcon:scope engine
-- orcon:hot
UPDATE token
SET status = 'RUNNABLE', lease_owner = NULL, lease_expires_at = NULL, updated_at = sqlc.arg(now)
WHERE lease_owner = sqlc.arg(worker_id) AND status = 'LEASED'
RETURNING org_id, id;

-- Startup of a standalone process: the lock file guarantees no other holder can exist, so every lease is released at
-- once (ADR-0013 section 9).
-- name: ReleaseAllLeases :execrows
-- orcon:scope engine
UPDATE token
SET status = 'RUNNABLE', lease_owner = NULL, lease_expires_at = NULL, updated_at = sqlc.arg(now)
WHERE status = 'LEASED';

-- name: MarkDueTimersFired :many
-- orcon:scope engine
-- orcon:hot
UPDATE timer
SET status = 'FIRED', updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT tm.id FROM timer tm
  WHERE tm.status = 'SCHEDULED' AND tm.due_at <= sqlc.arg(now)
  ORDER BY tm.due_at, tm.id
  LIMIT sqlc.arg(page_size))
RETURNING org_id, id, execution_id, token_id, kind, due_at, payload_document;

-- name: ClaimOutbox :many
-- orcon:scope engine
-- orcon:hot
UPDATE outbox
SET lease_owner = sqlc.arg(worker_id), lease_expires_at = sqlc.arg(lease_expires_at), attempt_count = attempt_count + 1,
    updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT o.id FROM outbox o
  WHERE o.delivered_at IS NULL AND (o.lease_expires_at IS NULL OR o.lease_expires_at < sqlc.arg(now))
  ORDER BY o.created_at, o.id
  LIMIT sqlc.arg(page_size))
RETURNING org_id, id, topic, payload_document, attempt_count;

-- name: ResolveWaitSecret :one
-- orcon:scope engine
-- orcon:hot
SELECT org_id, id, secret_hash, status, expires_at
FROM wait
WHERE id = sqlc.arg(id) AND kind = 'SIGNAL';

-- name: CountJoinArrivals :one
-- orcon:scope org
SELECT count(*) AS arrival_count, count(*) FILTER (WHERE is_live = 1) AS live_count
FROM join_arrival
WHERE org_id = sqlc.arg(org_id) AND execution_id = sqlc.arg(execution_id) AND node_id = sqlc.arg(node_id)
  AND scope_path = sqlc.arg(scope_path) AND round_number = sqlc.arg(round_number);
