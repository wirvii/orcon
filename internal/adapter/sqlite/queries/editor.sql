-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Visual flow editor storage (SPEC-014 section 6). The PostgreSQL file of the same name holds the same
-- queries with the same names. Pruning of editor data crosses organizations; PostgreSQL runs it through
-- the orcon_engine function prune_batch, SQLite through the two queries at the end of this file.

-- Node test outputs are append-only (ADR-0082 section 3).
-- name: InsertNodeTestData :exec
-- orcon:scope org
INSERT INTO node_test_data (id, org_id, flow_id, node_id, source_kind, output_document, output_document_format, blob_id,
                            item_count, size_bytes, content_sha256, has_masked_values, created_by_principal_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_id), sqlc.arg(node_id), sqlc.arg(source_kind), sqlc.arg(output_document),
        sqlc.arg(output_document_format), sqlc.arg(blob_id), sqlc.arg(item_count), sqlc.arg(size_bytes), sqlc.arg(content_sha256),
        sqlc.arg(has_masked_values), sqlc.arg(created_by_principal_id), sqlc.arg(created_at));

-- name: GetNodeTestData :one
-- orcon:scope org
SELECT id, org_id, flow_id, node_id, source_kind, output_document, output_document_format, blob_id, item_count, size_bytes,
       content_sha256, has_masked_values, created_by_principal_id, created_at
FROM node_test_data
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- Latest outputs of one node first; the last sort key is unique (id), the page is bounded.
-- name: ListNodeTestDataForNode :many
-- orcon:scope org
SELECT id, org_id, flow_id, node_id, source_kind, output_document, output_document_format, blob_id, item_count, size_bytes,
       content_sha256, has_masked_values, created_by_principal_id, created_at
FROM node_test_data
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND node_id = sqlc.arg(node_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- One pin per node: a second pin of the same node replaces the first and counts one more revision.
-- name: UpsertFlowNodePin :exec
-- orcon:scope org
INSERT INTO flow_node_pin (id, org_id, flow_id, node_id, node_test_data_id, revision, pinned_by_principal_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_id), sqlc.arg(node_id), sqlc.arg(node_test_data_id), 1,
        sqlc.arg(pinned_by_principal_id), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (org_id, flow_id, node_id) DO UPDATE SET
  node_test_data_id = excluded.node_test_data_id,
  revision = flow_node_pin.revision + 1,
  pinned_by_principal_id = excluded.pinned_by_principal_id,
  updated_at = excluded.updated_at;

-- name: GetFlowNodePin :one
-- orcon:scope org
SELECT id, org_id, flow_id, node_id, node_test_data_id, revision, pinned_by_principal_id, created_at, updated_at
FROM flow_node_pin
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND node_id = sqlc.arg(node_id);

-- Keyset pagination by node identifier; the last sort key is unique (id).
-- name: ListFlowNodePins :many
-- orcon:scope org
SELECT id, org_id, flow_id, node_id, node_test_data_id, revision, pinned_by_principal_id, created_at, updated_at
FROM flow_node_pin
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id)
  AND (node_id > sqlc.arg(after_node_id) OR (node_id = sqlc.arg(after_node_id) AND id > sqlc.arg(after_id)))
ORDER BY node_id, id
LIMIT sqlc.arg(page_size);

-- name: DeleteFlowNodePin :execrows
-- orcon:scope org
-- orcon:delete audit:flow.pin_changed
DELETE FROM flow_node_pin
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND node_id = sqlc.arg(node_id);

-- Test runs are append-only (one per TEST execution).
-- name: InsertTestRun :exec
-- orcon:scope org
INSERT INTO test_run (id, org_id, flow_id, execution_id, flow_snapshot_id, target_kind, target_node_id, effect_mode,
                      plan_document, plan_document_format, started_by_principal_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_id), sqlc.arg(execution_id), sqlc.arg(flow_snapshot_id), sqlc.arg(target_kind),
        sqlc.arg(target_node_id), sqlc.arg(effect_mode), sqlc.arg(plan_document), sqlc.arg(plan_document_format),
        sqlc.arg(started_by_principal_id), sqlc.arg(created_at));

-- name: GetTestRun :one
-- orcon:scope org
SELECT id, org_id, flow_id, execution_id, flow_snapshot_id, target_kind, target_node_id, effect_mode, plan_document,
       plan_document_format, started_by_principal_id, created_at
FROM test_run
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- Test history of a flow, newest first; bounded, so a lookup reads at most page_size runs (QY-6).
-- name: ListTestRuns :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, flow_id, execution_id, flow_snapshot_id, target_kind, target_node_id, effect_mode, plan_document,
       plan_document_format, started_by_principal_id, created_at
FROM test_run
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: InsertTestRunSeed :exec
-- orcon:scope org
INSERT INTO test_run_seed (id, org_id, test_run_id, node_id, seed_kind, node_test_data_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(test_run_id), sqlc.arg(node_id), sqlc.arg(seed_kind),
        sqlc.arg(node_test_data_id), sqlc.arg(created_at));

-- name: ListTestRunSeeds :many
-- orcon:scope org
SELECT id, org_id, test_run_id, node_id, seed_kind, node_test_data_id, created_at
FROM test_run_seed
WHERE org_id = sqlc.arg(org_id) AND test_run_id = sqlc.arg(test_run_id)
ORDER BY node_id, id
LIMIT sqlc.arg(page_size);

