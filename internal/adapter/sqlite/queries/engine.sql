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
  -- The execution cap applies inside the selection, before the page limit: a token is a candidate only while its
  -- execution has room for it, counting the tokens already leased and the candidates ahead of it in the same claim
  -- (all the tokens of an execution share one fairness key, so the key filter needs no repeat in the inner count).
  SELECT t.id FROM token t
  WHERE t.status = 'RUNNABLE' AND t.available_at <= sqlc.arg(now)
    AND (sqlc.arg(any_fairness_key) = 1 OR t.fairness_key IN (sqlc.slice(fairness_keys)))
    AND t.abandoned_count < sqlc.arg(poison_threshold) - 1
    AND (SELECT count(*) FROM token l WHERE l.execution_id = t.execution_id AND l.status = 'LEASED')
      + (SELECT count(*) FROM token a
         WHERE a.execution_id = t.execution_id AND a.status = 'RUNNABLE' AND a.available_at <= sqlc.arg(now)
           AND a.abandoned_count < sqlc.arg(poison_threshold) - 1
           AND (a.effective_priority < t.effective_priority
                OR (a.effective_priority = t.effective_priority
                    AND (a.available_at < t.available_at OR (a.available_at = t.available_at AND a.id < t.id)))))
      < CAST(sqlc.arg(execution_limit) AS INTEGER)
  ORDER BY t.effective_priority, t.available_at, t.id
  LIMIT sqlc.arg(page_size))
RETURNING org_id, id, execution_id, node_id, edge_id, scope_path, fork_lineage_document, payload_reference, fairness_key,
          lease_epoch, lease_expires_at;

-- A claim of one token needs no count of the candidates ahead of it: the cap is only the leased tokens of the
-- execution, and the executions at their cap are found once per claim from the leased tokens (few), so the cost does not grow with the
-- runnable tokens of an execution already at its cap. ClaimTokens is for claims of more than one token.
-- name: ClaimOneToken :many
-- orcon:scope engine
-- orcon:hot
UPDATE token
SET status = 'LEASED', lease_owner = sqlc.arg(worker_id), lease_epoch = lease_epoch + 1, lease_expires_at = sqlc.arg(lease_expires_at),
    claim_count = claim_count + 1, updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT t.id FROM token t
  WHERE t.status = 'RUNNABLE' AND t.available_at <= sqlc.arg(now)
    AND (sqlc.arg(any_fairness_key) = 1 OR t.fairness_key IN (sqlc.slice(fairness_keys)))
    AND t.abandoned_count < sqlc.arg(poison_threshold) - 1
    AND (t.org_id, t.execution_id) NOT IN (
      SELECT l.org_id, l.execution_id FROM token l WHERE l.status = 'LEASED'
      GROUP BY l.org_id, l.execution_id HAVING count(*) >= CAST(sqlc.arg(execution_limit) AS INTEGER))
  ORDER BY t.effective_priority, t.available_at, t.id
  LIMIT 1)
RETURNING org_id, id, execution_id, node_id, edge_id, scope_path, fork_lineage_document, payload_reference, fairness_key,
          lease_epoch, lease_expires_at;

-- The token one abandonment short of the poison threshold runs alone in its process (SPEC-001 Enmienda 12 section 12-1).
-- name: ClaimLastChanceToken :many
-- orcon:scope engine
-- orcon:hot
UPDATE token
SET status = 'LEASED', lease_owner = sqlc.arg(worker_id), lease_epoch = lease_epoch + 1, lease_expires_at = sqlc.arg(lease_expires_at),
    claim_count = claim_count + 1, updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT t.id FROM token t
  WHERE t.status = 'RUNNABLE' AND t.available_at <= sqlc.arg(now)
    AND t.abandoned_count > 0 AND t.abandoned_count = sqlc.arg(poison_threshold) - 1
  ORDER BY t.effective_priority, t.available_at, t.id
  LIMIT 1)
RETURNING org_id, id, execution_id, node_id, edge_id, scope_path, fork_lineage_document, payload_reference, fairness_key,
          lease_epoch, lease_expires_at;

-- name: GetOldestLastChance :one
-- orcon:scope engine
SELECT min(t.available_at) AS since
FROM token t
WHERE t.status = 'RUNNABLE' AND t.available_at <= sqlc.arg(now)
  AND t.abandoned_count > 0 AND t.abandoned_count = sqlc.arg(poison_threshold) - 1;

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
    abandoned_count = token.abandoned_count + 1, lease_owner = NULL, lease_expires_at = NULL,
    available_at = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN token.available_at ELSE sqlc.arg(now) END,
    updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT t.id FROM token t
  WHERE t.status = 'LEASED' AND t.lease_expires_at < sqlc.arg(now)
  ORDER BY t.lease_expires_at
  LIMIT sqlc.arg(page_size))
