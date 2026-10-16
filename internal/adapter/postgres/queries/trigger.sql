-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the trigger registry and the flow lifecycle (SPEC-002 cut T1). The SQLite file of the
-- same name holds the same query names.

-- name: GetFlowState :one
-- orcon:scope org
-- orcon:include-deleted the lifecycle reads a deleted flow to restore or purge it
SELECT id, project_id, name, activation_state, active_flow_version_id, draft_revision, revision, deleted_at
FROM flow
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: LockFlowState :one
-- orcon:scope org
-- orcon:include-deleted the lifecycle reads a deleted flow to restore or purge it
SELECT id, project_id, name, activation_state, active_flow_version_id, draft_revision, revision, deleted_at
FROM flow
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id)
FOR NO KEY UPDATE;

-- name: GetNextFlowVersionNumber :one
-- orcon:scope org
SELECT (COALESCE(MAX(version_number), 0) + 1)::bigint AS next_number
FROM flow_version
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id);

-- name: UpdateFlowActivationState :execrows
-- orcon:scope org
-- orcon:include-deleted deleting a flow also switches its state to INACTIVE
UPDATE flow
SET activation_state = sqlc.arg(activation_state),
    revision = revision + 1,
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: MarkFlowDeleted :execrows
-- orcon:scope org
UPDATE flow
SET deleted_at = sqlc.arg(deleted_at),
    deleted_by_principal_id = sqlc.arg(deleted_by_principal_id),
    revision = revision + 1,
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: RestoreFlow :execrows
-- orcon:scope org
-- orcon:include-deleted restoring clears the deletion marks of a deleted flow
UPDATE flow
SET deleted_at = NULL,
    deleted_by_principal_id = NULL,
    revision = revision + 1,
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND deleted_at IS NOT NULL;

-- name: PurgeFlow :execrows
-- orcon:scope org
-- orcon:delete retention:flow_purge
-- orcon:include-deleted retention deletes a flow past its recovery window
DELETE FROM flow
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND deleted_at IS NOT NULL;

-- name: ListFlowTriggers :many
-- orcon:scope org
SELECT id, org_id, flow_id, node_id, family, type, configuration_hash, flow_version_id, status, health, last_error, created_at, updated_at
FROM flow_trigger
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: InsertFlowTrigger :exec
-- orcon:scope org
INSERT INTO flow_trigger (id, org_id, flow_id, node_id, family, type, configuration_hash, flow_version_id, status, health, last_error, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_id), sqlc.arg(node_id), sqlc.arg(family), sqlc.arg(type), sqlc.arg(configuration_hash), sqlc.arg(flow_version_id), sqlc.arg(status), sqlc.arg(health), sqlc.arg(last_error), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: UpdateFlowTrigger :execrows
-- orcon:scope org
UPDATE flow_trigger
SET family = sqlc.arg(family),
    type = sqlc.arg(type),
    configuration_hash = sqlc.arg(configuration_hash),
    flow_version_id = sqlc.arg(flow_version_id),
    status = sqlc.arg(status),
    health = sqlc.arg(health),
    last_error = sqlc.arg(last_error),
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: UpdateFlowTriggerStatuses :exec
-- orcon:scope org
UPDATE flow_trigger
SET status = sqlc.arg(status),
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND status <> 'REMOVED';

-- name: ListTriggerAddresses :many
-- orcon:scope org
-- A trigger has a handful of addresses with a handful of methods each; one page holds them all.
SELECT a.id, a.org_id, a.flow_trigger_id, a.family, a.scope, a.pattern, a.static_prefix, a.host, a.random_suffix,
       a.response_profile, a.status, a.retired_at, a.created_at, a.updated_at,
       m.id AS method_row_id, m.method
FROM trigger_address a
LEFT JOIN trigger_address_method m ON m.org_id = a.org_id AND m.trigger_address_id = a.id
WHERE a.org_id = sqlc.arg(org_id) AND a.flow_trigger_id = sqlc.arg(flow_trigger_id)
ORDER BY a.id, m.id
LIMIT sqlc.arg(page_size);

