-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the key hierarchy (ADR-0042, SPEC-004 R-KEY-*): instance and organization keys, wrapped by the
-- master key, with the probe that proves each key. The SQLite file of the same name holds the same query names,
-- plus ListActiveOrgKeys, which PostgreSQL serves through the engine function orcon_engine.list_active_org_keys.

-- name: InsertInstanceKey :exec
-- orcon:scope instance
INSERT INTO instance_key (key_level, key_version, wrapped, master_key_reference, status, created_at, updated_at)
VALUES (sqlc.arg(key_level), sqlc.arg(key_version), sqlc.arg(wrapped), sqlc.arg(master_key_reference), sqlc.arg(status), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: InsertInstanceKeyCanary :exec
-- orcon:scope instance
INSERT INTO instance_key_canary (key_level, key_version, probe, created_at, updated_at)
VALUES (sqlc.arg(key_level), sqlc.arg(key_version), sqlc.arg(probe), sqlc.arg(created_at), sqlc.arg(created_at));

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
INSERT INTO instance_key_canary (key_level, key_version, probe, created_at, updated_at)
VALUES ('MASTER', sqlc.arg(key_version), sqlc.arg(probe), sqlc.arg(created_at), sqlc.arg(created_at));

-- name: InsertOrgKey :exec
-- orcon:scope org
INSERT INTO org_key (org_id, key_version, wrapped, master_key_reference, status, probe, created_at, updated_at)
VALUES (sqlc.arg(org_id), sqlc.arg(key_version), sqlc.arg(wrapped), sqlc.arg(master_key_reference), sqlc.arg(status), sqlc.arg(probe), sqlc.arg(created_at), sqlc.arg(created_at));

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
