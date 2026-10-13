-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the execution tables that run inside an organization: executions,
-- tokens, the journal of external operations, joins, timers, waits, events, the
-- outbox and the attention inbox. The PostgreSQL file of the same name holds the
-- same text; the queries of the engine plane are in engine.sql.

-- name: InsertExecution :exec
-- orcon:scope org
INSERT INTO execution (id, org_id, project_id, flow_id, flow_version_id, flow_snapshot_id, status, mode, priority,
                       parent_execution_id, root_execution_id, call_depth, run_generation,
                       runnable_count, leased_count, waiting_count, suspended_count, active_milliseconds,
                       created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(project_id), sqlc.arg(flow_id), sqlc.arg(flow_version_id), sqlc.arg(flow_snapshot_id),
        sqlc.arg(status), sqlc.arg(mode), sqlc.arg(priority), sqlc.arg(parent_execution_id), sqlc.arg(root_execution_id),
        sqlc.arg(call_depth), 0, sqlc.arg(runnable_count), 0, 0, 0, 0, sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetExecution :one
-- orcon:scope org
SELECT id, org_id, project_id, flow_id, flow_version_id, flow_snapshot_id, status, mode, priority, parent_execution_id,
       root_execution_id, call_depth, run_generation, cancel_requested_at, runnable_count, leased_count, waiting_count,
       suspended_count, started_at, finished_at, created_at, updated_at
FROM execution
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: ListExecutions :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, project_id, flow_id, flow_version_id, flow_snapshot_id, status, mode, priority, parent_execution_id,
       root_execution_id, call_depth, run_generation, cancel_requested_at, runnable_count, leased_count, waiting_count,
       suspended_count, started_at, finished_at, created_at, updated_at
FROM execution
WHERE org_id = sqlc.arg(org_id) AND project_id = sqlc.arg(project_id)
  AND (created_at, id) < (sqlc.arg(before_created_at), CAST(sqlc.arg(before_id) AS BLOB))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListFlowExecutions :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, project_id, flow_id, flow_version_id, flow_snapshot_id, status, mode, priority, parent_execution_id,
       root_execution_id, call_depth, run_generation, cancel_requested_at, runnable_count, leased_count, waiting_count,
       suspended_count, started_at, finished_at, created_at, updated_at
FROM execution
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id)
  AND (created_at, id) < (sqlc.arg(before_created_at), CAST(sqlc.arg(before_id) AS BLOB))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListExecutionsByStatus :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, project_id, flow_id, flow_version_id, flow_snapshot_id, status, mode, priority, parent_execution_id,
       root_execution_id, call_depth, run_generation, cancel_requested_at, runnable_count, leased_count, waiting_count,
       suspended_count, started_at, finished_at, created_at, updated_at
FROM execution
WHERE org_id = sqlc.arg(org_id) AND status = sqlc.arg(status)
  AND (created_at, id) < (sqlc.arg(before_created_at), CAST(sqlc.arg(before_id) AS BLOB))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: InsertToken :exec
