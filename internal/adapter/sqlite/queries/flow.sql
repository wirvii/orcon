-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of flow, flow_version and flow_snapshot. The PostgreSQL file
-- of the same name holds the same query names.

-- name: InsertFlow :exec
-- orcon:scope org
-- orcon:include-deleted a new flow row is inserted active; nothing is read
INSERT INTO flow (id, org_id, project_id, name, draft_document, draft_document_format, draft_revision, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(project_id), sqlc.arg(name), sqlc.arg(draft_document), 'orcon/flow@1', 0, sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetFlow :one
-- orcon:scope org
SELECT id, org_id, project_id, name, draft_document, draft_revision, active_flow_version_id, activation_state, deleted_at, deleted_by_principal_id, created_at, updated_at
FROM flow
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetFlowByName :one
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, project_id, name, draft_document, draft_revision, active_flow_version_id, activation_state, deleted_at, deleted_by_principal_id, created_at, updated_at
FROM flow
WHERE org_id = sqlc.arg(org_id) AND project_id = sqlc.arg(project_id) AND name = sqlc.arg(name) AND deleted_at IS NULL;

-- name: ListFlows :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, project_id, name, draft_document, draft_revision, active_flow_version_id, activation_state, deleted_at, deleted_by_principal_id, created_at, updated_at
FROM flow
WHERE org_id = sqlc.arg(org_id) AND project_id = sqlc.arg(project_id) AND id > sqlc.arg(after_id) AND deleted_at IS NULL
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListFlowActivity :many
-- orcon:scope org
-- orcon:hot
SELECT f.id, f.org_id, f.project_id, f.name, f.draft_revision, f.active_flow_version_id, f.activation_state, f.created_at, f.updated_at,
       le.id AS last_execution_id, le.status AS last_execution_status, le.created_at AS last_execution_created_at, le.finished_at AS last_execution_finished_at,
       CAST(EXISTS (SELECT 1 FROM external_operation o WHERE o.org_id = le.org_id AND o.execution_id = le.id AND o.state = 'UNCERTAIN') AS INTEGER) AS last_execution_has_uncertain_operation,
       (SELECT COUNT(*) FROM execution c WHERE c.org_id = f.org_id AND c.flow_id = f.id AND c.created_at >= sqlc.arg(since_at)) AS execution_count_last_24h
FROM flow f
LEFT JOIN execution le ON le.org_id = f.org_id AND le.id = (SELECT l.id FROM execution l WHERE l.org_id = f.org_id AND l.flow_id = f.id ORDER BY l.created_at DESC, l.id DESC LIMIT 1)
WHERE f.org_id = sqlc.arg(org_id) AND f.project_id = sqlc.arg(project_id) AND f.id > sqlc.arg(after_id) AND f.deleted_at IS NULL
ORDER BY f.id
LIMIT sqlc.arg(page_size);

-- name: CountFlows :one
-- orcon:scope org
SELECT COUNT(*)
FROM flow
WHERE org_id = sqlc.arg(org_id) AND project_id = sqlc.arg(project_id) AND deleted_at IS NULL;

-- name: CountActiveFlows :one
-- orcon:scope org
SELECT COUNT(*)
FROM flow
WHERE org_id = sqlc.arg(org_id) AND project_id = sqlc.arg(project_id) AND activation_state = 'ACTIVE' AND deleted_at IS NULL;

-- name: CountUncertainExecutions :one
-- orcon:scope org
-- orcon:hot
SELECT COUNT(DISTINCT o.execution_id)
FROM external_operation AS o
WHERE o.org_id = sqlc.arg(org_id) AND o.state = 'UNCERTAIN'
  AND EXISTS (SELECT 1 FROM execution AS e WHERE e.org_id = o.org_id AND e.id = o.execution_id AND e.project_id = sqlc.arg(project_id) AND e.created_at >= sqlc.arg(since_at));

-- name: ListExecutionHourCounts :many
-- orcon:scope org
-- orcon:hot
SELECT hour_started_at, mode, status, execution_count
FROM execution_hour_count
WHERE org_id = sqlc.arg(org_id) AND project_id = sqlc.arg(project_id) AND hour_started_at >= sqlc.arg(since_at)
LIMIT 1000;

-- name: CountSucceededTriggerExecutions :one
-- orcon:scope org
-- orcon:hot
SELECT CAST(COALESCE(SUM(execution_count), 0) AS INTEGER) AS succeeded
FROM execution_hour_count
WHERE org_id = sqlc.arg(org_id) AND project_id = sqlc.arg(project_id) AND status = 'SUCCEEDED' AND mode = 'TRIGGER' AND hour_started_at >= sqlc.arg(since_at);

-- name: UpdateFlowDraft :execrows
-- orcon:scope org
UPDATE flow
SET draft_document = sqlc.arg(draft_document),
    draft_revision = draft_revision + 1,
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND draft_revision = sqlc.arg(expected_revision) AND deleted_at IS NULL;

-- name: UpdateFlowActiveVersion :execrows
-- orcon:scope org
UPDATE flow
SET active_flow_version_id = sqlc.arg(flow_version_id),
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: InsertFlowVersion :exec
-- orcon:scope org
INSERT INTO flow_version (id, org_id, flow_id, version_number, document, document_format, content_hash, warning_document, warning_document_format, published_by_principal_id, published_at, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_id), sqlc.arg(version_number), sqlc.arg(document), 'orcon/flow@1', sqlc.arg(content_hash), sqlc.arg(warning_document), sqlc.arg(warning_document_format), sqlc.arg(published_by_principal_id), sqlc.arg(published_at), sqlc.arg(created_at));

-- name: GetFlowVersion :one
-- orcon:scope org
SELECT id, org_id, flow_id, version_number, document, content_hash, warning_document, published_by_principal_id, published_at, created_at
FROM flow_version
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: GetFlowVersionByNumber :one
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, flow_id, version_number, document, content_hash, warning_document, published_by_principal_id, published_at, created_at
FROM flow_version
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND version_number = sqlc.arg(version_number);

-- name: ListFlowVersions :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, flow_id, version_number, document, content_hash, warning_document, published_by_principal_id, published_at, created_at
FROM flow_version
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND version_number > sqlc.arg(after_number)
ORDER BY version_number, id
LIMIT sqlc.arg(page_size);

-- name: InsertFlowSnapshot :execrows
-- orcon:scope org
INSERT INTO flow_snapshot (id, org_id, flow_id, content_hash, document, document_format, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_id), sqlc.arg(content_hash), sqlc.arg(document), 'orcon/flow@1', sqlc.arg(created_at))
ON CONFLICT (org_id, content_hash) DO NOTHING;

-- name: GetFlowSnapshot :one
-- orcon:scope org
SELECT id, org_id, flow_id, content_hash, document, created_at
FROM flow_snapshot
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: GetFlowSnapshotByContentHash :one
-- orcon:scope org
SELECT id, org_id, flow_id, content_hash, document, created_at
FROM flow_snapshot
WHERE org_id = sqlc.arg(org_id) AND content_hash = sqlc.arg(content_hash);
