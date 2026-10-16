-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the credential store (SPEC-004 SE7, ADR-0043): credentials, their versions and their shares. Every query
-- is bound to one organization. The PostgreSQL file of the same name holds the same query names.
-- A credential is always read with its current version, so the secret state (who wrote it, when) comes with it;
-- the ciphertext is read only by GetCredentialVersion.

-- name: InsertCredential :exec
-- orcon:scope org
INSERT INTO credential (
  id, org_id, project_id, name, connector_reference, status, allowed_host_document, allowed_host_document_format,
  current_version_number, created_by_principal_id, created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(project_id), sqlc.arg(name), sqlc.arg(connector_reference),
  sqlc.arg(status), sqlc.arg(allowed_host_document), sqlc.arg(allowed_host_document_format),
  sqlc.arg(current_version_number), sqlc.narg(created_by_principal_id), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: InsertCredentialVersion :exec
-- orcon:scope org
INSERT INTO credential_version (
  id, org_id, credential_id, version_number, public_field_document, public_field_document_format,
  secret_field_ciphertext, data_key_wrapped, org_key_version_number, created_by_principal_id, created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(credential_id), sqlc.arg(version_number), sqlc.arg(public_field_document),
  sqlc.arg(public_field_document_format), sqlc.arg(secret_field_ciphertext), sqlc.arg(data_key_wrapped),
  sqlc.arg(org_key_version_number), sqlc.narg(created_by_principal_id), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetCredential :one
-- orcon:scope org
SELECT c.id, c.org_id, c.project_id, c.name, c.connector_reference, c.status, c.allowed_host_document,
  c.current_version_number, c.created_by_principal_id, c.created_at, c.updated_at,
  v.created_at AS secret_set_at, v.created_by_principal_id AS secret_set_by_principal_id
FROM credential AS c
JOIN credential_version AS v ON v.org_id = c.org_id AND v.credential_id = c.id AND v.version_number = c.current_version_number
WHERE c.org_id = sqlc.arg(org_id) AND c.id = sqlc.arg(id);

-- name: ListCredentialsOfProject :many
-- orcon:scope org
SELECT c.id, c.org_id, c.project_id, c.name, c.connector_reference, c.status, c.allowed_host_document,
  c.current_version_number, c.created_by_principal_id, c.created_at, c.updated_at,
  v.created_at AS secret_set_at, v.created_by_principal_id AS secret_set_by_principal_id
FROM credential AS c
JOIN credential_version AS v ON v.org_id = c.org_id AND v.credential_id = c.id AND v.version_number = c.current_version_number
WHERE c.org_id = sqlc.arg(org_id)
  AND (c.project_id = sqlc.arg(project_id)
    OR c.id IN (SELECT s.credential_id FROM credential_share AS s WHERE s.org_id = sqlc.arg(org_id) AND s.project_id = sqlc.arg(project_id)))
ORDER BY c.name, c.id
LIMIT sqlc.arg(row_limit);

-- name: GetCredentialVersion :one
-- orcon:scope org
SELECT id, org_id, credential_id, version_number, public_field_document, public_field_document_format,
  secret_field_ciphertext, data_key_wrapped, org_key_version_number, created_by_principal_id, created_at
FROM credential_version
WHERE org_id = sqlc.arg(org_id) AND credential_id = sqlc.arg(credential_id) AND version_number = sqlc.arg(version_number);

-- name: UpdateCredentialCurrentVersion :execrows
-- orcon:scope org
UPDATE credential
SET current_version_number = sqlc.arg(next_version_number), status = sqlc.arg(status), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND current_version_number = sqlc.arg(expected_version_number);

-- name: UpdateCredentialSettings :execrows
-- orcon:scope org
UPDATE credential
SET name = sqlc.arg(name), allowed_host_document = sqlc.arg(allowed_host_document), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND project_id = sqlc.arg(project_id) AND updated_at = sqlc.arg(expected_updated_at);

-- name: DeleteCredential :execrows
-- orcon:scope org
-- orcon:delete bookkeeping:the credential audit event (R-AUD-01) is appended in the same transaction by the adapter; the versions and shares go with the row
DELETE FROM credential
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND project_id = sqlc.arg(project_id) AND updated_at = sqlc.arg(expected_updated_at);

-- name: GetCredentialShare :one
-- orcon:scope org
SELECT id, org_id, credential_id, project_id, granted_by_principal_id, created_at
FROM credential_share
WHERE org_id = sqlc.arg(org_id) AND credential_id = sqlc.arg(credential_id) AND project_id = sqlc.arg(project_id);

-- name: InsertCredentialShare :exec
-- orcon:scope org
INSERT INTO credential_share (id, org_id, credential_id, project_id, granted_by_principal_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(credential_id), sqlc.arg(project_id), sqlc.narg(granted_by_principal_id), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: DeleteCredentialShare :execrows
-- orcon:scope org
-- orcon:delete bookkeeping:the share audit event (R-AUD-01) is appended in the same transaction by the adapter; the next dispatch sees the removal at once
DELETE FROM credential_share
WHERE org_id = sqlc.arg(org_id) AND credential_id = sqlc.arg(credential_id) AND project_id = sqlc.arg(project_id);

-- Deleting a principal clears the author columns that name it, in bounded batches, before the principal row goes
-- (orcon-db IN-12). The columns are in every organization, so the statements are engine plane (listed in
-- engine_allowlist.yaml). PostgreSQL does the same in orcon_engine.clear_principal_references (migration 0028).

-- name: UpdateCredentialCreatedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE credential
SET created_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM credential AS target WHERE target.created_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateCredentialVersionCreatedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE credential_version
SET created_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM credential_version AS target WHERE target.created_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateCredentialShareGrantedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE credential_share
SET granted_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM credential_share AS target WHERE target.granted_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));
