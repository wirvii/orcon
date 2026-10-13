-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the instance tables (org, principal, worker, schema_migration).
-- The PostgreSQL file of the same name holds the same query names.

-- name: InsertOrganization :exec
-- orcon:scope instance
INSERT INTO org (id, name, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(status), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetOrganization :one
-- orcon:scope instance
SELECT id, name, status, created_at, updated_at
FROM org
WHERE id = sqlc.arg(id);

-- name: InsertPrincipal :exec
-- orcon:scope instance
INSERT INTO principal (id, kind, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(kind), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetSystemPrincipal :one
-- orcon:scope instance
SELECT id, kind, created_at, updated_at
FROM principal
WHERE kind = 'SYSTEM';

-- name: UpsertWorker :exec
-- orcon:scope instance
INSERT INTO worker (id, role, host, version, started_at, heartbeat_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(role), sqlc.arg(host), sqlc.arg(version), sqlc.arg(started_at), sqlc.arg(heartbeat_at), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (id) DO UPDATE
SET version = excluded.version,
    heartbeat_at = excluded.heartbeat_at,
    updated_at = excluded.updated_at;
