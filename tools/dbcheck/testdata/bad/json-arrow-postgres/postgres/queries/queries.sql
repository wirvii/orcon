-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- name: GetOrg :one
-- orcon:scope instance
SELECT id, created_at, updated_at, name FROM org WHERE name ->> 'a' = $1;

-- name: ListApiKeys :many
-- orcon:scope org
SELECT id, created_at, token_hash
FROM api_key
WHERE org_id = $1 AND (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $4;

-- name: CountApiKeys :one
-- orcon:scope org
SELECT count(*) FROM api_key WHERE org_id = $1;

-- name: CreateApiKey :one
-- orcon:scope org
INSERT INTO api_key (id, org_id, created_at, updated_at, token_hash, is_active, http_status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at;

-- name: DeleteApiKey :exec
-- orcon:scope org
-- orcon:delete audit:api_key.deleted
DELETE FROM api_key WHERE org_id = $1 AND id = $2;

-- name: ListFlows :many
-- orcon:scope org
SELECT id, name FROM flow WHERE org_id = $1 AND deleted_at IS NULL ORDER BY name, id LIMIT $2;

-- name: ClaimTokens :many
-- orcon:scope engine
-- orcon:hot
SELECT id FROM token WHERE state = 'READY' ORDER BY id LIMIT $1;
