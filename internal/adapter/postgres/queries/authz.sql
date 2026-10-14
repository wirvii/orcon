-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the authorization evaluator (ADR-0041, SPEC-004 R-AZ-*): the epoch of an organization, the role bindings
-- of a user, and the permissions of the custom roles. The SQLite file of the same name holds the same query names,
-- plus GetProjectOrganization, which PostgreSQL serves through orcon_engine.project_organization.

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
