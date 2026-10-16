-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the key hierarchy (ADR-0042, SPEC-004 R-KEY-*): instance and organization keys, wrapped by the
-- master key, with the probe that proves each key. The PostgreSQL file of the same name holds the same query names,
-- except ListActiveOrgKeys, which PostgreSQL serves through the engine function orcon_engine.list_active_org_keys.

-- name: InsertInstanceKey :exec
-- orcon:scope instance
INSERT INTO instance_key (id, key_level, key_version, wrapped, master_key_reference, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(key_level), sqlc.arg(key_version), sqlc.arg(wrapped), sqlc.arg(master_key_reference), sqlc.arg(status), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: InsertInstanceKeyCanary :exec
-- orcon:scope instance
INSERT INTO instance_key_canary (id, key_level, key_version, probe, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(key_level), sqlc.arg(key_version), sqlc.arg(probe), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: GetInstanceKeyByVersion :one
-- orcon:scope instance
SELECT k.key_version, k.wrapped, k.master_key_reference, k.status, k.created_at, c.probe
FROM instance_key AS k
LEFT JOIN instance_key_canary AS c ON c.key_level = k.key_level AND c.key_version = k.key_version
WHERE k.key_level = 'INSTANCE' AND k.key_version = sqlc.arg(key_version);

-- name: GetActiveInstanceKey :one
-- orcon:scope instance
SELECT k.key_version, k.wrapped, k.master_key_reference, k.status, k.created_at, c.probe
FROM instance_key AS k
LEFT JOIN instance_key_canary AS c ON c.key_level = k.key_level AND c.key_version = k.key_version
WHERE k.key_level = 'INSTANCE' AND k.status = 'ACTIVE';

-- name: GetMasterCanary :one
-- orcon:scope instance
SELECT key_version, probe
FROM instance_key_canary
WHERE key_level = 'MASTER'
ORDER BY key_version DESC
LIMIT 1;

-- name: InsertMasterCanary :exec
-- orcon:scope instance
INSERT INTO instance_key_canary (id, key_level, key_version, probe, created_at, updated_at)
VALUES (sqlc.arg(id), 'MASTER', sqlc.arg(key_version), sqlc.arg(probe), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: InsertOrgKey :exec
-- orcon:scope org
INSERT INTO org_key (id, org_id, key_version, wrapped, master_key_reference, status, probe, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(key_version), sqlc.arg(wrapped), sqlc.arg(master_key_reference), sqlc.arg(status), sqlc.arg(probe), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: GetOrgKeyByVersion :one
-- orcon:scope org
SELECT key_version, wrapped, master_key_reference, status, created_at, probe
FROM org_key
WHERE org_id = sqlc.arg(org_id) AND key_version = sqlc.arg(key_version);

-- name: GetActiveOrgKey :one
-- orcon:scope org
SELECT key_version, wrapped, master_key_reference, status, created_at, probe
FROM org_key
WHERE org_id = sqlc.arg(org_id) AND status = 'ACTIVE';

-- name: ListActiveOrgKeys :many
-- orcon:scope engine
-- the active keys of every organization are read in batches for startup verification and the scope lookup (SPEC-004 R-KEY-05)
SELECT k.org_id, k.key_version, k.wrapped, k.master_key_reference, k.status, k.created_at, k.probe
FROM org_key AS k
JOIN org AS o ON o.id = k.org_id
WHERE k.status = 'ACTIVE' AND o.id > sqlc.arg(after_org_id)
ORDER BY o.id
LIMIT sqlc.arg(page_size);

-- name: ListOrgsWithoutActiveKey :many
-- orcon:scope engine
-- the organizations that have no active key, for the startup check that gives each one a key (SPEC-004 R-KEY-05, SE3b)
SELECT o.id
FROM org AS o
WHERE o.status <> 'DELETION_SCHEDULED'
  AND NOT EXISTS (SELECT 1 FROM org_key AS k WHERE k.org_id = o.id AND k.status = 'ACTIVE')
ORDER BY o.id
LIMIT sqlc.arg(row_limit);