-- orcon:scope org
INSERT INTO token (id, org_id, execution_id, status, node_id, edge_id, scope_path, fork_lineage_document, fork_lineage_document_format,
                   payload_reference, fairness_key, effective_priority, available_at, lease_owner, lease_epoch, lease_expires_at,
                   claim_count, abandoned_count, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(status), sqlc.arg(node_id), sqlc.arg(edge_id), sqlc.arg(scope_path),
        sqlc.arg(fork_lineage_document), 'orcon/fork-lineage@1', sqlc.arg(payload_reference), sqlc.arg(fairness_key),
        sqlc.arg(effective_priority), sqlc.arg(available_at), sqlc.arg(lease_owner), sqlc.arg(lease_epoch), sqlc.arg(lease_expires_at),
        0, 0, sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetToken :one
-- orcon:scope org
SELECT id, org_id, execution_id, status, node_id, edge_id, scope_path, fork_lineage_document, payload_reference, fairness_key,
       effective_priority, available_at, lease_owner, lease_epoch, lease_expires_at, claim_count, abandoned_count, suspend_reason
FROM token
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- Every write of a lease holder is fenced on (id, lease_epoch, lease_owner, status = LEASED) (ADR-0009 section 2).
-- name: MarkTokenFenced :execrows
-- orcon:scope org
UPDATE token
SET updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND lease_epoch = sqlc.arg(lease_epoch)
  AND lease_owner = sqlc.arg(lease_owner) AND status = 'LEASED';

-- name: UpdateTokenFenced :execrows
-- orcon:scope org
UPDATE token
SET status = sqlc.arg(status), lease_owner = NULL, lease_expires_at = NULL, updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND lease_epoch = sqlc.arg(lease_epoch)
  AND lease_owner = sqlc.arg(lease_owner) AND status = 'LEASED';

-- name: InsertNodeExecution :exec
-- orcon:scope org
INSERT INTO node_execution (id, org_id, execution_id, token_id, node_id, scope_path, status, attempt_count, started_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(token_id), sqlc.arg(node_id), sqlc.arg(scope_path),
        sqlc.arg(status), 0, sqlc.arg(started_at), sqlc.arg(created_at), sqlc.arg(updated_at));

-- The sequence number is one more than the newest of the execution, assigned in the transaction that writes the
-- event (orcon-db ID-6); the caller holds the execution row (single writer on SQLite, LockExecution on PostgreSQL).
-- name: AppendExecutionEvent :one
-- orcon:scope org
-- orcon:hot
INSERT INTO execution_event (id, org_id, execution_id, root_execution_id, sequence_number, occurred_at, type, schema_version,
                             reference_document, reference_document_format, data_document, data_document_format, created_at)
SELECT sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(root_execution_id),
       COALESCE(MAX(sequence_number), 0) + 1, sqlc.arg(occurred_at), sqlc.arg(type), sqlc.arg(schema_version),
       sqlc.arg(reference_document), 'orcon/event-reference@1', sqlc.arg(data_document), sqlc.arg(data_document_format), sqlc.arg(created_at)
FROM execution_event
WHERE org_id = sqlc.arg(org_id) AND execution_id = sqlc.arg(execution_id)
RETURNING sequence_number;

-- name: ListExecutionEvents :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, execution_id, root_execution_id, sequence_number, occurred_at, type, schema_version, reference_document, data_document
FROM execution_event
WHERE org_id = sqlc.arg(org_id) AND execution_id = sqlc.arg(execution_id) AND sequence_number > sqlc.arg(after_sequence_number)
ORDER BY sequence_number
LIMIT sqlc.arg(page_size);

-- name: InsertExternalOperation :exec
-- orcon:scope org
-- orcon:hot
INSERT INTO external_operation (id, org_id, execution_id, node_execution_id, item_index, correlation_key, contract_reference, state,
                                batch_key, retry_epoch, compensation_state, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(node_execution_id), sqlc.arg(item_index),
        sqlc.arg(correlation_key), sqlc.arg(contract_reference), 'PREPARED', sqlc.arg(batch_key), 0, 'NONE',
        sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: MarkExternalOperationDispatching :execrows
-- orcon:scope org
-- orcon:hot
UPDATE external_operation
SET state = 'DISPATCHING', dispatch_deadline_at = sqlc.arg(dispatch_deadline_at),
    first_dispatched_at = COALESCE(first_dispatched_at, sqlc.arg(dispatched_at)), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND state = 'PREPARED';

-- name: MarkExternalOperationOutcome :execrows
-- orcon:scope org
-- orcon:hot
UPDATE external_operation
SET state = sqlc.arg(next_state), response_reference = sqlc.arg(response_reference), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND state = sqlc.arg(expected_state);

-- name: GetExternalOperation :one
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, execution_id, node_execution_id, item_index, correlation_key, contract_reference, state, batch_key,
       dispatch_deadline_at, first_dispatched_at, retry_epoch, response_reference
FROM external_operation
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: ListExternalOperations :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, execution_id, node_execution_id, item_index, correlation_key, contract_reference, state, batch_key,
       dispatch_deadline_at, first_dispatched_at, retry_epoch, response_reference
FROM external_operation
WHERE org_id = sqlc.arg(org_id) AND node_execution_id = sqlc.arg(node_execution_id) AND item_index >= sqlc.arg(from_item_index)
ORDER BY item_index, id
LIMIT sqlc.arg(page_size);

-- name: InsertJoinInstance :execrows
-- orcon:scope org
INSERT INTO join_instance (id, org_id, execution_id, node_id, scope_path, round_number, split_node_execution_id,
                           edge_state_document, edge_state_document_format, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(node_id), sqlc.arg(scope_path), sqlc.arg(round_number),
        sqlc.arg(split_node_execution_id), sqlc.arg(edge_state_document), 'orcon/edge-state@1', 'OPEN', sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (org_id, execution_id, node_id, scope_path, round_number) DO NOTHING;

-- name: InsertJoinArrival :execrows
-- orcon:scope org
INSERT INTO join_arrival (id, org_id, execution_id, node_id, scope_path, round_number, edge_index, token_id, payload_reference,
                          is_live, arrived_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(node_id), sqlc.arg(scope_path), sqlc.arg(round_number),
        sqlc.arg(edge_index), sqlc.arg(token_id), sqlc.arg(payload_reference), sqlc.arg(is_live), sqlc.arg(arrived_at),
        sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (org_id, execution_id, node_id, scope_path, round_number, edge_index) DO NOTHING;

-- name: MarkJoinFired :execrows
-- orcon:scope org
UPDATE join_instance
SET status = 'FIRED', continuation_token_id = sqlc.arg(continuation_token_id), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND execution_id = sqlc.arg(execution_id) AND node_id = sqlc.arg(node_id)
  AND scope_path = sqlc.arg(scope_path) AND round_number = sqlc.arg(round_number) AND status = 'OPEN';

-- name: GetJoinInstance :one
-- orcon:scope org
SELECT id, org_id, execution_id, node_id, scope_path, round_number, status, continuation_token_id
FROM join_instance
WHERE org_id = sqlc.arg(org_id) AND execution_id = sqlc.arg(execution_id) AND node_id = sqlc.arg(node_id)
  AND scope_path = sqlc.arg(scope_path) AND round_number = sqlc.arg(round_number);

-- name: InsertTimer :exec
-- orcon:scope org
INSERT INTO timer (id, org_id, execution_id, token_id, kind, due_at, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(token_id), sqlc.arg(kind), sqlc.arg(due_at), 'SCHEDULED',
        sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetTimer :one
-- orcon:scope org
SELECT id, org_id, execution_id, token_id, kind, due_at, status
FROM timer
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: InsertWait :exec
-- orcon:scope org
INSERT INTO wait (id, org_id, execution_id, token_id, node_execution_id, kind, wait_group_key, secret_hash, expires_at, status,
                  created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(token_id), sqlc.arg(node_execution_id), sqlc.arg(kind),
        sqlc.arg(wait_group_key), sqlc.arg(secret_hash), sqlc.arg(expires_at), 'OPEN', sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: InsertOutbox :exec
-- orcon:scope org
INSERT INTO outbox (id, org_id, topic, payload_document, payload_document_format, attempt_count, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(topic), sqlc.arg(payload_document), 'orcon/outbox-payload@1', 0,
        sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: MarkOutboxDelivered :execrows
-- orcon:scope org
UPDATE outbox
SET delivered_at = sqlc.arg(delivered_at), lease_owner = NULL, lease_expires_at = NULL, updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND lease_owner = sqlc.arg(lease_owner) AND delivered_at IS NULL;

-- name: InsertAttentionRequest :exec
-- orcon:scope org
INSERT INTO attention_request (id, org_id, execution_id, project_id, flow_id, node_id, token_id, cause, cause_reference, notify_class,
                               status, level, group_key, item_count, summary, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(project_id), sqlc.arg(flow_id), sqlc.arg(node_id),
        sqlc.arg(token_id), sqlc.arg(cause), sqlc.arg(cause_reference), 'NOTIFY', 'OPEN', 0, sqlc.arg(group_key),
        sqlc.arg(item_count), sqlc.arg(summary), sqlc.arg(created_at), sqlc.arg(updated_at));

-- The inbox: requests in one status, oldest first, keyset by (created_at, id).
-- name: ListAttentionRequests :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, execution_id, project_id, flow_id, node_id, token_id, cause, cause_reference, notify_class, status, level,
       group_key, item_count, summary, created_at, updated_at
FROM attention_request
WHERE org_id = sqlc.arg(org_id) AND status = sqlc.arg(status)
  AND (created_at, id) > (sqlc.arg(after_created_at), CAST(sqlc.arg(after_id) AS BLOB))
ORDER BY created_at, id
LIMIT sqlc.arg(page_size);