RETURNING token.org_id, token.id, token.execution_id, token.abandoned_count, token.lease_epoch, token.node_id,
          CAST(token.status = 'SUSPENDED' AS INTEGER) AS is_poisoned;

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
SET status = CASE WHEN sqlc.arg(counts) = 1 AND token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN 'SUSPENDED' ELSE 'RUNNABLE' END,
    suspend_reason = CASE WHEN sqlc.arg(counts) = 1 AND token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN 'POISON' ELSE NULL END,
    abandoned_count = token.abandoned_count + sqlc.arg(counts), lease_owner = NULL, lease_expires_at = NULL,
    available_at = CASE WHEN sqlc.arg(counts) = 1 AND token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN token.available_at ELSE sqlc.arg(now) END,
    updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT t.id FROM token t
  WHERE t.lease_owner = sqlc.arg(worker_id) AND t.status = 'LEASED'
  LIMIT sqlc.arg(page_size))
RETURNING token.org_id, token.id, token.execution_id, token.abandoned_count, token.lease_epoch, token.node_id,
          CAST(token.status = 'SUSPENDED' AS INTEGER) AS is_poisoned;

-- One token the worker could not go on with (ADR-0013 sections 4 and 9): the fence names the epoch and the owner, so
-- a lease that was lost meanwhile is left alone.
-- name: ReleaseTokenLease :many
-- orcon:scope org
UPDATE token
SET status = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN 'SUSPENDED' ELSE 'RUNNABLE' END,
    suspend_reason = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN 'POISON' ELSE NULL END,
    abandoned_count = token.abandoned_count + 1, lease_owner = NULL, lease_expires_at = NULL,
    available_at = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN token.available_at ELSE sqlc.arg(now) END,
    updated_at = sqlc.arg(now)
WHERE token.org_id = sqlc.arg(org_id) AND token.id = sqlc.arg(token_id) AND token.lease_epoch = sqlc.arg(lease_epoch)
  AND token.lease_owner = sqlc.arg(worker_id) AND token.status = 'LEASED'
RETURNING token.org_id, token.id, token.execution_id, token.abandoned_count, token.lease_epoch, token.node_id,
          CAST(token.status = 'SUSPENDED' AS INTEGER) AS is_poisoned;

-- Startup of a standalone process: the lock file guarantees no other holder can exist, so every lease is released at
-- once (ADR-0013 section 9). A lease a crashed predecessor left counts as an abandonment, so that a token that
-- crashes the process every time reaches the poison threshold.
-- name: ReleaseAllLeases :many
-- orcon:scope engine
UPDATE token
SET status = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN 'SUSPENDED' ELSE 'RUNNABLE' END,
    suspend_reason = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN 'POISON' ELSE NULL END,
    abandoned_count = token.abandoned_count + 1, lease_owner = NULL, lease_expires_at = NULL,
    available_at = CASE WHEN token.abandoned_count + 1 >= sqlc.arg(poison_threshold) THEN token.available_at ELSE sqlc.arg(now) END,
    updated_at = sqlc.arg(now)
WHERE id IN (
  SELECT t.id FROM token t
  WHERE t.status = 'LEASED'
  LIMIT sqlc.arg(page_size))
RETURNING token.org_id, token.id, token.execution_id, token.abandoned_count, token.lease_epoch, token.node_id,
          CAST(token.status = 'SUSPENDED' AS INTEGER) AS is_poisoned;

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

-- A timer fired: the token that waits for it runs again. The conditional update leaves a token that moved on alone.
-- name: UpdateWokenToken :execrows
-- orcon:scope org
UPDATE token
SET status = 'RUNNABLE', available_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE token.org_id = sqlc.arg(org_id) AND token.id = sqlc.arg(token_id) AND token.status = 'WAITING';

-- The status an execution takes from its counters after recovery or a wake-up (ADR-0009 section 1).
-- name: UpdateExecutionStatus :execrows
-- orcon:scope org
UPDATE execution
SET status = sqlc.arg(status), updated_at = sqlc.arg(now)
WHERE execution.org_id = sqlc.arg(org_id) AND execution.id = sqlc.arg(id);

-- The first claim of a token of an execution starts it (ADR-0009 section 1: QUEUED to RUNNING when the first token is
-- claimed).
-- name: UpdateStartedExecution :execrows
-- orcon:scope org
UPDATE execution
SET status = 'RUNNING', started_at = COALESCE(execution.started_at, sqlc.arg(now)), updated_at = sqlc.arg(now)
WHERE execution.org_id = sqlc.arg(org_id) AND execution.id = sqlc.arg(id) AND execution.status = 'QUEUED';

-- The claimable tokens grouped by fairness key and priority: the snapshot a worker plans its claims from (ADR-0014 section 1).
-- name: CountRunnableTokens :many
-- orcon:scope engine
-- orcon:hot
SELECT t.fairness_key, t.effective_priority, count(*) AS runnable_count, CAST(min(t.available_at) AS INTEGER) AS oldest_available_at
FROM token t
WHERE t.status = 'RUNNABLE' AND t.available_at <= sqlc.arg(now)
GROUP BY t.fairness_key, t.effective_priority
LIMIT sqlc.arg(row_limit);