-- name: InsertTriggerAddress :exec
-- orcon:scope org
INSERT INTO trigger_address (id, org_id, flow_trigger_id, family, scope, pattern, static_prefix, host, random_suffix, response_profile, status, retired_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(flow_trigger_id), sqlc.arg(family), sqlc.arg(scope), sqlc.arg(pattern), sqlc.arg(static_prefix), sqlc.arg(host), sqlc.arg(random_suffix), sqlc.arg(response_profile), sqlc.arg(status), sqlc.arg(retired_at), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: UpdateTriggerAddress :execrows
-- orcon:scope org
UPDATE trigger_address
SET family = sqlc.arg(family),
    host = sqlc.arg(host),
    random_suffix = sqlc.arg(random_suffix),
    response_profile = sqlc.arg(response_profile),
    status = sqlc.arg(status),
    retired_at = sqlc.arg(retired_at),
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: InsertTriggerAddressMethod :exec
-- orcon:scope org
INSERT INTO trigger_address_method (id, org_id, trigger_address_id, method, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(trigger_address_id), sqlc.arg(method), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: DeleteTriggerAddressMethods :exec
-- orcon:scope org
-- orcon:delete bookkeeping:the methods of an address are replaced as a whole when a publication changes them
DELETE FROM trigger_address_method
WHERE org_id = sqlc.arg(org_id) AND trigger_address_id = sqlc.arg(trigger_address_id);

-- name: ListTriggerRoutes :many
-- orcon:scope org
-- orcon:include-deleted a deleted flow keeps its paths reserved until it is purged
SELECT m.id AS method_row_id, a.id AS address_id, a.flow_trigger_id, t.flow_id, f.name AS flow_name, t.node_id,
       a.pattern, m.method, a.status, a.retired_at
FROM trigger_address_method m
JOIN trigger_address a ON a.org_id = m.org_id AND a.id = m.trigger_address_id
JOIN flow_trigger t ON t.org_id = a.org_id AND t.id = a.flow_trigger_id
JOIN flow f ON f.org_id = t.org_id AND f.id = t.flow_id
WHERE m.org_id = sqlc.arg(org_id) AND a.scope = sqlc.arg(scope) AND m.id > sqlc.arg(after_id)
ORDER BY m.id
LIMIT sqlc.arg(page_size);

-- name: ExistsRouteTakenByOtherOrganization :one
-- orcon:scope engine
-- Answers yes or no through the engine function; no row of another organization is read in this
-- context (ADR-0022 section 3, ADR-0077 section 6).
SELECT orcon_engine.is_route_taken(sqlc.arg(scope)::text, sqlc.arg(pattern)::text, sqlc.arg(method)::text, sqlc.arg(retired_after)::timestamptz) AS is_taken;

-- name: LockTriggerRoute :exec
-- orcon:scope engine
-- Serializes the claims of one path across organizations until the transaction ends. The key is the
-- scope and the pattern; nothing about an organization leaves it.
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(route_key)::text, 0));

-- name: MarkTriggerRoutesChanged :exec
-- orcon:scope engine
-- Tells the api processes that the route index must be rebuilt (ADR-0022 section 3).
SELECT pg_notify('orcon_routes', sqlc.arg(payload)::text);

-- name: AppendTriggerEvent :exec
-- orcon:scope org
INSERT INTO trigger_event (id, org_id, occurred_at, flow_id, node_id, type, schema_version, data_document, data_document_format, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(occurred_at), sqlc.arg(flow_id), sqlc.arg(node_id), sqlc.arg(type), sqlc.arg(schema_version), sqlc.arg(data_document), 'orcon/trigger-event@1', sqlc.arg(created_at));

-- name: ListTriggerEvents :many
-- orcon:scope org
SELECT id, org_id, occurred_at, flow_id, node_id, type, schema_version, data_document, created_at
FROM trigger_event
WHERE org_id = sqlc.arg(org_id) AND flow_id = sqlc.arg(flow_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: GetTriggerAcceptance :one
-- orcon:scope org
-- orcon:hot
-- orcon:include-deleted a deleted flow answers that its triggers are inactive
SELECT t.id, t.family, t.type, t.node_id, t.flow_version_id, v.version_number, t.configuration_hash, t.status,
       f.activation_state, f.deleted_at
FROM flow_trigger t
JOIN flow f ON f.org_id = t.org_id AND f.id = t.flow_id
JOIN flow_version v ON v.org_id = t.org_id AND v.id = t.flow_version_id
WHERE t.org_id = sqlc.arg(org_id) AND t.flow_id = sqlc.arg(flow_id) AND t.node_id = sqlc.arg(node_id);
