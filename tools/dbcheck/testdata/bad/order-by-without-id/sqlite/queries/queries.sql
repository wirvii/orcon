-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- name: GetOrg :one
-- orcon:scope instance
SELECT id, created_at, updated_at, name FROM org WHERE id = ?;

-- name: ListApiKeys :many
-- orcon:scope org
SELECT id, created_at, token_hash
FROM api_key
WHERE org_id = ? AND (created_at, id) < (?, ?)
ORDER BY created_at DESC, id DESC
LIMIT ?;

-- name: CountApiKeys :one
-- orcon:scope org
SELECT count(*) FROM api_key WHERE org_id = ?;

-- name: CreateApiKey :one
-- orcon:scope org
INSERT INTO api_key (id, org_id, created_at, updated_at, token_hash, is_active, http_status, status, authentication_kind, body_document, body_document_format)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, created_at;

-- name: DeleteApiKey :exec
-- orcon:scope org
-- orcon:delete audit:api_key.deleted
DELETE FROM api_key WHERE org_id = ? AND id = ?;

-- name: ListFlows :many
-- orcon:scope org
SELECT id, name FROM flow WHERE org_id = ? AND deleted_at IS NULL ORDER BY name LIMIT ?;

-- name: ClaimTokens :many
-- orcon:scope engine
-- orcon:hot
SELECT id FROM token WHERE state = 'READY' ORDER BY id LIMIT ?;
