-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the authorization evaluator (ADR-0041, SPEC-004 R-AZ-*): the epoch of an organization, the role bindings
-- of a user, and the permissions of the custom roles. The PostgreSQL file of the same name holds the same query names,
-- except GetProjectOrganization, which PostgreSQL serves through orcon_engine.project_organization.

-- name: GetAuthorizationEpoch :one
-- orcon:scope instance
SELECT authorization_epoch
FROM org
WHERE id = sqlc.arg(id);

-- name: ListRoleBindingsOfUser :many
-- orcon:scope org
SELECT scope_kind, project_id, role_name
FROM role_binding
WHERE org_id = sqlc.arg(org_id) AND application_user_id = sqlc.arg(application_user_id)
ORDER BY id
LIMIT sqlc.arg(row_limit);

-- name: ListCustomRolePermissions :many
-- orcon:scope org
SELECT r.id, p.permission
FROM custom_role AS r
JOIN custom_role_permission AS p ON p.org_id = r.org_id AND p.custom_role_id = r.id
WHERE r.org_id = sqlc.arg(org_id)
ORDER BY p.id
LIMIT sqlc.arg(row_limit);

-- name: GetProjectOrganization :one
-- orcon:scope engine
SELECT org_id
FROM project
WHERE id = sqlc.arg(id);

-- name: InsertRoleBinding :exec
-- orcon:scope org
INSERT INTO role_binding (id, org_id, scope_kind, project_id, application_user_id, role_name, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(scope_kind), sqlc.narg(project_id), sqlc.arg(application_user_id), sqlc.arg(role_name), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: UpdateAuthorizationEpoch :execrows
-- orcon:scope instance
UPDATE org
SET authorization_epoch = authorization_epoch + 1
WHERE id = sqlc.arg(id);

-- name: GetResourceProject :one
-- orcon:scope engine
-- Reads across organizations on purpose: a flow, execution, external operation or human task of another organization
-- must be hidden, so its owner is resolved before any organization scope exists (ADR-0041 section 6).
SELECT f.org_id AS org_id, f.project_id AS project_id FROM flow f
WHERE sqlc.arg(kind) = 'flow' AND f.id = sqlc.arg(id) AND f.deleted_at IS NULL
UNION ALL
SELECT e.org_id, e.project_id FROM execution e
WHERE sqlc.arg(kind) = 'execution' AND e.id = sqlc.arg(id)
UNION ALL
SELECT e.org_id, e.project_id FROM external_operation o JOIN execution e ON e.org_id = o.org_id AND e.id = o.execution_id
WHERE sqlc.arg(kind) = 'externalOperation' AND o.id = sqlc.arg(id)
UNION ALL
SELECT e.org_id, e.project_id FROM human_task h JOIN execution e ON e.org_id = h.org_id AND e.id = h.execution_id
WHERE sqlc.arg(kind) = 'humanTask' AND h.id = sqlc.arg(id)
LIMIT 1;
