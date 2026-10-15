-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the audit area: pending entries, sealed events and blocks of the organization chains and of the
-- instance chain (SPEC-004 R-AUD-01..03). The PostgreSQL file of the same name holds the same query names, except
-- ListAuditPendingOrgs: the organizations with pending rows are read through the engine function
-- orcon_engine.list_audit_pending_orgs, because the application role sees one organization at a time.

-- name: InsertAuditPending :exec
-- orcon:scope org
INSERT INTO audit_pending (id, org_id, occurred_at, actor_principal_id, actor_kind, action_name, outcome, body_document, body_document_format, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(occurred_at), sqlc.arg(actor_principal_id), sqlc.arg(actor_kind), sqlc.arg(action_name), sqlc.arg(outcome), sqlc.arg(body_document), sqlc.arg(body_document_format), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: ListAuditPending :many
-- orcon:scope org
SELECT id, occurred_at, actor_principal_id, actor_kind, action_name, outcome, body_document
FROM audit_pending
WHERE org_id = sqlc.arg(org_id)
ORDER BY occurred_at, id
LIMIT sqlc.arg(row_limit);

-- name: DeleteAuditPending :exec
-- orcon:scope org
-- orcon:delete bookkeeping:the pending entry moves into the sealed chain in the same transaction
DELETE FROM audit_pending
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: ListAuditPendingOrgs :many
-- orcon:scope engine
SELECT DISTINCT org_id
FROM audit_pending
LIMIT sqlc.arg(row_limit);

-- name: ListLastAuditEvent :many
-- orcon:scope org
SELECT sequence_number, hash
FROM audit_event
WHERE org_id = sqlc.arg(org_id)
ORDER BY sequence_number DESC
LIMIT 1;

-- name: InsertAuditEvent :exec
-- orcon:scope org
INSERT INTO audit_event (id, org_id, sequence_number, occurred_at, actor_principal_id, actor_kind, action_name, outcome, body_document, body_document_format, previous_hash, hash, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(sequence_number), sqlc.arg(occurred_at), sqlc.arg(actor_principal_id), sqlc.arg(actor_kind), sqlc.arg(action_name), sqlc.arg(outcome), sqlc.arg(body_document), sqlc.arg(body_document_format), sqlc.arg(previous_hash), sqlc.arg(hash), sqlc.arg(created_at));

-- name: InsertAuditBlock :exec
-- orcon:scope org
INSERT INTO audit_block (id, org_id, first_sequence_number, last_sequence_number, head_hash, mac, key_version, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(first_sequence_number), sqlc.arg(last_sequence_number), sqlc.arg(head_hash), sqlc.arg(mac), sqlc.arg(key_version), sqlc.arg(created_at));

-- name: ListAuditEvents :many
-- orcon:scope org
SELECT sequence_number, id, occurred_at, actor_principal_id, actor_kind, action_name, outcome, body_document, body_document_format, previous_hash, hash
FROM audit_event
WHERE org_id = sqlc.arg(org_id) AND sequence_number > sqlc.arg(after_sequence)
ORDER BY sequence_number
LIMIT sqlc.arg(row_limit);

-- name: ListAuditBlocks :many
-- orcon:scope org
SELECT id, first_sequence_number, last_sequence_number, head_hash, mac, key_version, created_at
FROM audit_block
WHERE org_id = sqlc.arg(org_id) AND last_sequence_number > sqlc.arg(after_sequence)
ORDER BY last_sequence_number, id
LIMIT sqlc.arg(row_limit);

-- name: CountInstanceAuditPending :one
-- orcon:scope instance
SELECT count(*)
FROM instance_audit_pending;

-- name: InsertInstanceAuditPending :exec
-- orcon:scope instance
INSERT INTO instance_audit_pending (id, occurred_at, actor_principal_id, actor_kind, action_name, outcome, body_document, body_document_format, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(occurred_at), sqlc.arg(actor_principal_id), sqlc.arg(actor_kind), sqlc.arg(action_name), sqlc.arg(outcome), sqlc.arg(body_document), sqlc.arg(body_document_format), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: ListInstanceAuditPending :many
-- orcon:scope instance
SELECT id, occurred_at, actor_principal_id, actor_kind, action_name, outcome, body_document
FROM instance_audit_pending
ORDER BY occurred_at, id
LIMIT sqlc.arg(row_limit);

-- name: DeleteInstanceAuditPending :exec
-- orcon:scope instance
-- orcon:delete bookkeeping:the pending entry moves into the sealed instance chain in the same transaction
DELETE FROM instance_audit_pending
WHERE id = sqlc.arg(id);

-- name: ListLastInstanceAuditEvent :many
-- orcon:scope instance
SELECT sequence_number, hash
FROM instance_audit_event
ORDER BY sequence_number DESC
LIMIT 1;

-- name: InsertInstanceAuditEvent :exec
-- orcon:scope instance
INSERT INTO instance_audit_event (id, sequence_number, occurred_at, actor_principal_id, actor_kind, action_name, outcome, body_document, body_document_format, previous_hash, hash, created_at)
VALUES (sqlc.arg(id), sqlc.arg(sequence_number), sqlc.arg(occurred_at), sqlc.arg(actor_principal_id), sqlc.arg(actor_kind), sqlc.arg(action_name), sqlc.arg(outcome), sqlc.arg(body_document), sqlc.arg(body_document_format), sqlc.arg(previous_hash), sqlc.arg(hash), sqlc.arg(created_at));

-- name: InsertInstanceAuditBlock :exec
-- orcon:scope instance
INSERT INTO instance_audit_block (id, first_sequence_number, last_sequence_number, head_hash, mac, key_version, created_at)
VALUES (sqlc.arg(id), sqlc.arg(first_sequence_number), sqlc.arg(last_sequence_number), sqlc.arg(head_hash), sqlc.arg(mac), sqlc.arg(key_version), sqlc.arg(created_at));

-- name: ListInstanceAuditEvents :many
-- orcon:scope instance
SELECT sequence_number, id, occurred_at, actor_principal_id, actor_kind, action_name, outcome, body_document, body_document_format, previous_hash, hash
FROM instance_audit_event
WHERE sequence_number > sqlc.arg(after_sequence)
ORDER BY sequence_number
LIMIT sqlc.arg(row_limit);

-- name: ListInstanceAuditBlocks :many
-- orcon:scope instance
SELECT id, first_sequence_number, last_sequence_number, head_hash, mac, key_version, created_at
FROM instance_audit_block
WHERE last_sequence_number > sqlc.arg(after_sequence)
ORDER BY last_sequence_number, id
LIMIT sqlc.arg(row_limit);
