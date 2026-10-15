-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the worker registry. The organization and principal queries are in identity.sql.
-- The PostgreSQL file of the same name holds the same query names.

-- name: UpsertWorker :exec
-- orcon:scope instance
INSERT INTO worker (id, role, host, version, started_at, heartbeat_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(role), sqlc.arg(host), sqlc.arg(version), sqlc.arg(started_at), sqlc.arg(heartbeat_at), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (id) DO UPDATE
SET version = excluded.version,
    heartbeat_at = excluded.heartbeat_at,
    updated_at = excluded.updated_at;
