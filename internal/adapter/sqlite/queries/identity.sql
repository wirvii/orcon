-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the identity area: organizations, principals, users, memberships and projects.
-- The PostgreSQL file of the same name holds the same query names.

-- name: InsertOrganization :exec
-- orcon:scope instance
INSERT INTO org (id, name, slug, status, deletion_scheduled_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(slug), sqlc.arg(status), sqlc.arg(deletion_scheduled_at), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetOrganization :one
-- orcon:scope instance
SELECT id, name, slug, status, deletion_scheduled_at, created_at, updated_at
FROM org
WHERE id = sqlc.arg(id);

-- name: InsertPrincipal :exec
-- orcon:scope instance
INSERT INTO principal (id, kind, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(kind), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetSystemPrincipal :one
-- orcon:scope instance
SELECT id, kind, created_at, updated_at
FROM principal
WHERE kind = 'SYSTEM';

-- name: InsertProject :exec
-- orcon:scope org
INSERT INTO project (id, org_id, name, slug, handle, environment, is_protected, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(name), sqlc.arg(slug), sqlc.arg(handle), sqlc.arg(environment), sqlc.arg(is_protected), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetProject :one
-- orcon:scope org
SELECT id, org_id, name, slug, handle, environment, is_protected, created_at, updated_at
FROM project
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: ListProjects :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, name, slug, handle, environment, is_protected, created_at, updated_at
FROM project
WHERE org_id = sqlc.arg(org_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: InsertApplicationUser :exec
-- orcon:scope instance
INSERT INTO application_user (id, principal_id, email_normalized, display_name, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(principal_id), sqlc.arg(email_normalized), sqlc.arg(display_name), sqlc.arg(status), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetApplicationUser :one
-- orcon:scope instance
SELECT id, principal_id, email_normalized, display_name, status, created_at, updated_at
FROM application_user
WHERE id = sqlc.arg(id);

-- name: GetApplicationUserByEmail :one
-- orcon:scope instance
SELECT id, principal_id, email_normalized, display_name, status, created_at, updated_at
FROM application_user
WHERE email_normalized = sqlc.arg(email_normalized);

-- name: DeletePrincipal :execrows
-- orcon:scope instance
-- orcon:delete bookkeeping:the actor columns that name the principal are cleared first and the audit log keeps the identity; its audit event is written by the audit cut (SPEC-004 SE8)
DELETE FROM principal
WHERE id = sqlc.arg(id);

-- name: InsertMembership :exec
-- orcon:scope org
INSERT INTO membership (id, org_id, application_user_id, status, invited_by_principal_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(application_user_id), sqlc.arg(status), sqlc.arg(invited_by_principal_id), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetMembership :one
-- orcon:scope org
SELECT id, org_id, application_user_id, status, invited_by_principal_id, created_at, updated_at
FROM membership
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: GetMembershipOfApplicationUser :one
-- orcon:scope org
SELECT id, org_id, application_user_id, status, invited_by_principal_id, created_at, updated_at
FROM membership
WHERE org_id = sqlc.arg(org_id) AND application_user_id = sqlc.arg(application_user_id);

-- name: ListMemberships :many
-- orcon:scope org
-- orcon:hot
SELECT id, org_id, application_user_id, status, invited_by_principal_id, created_at, updated_at
FROM membership
WHERE org_id = sqlc.arg(org_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: UpdateMembershipStatus :execrows
-- orcon:scope org
UPDATE membership
SET status = sqlc.arg(status), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- Deleting a principal clears the actor columns that name it, in bounded batches, before the principal
-- row goes (orcon-db IN-12). The columns are in every organization, so the statements are engine plane
-- (listed in engine_allowlist.yaml). PostgreSQL does the same in orcon_engine.clear_principal_references.
-- name: UpdateFlowDeletedByPrincipalToNull :execrows
-- orcon:scope engine
-- orcon:include-deleted the rows to change are exactly the soft-deleted ones that name the principal
UPDATE flow
SET deleted_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM flow AS target WHERE target.deleted_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateFlowVersionPublishedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE flow_version
SET published_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM flow_version AS target WHERE target.published_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateMembershipInvitedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE membership
SET invited_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM membership AS target WHERE target.invited_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateInstanceAlertAcknowledgedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE instance_alert
SET acknowledged_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM instance_alert AS target WHERE target.acknowledged_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateExternalOperationResolvedByPrincipalToNull :execrows
-- orcon:scope engine
-- orcon:hot
UPDATE external_operation
SET resolved_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM external_operation AS target WHERE target.resolved_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateHumanTaskResolvedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE human_task
SET resolved_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM human_task AS target WHERE target.resolved_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateAttentionRequestAcknowledgedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE attention_request
SET acknowledged_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM attention_request AS target WHERE target.acknowledged_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- A row that names the principal in both attention columns is cleared by the acknowledged statement first;
-- this one leaves it for the next batch (the same rule as the PostgreSQL function).
-- name: UpdateAttentionRequestResolvedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE attention_request
SET resolved_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM attention_request AS target WHERE target.resolved_by_principal_id = sqlc.arg(principal_id) AND target.acknowledged_by_principal_id IS NOT sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateOperationalAlertAcknowledgedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE operational_alert
SET acknowledged_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM operational_alert AS target WHERE target.acknowledged_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateConnectorPackageInstalledByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE connector_package
SET installed_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM connector_package AS target WHERE target.installed_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateConnectorGrantGrantedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE connector_grant
SET granted_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM connector_grant AS target WHERE target.granted_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- The editor tables (migrations 0015 and 0016) add six actor columns; PostgreSQL clears them in the same
-- function as the columns above (migration 0017).

-- name: UpdateFlowDraftUpdatedByPrincipalToNull :execrows
-- orcon:scope engine
-- orcon:include-deleted the draft of a deleted flow keeps the principal that last changed it
UPDATE flow
SET draft_updated_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM flow AS target WHERE target.draft_updated_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateNodeTestDataCreatedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE node_test_data
SET created_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM node_test_data AS target WHERE target.created_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateFlowNodePinPinnedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE flow_node_pin
SET pinned_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM flow_node_pin AS target WHERE target.pinned_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateTestRunStartedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE test_run
SET started_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM test_run AS target WHERE target.started_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateEffectPreviewDecidedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE effect_preview
SET decided_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM effect_preview AS target WHERE target.decided_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));

-- name: UpdateFlowEditingSessionOpenedByPrincipalToNull :execrows
-- orcon:scope engine
UPDATE flow_editing_session
SET opened_by_principal_id = NULL
WHERE id IN (SELECT target.id FROM flow_editing_session AS target WHERE target.opened_by_principal_id = sqlc.arg(principal_id) ORDER BY target.id LIMIT sqlc.arg(batch_size));