-- name: InsertEffectPreview :exec
-- orcon:scope org
INSERT INTO effect_preview (id, org_id, execution_id, node_execution_id, node_id, status, effect, safety_level, item_count,
                            is_preview_available, preview_document, preview_document_format, preview_sha256, decision_document,
                            decision_document_format, expires_at, decided_by_principal_id, decided_at, revision, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(execution_id), sqlc.arg(node_execution_id), sqlc.arg(node_id), sqlc.arg(status),
        sqlc.arg(effect), sqlc.arg(safety_level), sqlc.arg(item_count), sqlc.arg(is_preview_available), sqlc.arg(preview_document),
        sqlc.arg(preview_document_format), sqlc.arg(preview_sha256), sqlc.arg(decision_document), sqlc.arg(decision_document_format),
        sqlc.arg(expires_at), sqlc.arg(decided_by_principal_id), sqlc.arg(decided_at), sqlc.arg(revision), sqlc.arg(created_at),
        sqlc.arg(updated_at));

-- name: GetEffectPreview :one
-- orcon:scope org
SELECT id, org_id, execution_id, node_execution_id, node_id, status, effect, safety_level, item_count, is_preview_available,
       preview_document, preview_document_format, preview_sha256, decision_document, decision_document_format, expires_at,
       decided_by_principal_id, decided_at, revision, created_at, updated_at
FROM effect_preview
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- A decision applies only to the revision the person saw (compare and set on revision).
-- name: MarkEffectPreviewDecided :execrows
-- orcon:scope org
UPDATE effect_preview
SET status = sqlc.arg(status), decision_document = sqlc.arg(decision_document),
    decision_document_format = sqlc.arg(decision_document_format), decided_by_principal_id = sqlc.arg(decided_by_principal_id),
    decided_at = sqlc.arg(decided_at), revision = revision + 1, updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND revision = sqlc.arg(expected_revision) AND status = 'OPEN';

-- name: ListEffectPreviews :many
-- orcon:scope org
SELECT id, org_id, execution_id, node_execution_id, node_id, status, effect, safety_level, item_count, is_preview_available,
       preview_document, preview_document_format, preview_sha256, decision_document, decision_document_format, expires_at,
       decided_by_principal_id, decided_at, revision, created_at, updated_at
FROM effect_preview
WHERE org_id = sqlc.arg(org_id) AND execution_id = sqlc.arg(execution_id)
ORDER BY created_at, id
LIMIT sqlc.arg(page_size);

-- Presence is a heartbeat: the first call opens the session, later calls renew it (same id).
-- name: UpsertFlowEditingSession :exec
-- orcon:scope org
INSERT INTO flow_editing_session (id, org_id, flow_id, opened_by_principal_id, client_label, last_seen_draft_revision, opened_at,
                                  last_seen_at, expires_at, revision, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_id), sqlc.arg(opened_by_principal_id), sqlc.arg(client_label),
        sqlc.arg(last_seen_draft_revision), sqlc.arg(opened_at), sqlc.arg(last_seen_at), sqlc.arg(expires_at), 1,
        sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (org_id, id) DO UPDATE SET
  last_seen_draft_revision = excluded.last_seen_draft_revision,
  last_seen_at = excluded.last_seen_at,
  expires_at = excluded.expires_at,
  revision = flow_editing_session.revision + 1,
  updated_at = excluded.updated_at;

-- name: ListFlowEditingSessions :many
-- orcon:scope org
SELECT id, org_id, flow_id, opened_by_principal_id, client_label, last_seen_draft_revision, opened_at, last_seen_at,
       expires_at, revision, created_at, updated_at
FROM flow_editing_session
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND expires_at > sqlc.arg(now)
ORDER BY opened_at, id
LIMIT sqlc.arg(page_size);

-- Retention of editor test data (ADR-0017 amendment): rows that no pin and no test run seed references,
-- created before created_before (24 hours before the run). Crosses organizations (engine plane).
-- name: PurgeEditorTestData :execrows
-- orcon:scope engine
-- orcon:delete retention:editor_test_data
DELETE FROM node_test_data
WHERE id IN (
  SELECT n.id
  FROM node_test_data n
  WHERE n.created_at < sqlc.arg(created_before)
    AND NOT EXISTS (SELECT 1 FROM flow_node_pin p WHERE p.org_id = n.org_id AND p.node_test_data_id = n.id)
    AND NOT EXISTS (SELECT 1 FROM test_run_seed s WHERE s.org_id = n.org_id AND s.node_test_data_id = n.id)
  ORDER BY n.created_at, n.id
  LIMIT sqlc.arg(page_size)
);

-- Presence expiry: sessions whose expires_at is before expires_before are removed (bookkeeping).
-- name: PurgeEditorPresence :execrows
-- orcon:scope engine
-- orcon:delete bookkeeping:presence-expired
DELETE FROM flow_editing_session
WHERE id IN (
  SELECT f.id
  FROM flow_editing_session f
  WHERE f.expires_at < sqlc.arg(expires_before)
  ORDER BY f.expires_at, f.id
  LIMIT sqlc.arg(page_size)
);
