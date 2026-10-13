-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of connector_package, connector_grant and connector_reference. The
-- PostgreSQL file of the same name holds the same query names.

-- name: InsertConnectorPackage :exec
-- orcon:scope instance
INSERT INTO connector_package (id, connector_id, version, digest, kind, manifest_document, manifest_document_format, status, installed_at, installed_by_principal_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(connector_id), sqlc.arg(version), sqlc.arg(digest), sqlc.arg(kind), sqlc.arg(manifest_document), 'orcon/connector@1', sqlc.arg(status), sqlc.arg(installed_at), sqlc.arg(installed_by_principal_id), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetConnectorPackage :one
-- orcon:scope instance
-- orcon:hot
SELECT id, connector_id, version, digest, kind, manifest_document, status, installed_at, installed_by_principal_id, created_at, updated_at
FROM connector_package
WHERE connector_id = sqlc.arg(connector_id) AND version = sqlc.arg(version);

-- name: GetConnectorPackageByID :one
-- orcon:scope instance
SELECT id, connector_id, version, digest, kind, manifest_document, status, installed_at, installed_by_principal_id, created_at, updated_at
FROM connector_package
WHERE id = sqlc.arg(id);

-- name: ListConnectorPackages :many
-- orcon:scope instance
SELECT id, connector_id, version, digest, kind, manifest_document, status, installed_at, installed_by_principal_id, created_at, updated_at
FROM connector_package
WHERE id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListConnectorPackageVersions :many
-- orcon:scope instance
SELECT id, connector_id, version, digest, kind, manifest_document, status, installed_at, installed_by_principal_id, created_at, updated_at
FROM connector_package
WHERE connector_id = sqlc.arg(connector_id) AND version > sqlc.arg(after_version)
ORDER BY version, id
LIMIT sqlc.arg(page_size);

-- name: InsertConnectorGrant :exec
-- orcon:scope org
INSERT INTO connector_grant (id, org_id, connector_package_id, project_id, permissions_digest, granted_by_principal_id, granted_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(connector_package_id), sqlc.arg(project_id), sqlc.arg(permissions_digest), sqlc.arg(granted_by_principal_id), sqlc.arg(granted_at), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetConnectorGrant :one
-- orcon:scope org
SELECT id, org_id, connector_package_id, project_id, permissions_digest, granted_by_principal_id, granted_at, created_at, updated_at
FROM connector_grant
WHERE org_id = sqlc.arg(org_id) AND connector_package_id = sqlc.arg(connector_package_id) AND (project_id IS NULL OR project_id = sqlc.arg(project_id))
LIMIT 1;

-- name: DeleteDraftConnectorReferences :exec
-- orcon:scope org
-- orcon:delete bookkeeping:the-references-of-a-draft-are-replaced-by-those-of-the-draft-being-saved
DELETE FROM connector_reference
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND flow_version_id IS NULL;

-- name: InsertConnectorReference :exec
-- orcon:scope org
INSERT INTO connector_reference (id, org_id, flow_id, flow_version_id, connector_id, connector_version, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_id), sqlc.arg(flow_version_id), sqlc.arg(connector_id), sqlc.arg(connector_version), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: ListDraftConnectorReferences :many
-- orcon:scope org
SELECT id, org_id, flow_id, flow_version_id, connector_id, connector_version, created_at, updated_at
FROM connector_reference
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND flow_version_id IS NULL AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListVersionConnectorReferences :many
-- orcon:scope org
SELECT id, org_id, flow_id, flow_version_id, connector_id, connector_version, created_at, updated_at
FROM connector_reference
WHERE org_id = sqlc.arg(org_id) AND flow_version_id = sqlc.arg(flow_version_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(page_size);
