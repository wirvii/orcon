-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- name: ListThings :many
-- orcon:scope org
-- orcon:hot
SELECT id, name FROM thing WHERE org_id = $1 AND (created_at, id) > ($2, $3) ORDER BY created_at, id LIMIT $4;

-- name: GetThing :one
-- orcon:scope org
-- orcon:hot
SELECT id, name FROM thing WHERE id = @thing_id;

-- name: CountThingsByName :one
-- orcon:scope org
SELECT count(*) FROM thing WHERE name = $1;
