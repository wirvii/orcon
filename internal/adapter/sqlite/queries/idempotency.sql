-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the idempotency records (ADR-0053 section 4, SPEC-005 Amendment 8 section 8-2). A record is claimed in the
-- transaction of the change it protects and completed once the response is built. The PostgreSQL file of the same
-- name holds the same query names.

-- name: GetIdempotencyRecord :one
-- orcon:scope org
SELECT id, fingerprint, status, resource_reference, response_status, response_content_type, response_location,
       response_etag, response_body, is_secret_issued, expires_at, created_at
FROM idempotency_record
WHERE org_id = sqlc.arg(org_id) AND requester = sqlc.arg(requester) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: InsertIdempotencyRecord :exec
-- orcon:scope org
INSERT INTO idempotency_record (id, org_id, requester, idempotency_key, fingerprint, status, resource_reference,
  response_status, response_content_type, response_location, response_etag, response_body, is_secret_issued,
  expires_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(requester), sqlc.arg(idempotency_key), sqlc.arg(fingerprint), sqlc.arg(status),
  sqlc.narg(resource_reference), sqlc.narg(response_status), sqlc.narg(response_content_type), sqlc.narg(response_location),
  sqlc.narg(response_etag), sqlc.narg(response_body), sqlc.arg(is_secret_issued),
  sqlc.arg(expires_at), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: ClaimExpiredIdempotencyRecord :execrows
-- orcon:hot
-- orcon:scope org
-- Served by uq_idempotency_record_org_id_requester_idempotency_key: every request with an Idempotency-Key reaches it.
UPDATE idempotency_record
SET id = sqlc.arg(id), fingerprint = sqlc.arg(fingerprint), status = 'IN_PROGRESS', resource_reference = sqlc.narg(resource_reference),
    response_status = NULL, response_content_type = NULL, response_location = NULL, response_etag = NULL, response_body = NULL,
    is_secret_issued = 0,
    expires_at = sqlc.arg(expires_at), created_at = sqlc.arg(created_at), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND requester = sqlc.arg(requester) AND idempotency_key = sqlc.arg(idempotency_key)
  AND expires_at <= sqlc.arg(now);

-- name: MarkIdempotencyRecordCompleted :execrows
-- orcon:scope org
UPDATE idempotency_record
SET status = 'COMPLETED', response_status = sqlc.arg(response_status), response_content_type = sqlc.narg(response_content_type),
    response_location = sqlc.narg(response_location), response_etag = sqlc.narg(response_etag), response_body = sqlc.narg(response_body),
    is_secret_issued = sqlc.arg(is_secret_issued),
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND requester = sqlc.arg(requester) AND idempotency_key = sqlc.arg(idempotency_key)
  AND status = 'IN_PROGRESS';
