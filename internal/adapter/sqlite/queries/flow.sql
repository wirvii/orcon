-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of project, flow, flow_version and flow_snapshot. The PostgreSQL file
-- of the same name holds the same query names.

-- name: InsertProject :exec
-- orcon:scope org
INSERT INTO project (id, org_id, name, slug, environment, is_protected, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(name), sqlc.arg(slug), sqlc.arg(environment), sqlc.arg(is_protected), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetProject :one
-- orcon:scope org
SELECT id, org_id, name, slug, environment, is_protected, created_at, updated_at
FROM project
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: ListProjects :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, name, slug, environment, is_protected, created_at, updated_at
FROM project
WHERE org_id = sqlc.arg(org_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(page_size);

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
