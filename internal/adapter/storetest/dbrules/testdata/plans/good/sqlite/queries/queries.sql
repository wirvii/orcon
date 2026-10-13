-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- name: ListThings :many
-- orcon:scope org
-- orcon:hot
SELECT id, name FROM thing WHERE org_id = ? AND (created_at, id) > (?, ?) ORDER BY created_at, id LIMIT ?;

-- name: GetThing :one
-- orcon:scope org
-- orcon:hot
SELECT id, name FROM thing WHERE id = ?;

-- name: CountThingsByName :one
-- orcon:scope org
SELECT count(*) FROM thing WHERE name = ?;
