-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the identity area: organizations, principals, users, memberships and projects.
-- The SQLite file of the same name holds the same query names.

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

-- name: GetApplicationUserCredentialsByEmail :one
-- orcon:scope instance
SELECT id, principal_id, email_normalized, display_name, status, password_hash, created_at, updated_at
FROM application_user
WHERE email_normalized = sqlc.arg(email_normalized);

-- name: UpdateApplicationUserPasswordHash :execrows
-- orcon:scope instance
UPDATE application_user
SET password_hash = sqlc.arg(password_hash), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: InsertSession :exec
-- orcon:scope instance
INSERT INTO session (id, application_user_id, token_hash, assurance_level, created_at, expires_at, idle_expires_at, last_seen_at, revoked_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(application_user_id), sqlc.arg(token_hash), sqlc.arg(assurance_level), sqlc.arg(created_at), sqlc.arg(expires_at), sqlc.arg(idle_expires_at), sqlc.arg(last_seen_at), NULL, sqlc.arg(created_at));

-- name: GetSessionByTokenHash :one
-- orcon:scope instance
SELECT s.id, s.application_user_id, s.assurance_level, s.created_at, s.expires_at, s.idle_expires_at, s.last_seen_at, s.revoked_at,
       u.principal_id, u.status AS user_status, u.email_normalized, u.display_name
FROM session AS s
JOIN application_user AS u ON u.id = s.application_user_id
WHERE s.token_hash = sqlc.arg(token_hash);

-- name: GetSession :one
-- orcon:scope instance
SELECT id, application_user_id, assurance_level, created_at, expires_at, idle_expires_at, last_seen_at, revoked_at
FROM session
WHERE application_user_id = sqlc.arg(application_user_id) AND id = sqlc.arg(id);

-- name: UpdateSessionActivity :execrows
-- orcon:scope instance
UPDATE session
SET last_seen_at = sqlc.arg(last_seen_at), idle_expires_at = sqlc.arg(idle_expires_at), updated_at = sqlc.arg(last_seen_at)
WHERE id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: MarkSessionRevoked :execrows
-- orcon:scope instance
UPDATE session
SET revoked_at = sqlc.arg(revoked_at), updated_at = sqlc.arg(revoked_at)
WHERE id = sqlc.arg(id) AND application_user_id = sqlc.arg(application_user_id) AND revoked_at IS NULL;

-- name: GetLoginThrottle :one
-- orcon:scope instance
SELECT id, failures, locked_until_at, created_at, updated_at
FROM login_throttle
WHERE id = sqlc.arg(id);

-- name: UpsertLoginThrottle :exec
-- orcon:scope instance
INSERT INTO login_throttle (id, failures, locked_until_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(failures), sqlc.arg(locked_until_at), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (id) DO UPDATE SET failures = excluded.failures, locked_until_at = excluded.locked_until_at, updated_at = excluded.updated_at;

-- name: DeleteLoginThrottle :execrows
-- orcon:scope instance
-- orcon:delete bookkeeping:the throttle record of an identifier ends when that identifier signs in
DELETE FROM login_throttle
WHERE id = sqlc.arg(id);

-- The failure count moves in one statement: concurrent failed attempts never lose a count (SPEC-004 R-AUTH-02).
-- name: UpsertLoginThrottleFailure :one
-- orcon:scope instance
INSERT INTO login_throttle (id, failures, locked_until_at, created_at, updated_at)
VALUES (sqlc.arg(id), 1, sqlc.arg(created_at), sqlc.arg(created_at), sqlc.arg(created_at))
ON CONFLICT (id) DO UPDATE SET failures = login_throttle.failures + 1, updated_at = excluded.updated_at
RETURNING failures;

-- name: MarkLoginThrottleLocked :execrows
-- orcon:scope instance
UPDATE login_throttle
SET locked_until_at = sqlc.arg(locked_until_at), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);
