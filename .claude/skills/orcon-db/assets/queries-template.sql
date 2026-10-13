-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
--
-- Template for sqlc query files (references/queries-and-transactions.md). Write
-- each query in both dialect query files with the same name (QY-1); PostgreSQL
-- uses $n parameters as below, SQLite uses ?. Names start with a verb from
-- policy/query_verbs.txt and spell every word out (NM-14). Annotations go on
-- their own lines right after "-- name:" (QY-9).

-- name: GetExample :one
-- orcon:scope org
SELECT id, org_id, created_at, updated_at, status, created_by_principal_id
FROM example
WHERE org_id = $1 AND id = $2;

-- Keyset pagination: the last ORDER BY key is unique (id), no OFFSET, and a
-- top-level LIMIT; the store clamps the page size (QY-5, QY-6). Add
-- "-- orcon:hot" after the scope line when the query is on a hot path (QY-4);
-- its index must then serve both the filter and the order.
-- name: ListExamples :many
-- orcon:scope org
SELECT id, created_at, status
FROM example
WHERE org_id = $1 AND (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $4;

-- A user-initiated delete is hard and writes its audit event in the same
-- transaction; the annotation names that event (DL, QY-9).
-- name: DeleteExample :exec
-- orcon:scope org
-- orcon:delete audit:example.deleted
DELETE FROM example
WHERE org_id = $1 AND id = $2;
